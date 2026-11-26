// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestAtLeast(t *testing.T) {
	for v, want := range map[string]bool{
		"2026.10.4": true, "2026.10.5": true, "2026.10.10": true, "2026.11.0": true, "2027.1.0": true,
		"2026.11.0.dev1201": true, "2026.10.5b1": true, "2026.10.4.post1": false,
		// A pre-release of the floor itself counts as below it (HR-P0-auth-50).
		"2026.10.4b1": false, "2026.10.4.dev1": false, "2026.10.4rc0": false,
		"2026.10.3": false, "2026.10.2": false, "2026.9.9": false, "2025.12.0": false,
		"": false, "2026.10": false, "x.y.z": false, "2026.x10.4": false, "dev": false,
	} {
		if got := atLeast(v, minSupervisor); got != want {
			t.Errorf("%q: %v, want %v", v, got, want)
		}
	}
}

// fakeSupervisor answers GET /info with version after fails failures,
// and only to the token.
func fakeSupervisor(t *testing.T, version string, fails int32) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if r.URL.Path != "/info" || r.Header.Get("Authorization") != "Bearer sup-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if n <= fails {
			http.Error(w, "starting", http.StatusBadGateway)
			return
		}
		_, _ = fmt.Fprintf(w, `{"result":"ok","data":{"supervisor":%q,"homeassistant":"2026.10.3"}}`, version)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func runGate(t *testing.T, url, token string) (*supervisorGate, *safeBuffer) {
	t.Helper()
	var logs safeBuffer
	g := newSupervisorGate(token, slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	g.url, g.retry, g.recheck = url+"/info", 10*time.Millisecond, 10*time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); g.run(ctx) }()
	t.Cleanup(func() { cancel(); <-done })
	return g, &logs
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("never: %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// TestSupervisorGate: the add-on's web UI gets an administrator only on a
// Supervisor of 2026.10.4 or later; before the version is read, and
// below it, the gate stays shut with a warning; a failed read is retried.
func TestSupervisorGate(t *testing.T) {
	srv, calls := fakeSupervisor(t, "2026.10.4", 2)
	g, logs := runGate(t, srv.URL, "sup-token")
	eventually(t, "the gate opens", g.open)
	if calls.Load() < 3 || len(g.Warnings()) != 0 || !strings.Contains(logs.String(), "ha2fabric.supervisor_unreadable") || strings.Contains(logs.String(), "sup-token") {
		t.Fatalf("calls %d, warnings %v\n%s", calls.Load(), g.Warnings(), logs.String())
	}

	old, _ := fakeSupervisor(t, "2026.10.3", 0)
	g, _ = runGate(t, old.URL, "sup-token")
	eventually(t, "the gate shuts", func() bool { return gateState(g.state.Load()) == gateClosed })
	if g.open() || len(g.Warnings()) != 1 || g.Warnings()[0].Code != warningSupervisorTooOld || !strings.Contains(g.Warnings()[0].Message, "2026.10.3") {
		t.Fatalf("an old Supervisor: open %v, %+v", g.open(), g.Warnings())
	}

	unauthorized, _ := fakeSupervisor(t, "2026.11.0", 0)
	g, _ = runGate(t, unauthorized.URL, "wrong")
	eventually(t, "a warning", func() bool { w := g.Warnings(); return len(w) == 1 && strings.Contains(w[0].Message, "401") })
	if g.open() || g.Warnings()[0].Code != warningSupervisorUnknown {
		t.Fatalf("an unread version: open %v, %+v", g.open(), g.Warnings())
	}
}

// TestSupervisorGateRechecks: a closed gate reads the version again and
// opens once the Supervisor is updated, without a restart; a version that
// does not parse is reported as such (HR-P0-auth-51).
func TestSupervisorGateRechecks(t *testing.T) {
	var version atomic.Value
	version.Store("2026.10.3")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, `{"result":"ok","data":{"supervisor":%q}}`, version.Load())
	}))
	t.Cleanup(srv.Close)
	g, _ := runGate(t, srv.URL, "sup-token")
	eventually(t, "the gate shuts", func() bool { return gateState(g.state.Load()) == gateClosed })
	version.Store("2026.10.4")
	eventually(t, "the gate opens after the update", g.open)
	if len(g.Warnings()) != 0 {
		t.Fatalf("warnings after the update: %+v", g.Warnings())
	}

	odd, _ := fakeSupervisor(t, "latest", 0)
	g, _ = runGate(t, odd.URL, "sup-token")
	eventually(t, "the gate shuts", func() bool { return gateState(g.state.Load()) == gateClosed })
	if w := g.Warnings()[0]; g.open() || !strings.Contains(w.Message, `"latest"`) || !strings.Contains(w.Message, "does not parse") {
		t.Fatalf("an unparsable version: %+v", w)
	}
}

// TestSupervisorGateRecloses (HR-P2-cfg-03): an open gate keeps reading
// the version; a downgrade below the floor closes it again, a read that
// fails meanwhile does not.
func TestSupervisorGateRecloses(t *testing.T) {
	var version atomic.Value
	version.Store("2026.10.4")
	var failing atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if failing.Load() {
			http.Error(w, "restarting", http.StatusBadGateway)
			return
		}
		_, _ = fmt.Fprintf(w, `{"result":"ok","data":{"supervisor":%q}}`, version.Load())
	}))
	t.Cleanup(srv.Close)
	g, logs := runGate(t, srv.URL, "sup-token")
	eventually(t, "the gate opens", g.open)
	failing.Store(true)
	eventually(t, "a failed read", func() bool { return strings.Count(logs.String(), "supervisor_unreadable") >= 2 })
	if !g.open() {
		t.Fatal("a failed read closed the open gate")
	}
	version.Store("2026.10.3")
	failing.Store(false)
	eventually(t, "the gate shuts after the downgrade", func() bool { return gateState(g.state.Load()) == gateClosed })
	if w := g.Warnings(); len(w) != 1 || w[0].Code != warningSupervisorTooOld {
		t.Fatalf("warnings after the downgrade: %+v", w)
	}
	version.Store("2026.11.0")
	eventually(t, "the gate opens again", g.open)
	// The state flips before the line is logged: wait for the second line.
	eventually(t, "one opening line per change", func() bool { return strings.Count(logs.String(), "admin=true") == 2 })
	if n := strings.Count(logs.String(), "admin=true"); n != 2 {
		t.Fatalf("%d opening lines, want one per change\n%s", n, logs.String())
	}
}

// TestSupervisorGateNoRedirectNoProxy (HR-P1-ha-13, -ha-14): the gate's
// request carries SUPERVISOR_TOKEN, so it follows no redirect and takes
// no proxy from the environment; the call to the Supervisor never leaves
// the host.
func TestSupervisorGateNoRedirectNoProxy(t *testing.T) {
	var elsewhere atomic.Int32
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		elsewhere.Add(1)
		_, _ = fmt.Fprint(w, `{"result":"ok","data":{"supervisor":"2026.10.4"}}`)
	}))
	t.Cleanup(other.Close)
	redirect := httptest.NewServer(http.RedirectHandler(other.URL+"/info", http.StatusFound))
	t.Cleanup(redirect.Close)
	g, logs := runGate(t, redirect.URL, "sup-token")
	eventually(t, "a failed read", func() bool { return strings.Contains(logs.String(), "ha2fabric.supervisor_unreadable") })
	if elsewhere.Load() != 0 || g.open() {
		t.Fatalf("the redirect was followed: %d requests elsewhere, open %v", elsewhere.Load(), g.open())
	}
	tr, ok := g.client.Transport.(*http.Transport)
	if !ok || tr.Proxy != nil {
		t.Fatalf("the gate's transport takes a proxy: %T", g.client.Transport)
	}
}

// TestAddonAddress reads the add-on's address from GET /addons/self/info
// (decision 54 of the 2026-11-23 review) and refuses an answer without one.
func TestAddonAddress(t *testing.T) {
	for _, c := range []struct {
		name, body string
		status     int
		want       string
	}{
		{"found", `{"result":"ok","data":{"ip_address":"172.30.32.1","ingress_port":8486}}`, 200, "172.30.32.1"},
		{"no address", `{"result":"ok","data":{"ip_address":"0.0.0.0"}}`, 200, ""},
		{"not an address", `{"result":"ok","data":{"ip_address":"hassio"}}`, 200, ""},
		{"missing", `{"result":"ok","data":{}}`, 200, ""},
		{"error result", `{"result":"error","message":"Self is not an App"}`, 200, ""},
		{"undecodable", `<html>`, 200, ""},
		{"refused", `{"result":"error"}`, 403, ""},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/addons/self/info" || r.Header.Get("Authorization") != "Bearer sup-token" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			w.WriteHeader(c.status)
			_, _ = w.Write([]byte(c.body))
		}))
		a, err := addonAddress(context.Background(), supervisorClient(), srv.URL+"/addons/self/info", "sup-token")
		srv.Close()
		switch {
		case c.want == "" && err == nil:
			t.Errorf("%s: %v, want an error", c.name, a)
		case c.want != "" && (err != nil || a.String() != c.want):
			t.Errorf("%s: %v, %v; want %s", c.name, a, err, c.want)
		}
	}
}

// TestAddonWebListen binds the add-on's web UI to the Supervisor's address
// of the add-on when it is found, and keeps WEB_LISTEN with one warning
// when it is not.
func TestAddonWebListen(t *testing.T) {
	found := func(context.Context) (netip.Addr, error) { return netip.MustParseAddr("172.30.32.1"), nil }
	var calls int
	missing := func(context.Context) (netip.Addr, error) {
		calls++
		return netip.Addr{}, errors.New("supervisor: GET /addons/self/info answered 502")
	}
	for _, c := range []struct {
		listen string
		read   func(context.Context) (netip.Addr, error)
		want   string
		warn   bool
	}{
		{"0.0.0.0:8486", found, "172.30.32.1:8486", false},
		{":8486", found, "172.30.32.1:8486", false},
		{"[::]:8486", found, "172.30.32.1:8486", false},
		{"0.0.0.0:8486", missing, "0.0.0.0:8486", true},
		// A host set explicitly stays.
		{"192.0.2.7:8486", found, "192.0.2.7:8486", false},
		{"localhost:8486", found, "localhost:8486", false},
	} {
		var logs safeBuffer
		log := slog.New(slog.NewTextHandler(&logs, nil))
		calls = 0
		if got := addonWebListen(context.Background(), c.listen, c.read, time.Millisecond, log); got != c.want {
			t.Errorf("%s: %s, want %s", c.listen, got, c.want)
		}
		if warned := strings.Contains(logs.String(), "level=WARN msg=ha2fabric.web_listen"); warned != c.warn {
			t.Errorf("%s: warned %v, want %v (%s)", c.listen, warned, c.warn, logs.String())
		}
		if c.warn && calls != addressTries {
			t.Errorf("%s: %d reads, want %d", c.listen, calls, addressTries)
		}
	}
}
