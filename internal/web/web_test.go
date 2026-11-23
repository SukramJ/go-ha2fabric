// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package web

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	fabricui "github.com/SukramJ/go-fabric-ui/fabricui"
)

func mustNew(t *testing.T, opts Options) *Server {
	t.Helper()
	s, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// get sends a GET from remote under a loopback Host, which go-fabric-ui's
// Host check admits without a name rule (httptest's default is
// example.com).
func get(t *testing.T, h http.Handler, path, remote string) (code int, body []byte) {
	t.Helper()
	return getHost(t, h, path, remote, "127.0.0.1:8080")
}

func getHost(t *testing.T, h http.Handler, path, remote, host string) (code int, body []byte) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
	req.Host = host
	if remote != "" {
		req.RemoteAddr = remote
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	res := rec.Result()
	defer func() { _ = res.Body.Close() }()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, b
}

// TestHandler: go-fabric-ui's shell and API; the 0.1.0 routes are gone.
func TestHandler(t *testing.T) {
	s := mustNew(t, Options{UI: fabricui.Options{Admin: ReadOnly}})
	for path, want := range map[string]string{"/": "<html", "/api/v1/node/status": `"enabled"`, "/api/v1/nope": "not_found"} {
		_, b := get(t, s.Handler(), path, "")
		if !strings.Contains(string(b), want) {
			t.Errorf("%s: %.200s", path, b)
		}
	}
	for _, path := range []string{"/api/health", "/api/plan"} {
		if _, b := get(t, s.Handler(), path, ""); strings.Contains(string(b), `"status"`) || strings.Contains(string(b), "entity_id") {
			t.Errorf("%s still answers JSON: %.200s", path, b)
		}
	}
}

// TestAllow: the client filter serves the listed addresses only, and an
// entry that is not an IP address fails New instead of being dropped —
// with none left the filter would serve everyone (HR-P0-err-26).
func TestAllow(t *testing.T) {
	for _, allow := range [][]string{{"172.30.32.2", "nonsense"}, {"nonsense"}, {""}} {
		if _, err := New(Options{Allow: allow}); err == nil || !strings.Contains(err.Error(), "not an IP address") {
			t.Fatalf("Allow %q: %v", allow, err)
		}
	}
	s := mustNew(t, Options{Allow: []string{"172.30.32.2"}})
	if code, _ := get(t, s.Handler(), "/api/v1/node/status", "172.30.32.2:4321"); code != http.StatusOK {
		t.Fatalf("Ingress proxy refused: %d", code)
	}
	if code, _ := get(t, s.Handler(), "/api/v1/node/status", "192.168.1.20:4321"); code != http.StatusForbidden {
		t.Fatalf("a LAN client was served: %d", code)
	}
	if code, _ := get(t, s.Handler(), "/api/v1/node/status", "garbage"); code != http.StatusForbidden {
		t.Fatalf("an unparsable client was served: %d", code)
	}
}

// TestHost: go-fabric-ui's Host check as the daemon wires it — a DNS
// name is refused unless WEB_ALLOWED_HOSTS lists it; in the add-on the
// Ingress proxy's address admits the browser's Host (Home Assistant's
// name) and nothing else does; an entry go-fabric-ui refuses fails New.
func TestHost(t *testing.T) {
	const status = "/api/v1/node/status"
	plain := mustNew(t, Options{UI: fabricui.Options{Admin: ReadOnly}})
	if code, b := getHost(t, plain.Handler(), status, "192.168.1.20:4000", "bridge.lan:8482"); code != http.StatusForbidden || !strings.Contains(string(b), "host_not_allowed") {
		t.Fatalf("unlisted name: %d %.200s", code, b)
	}
	named := mustNew(t, Options{UI: fabricui.Options{Admin: ReadOnly, AllowedHosts: []string{"bridge.lan"}}})
	if code, _ := getHost(t, named.Handler(), status, "192.168.1.20:4000", "bridge.lan:8482"); code != http.StatusOK {
		t.Fatalf("listed name: %d", code)
	}
	_, ring := NewLogger(io.Discard, new(slog.LevelVar))
	addon := mustNew(t, Options{UI: fabricui.Options{
		Admin: From("172.30.32.2"), AllowHost: From("172.30.32.2"),
		Operations: NewOperations(ring, "1.2.3", time.Now()),
	}})
	if code, _ := getHost(t, addon.Handler(), status, "172.30.32.2:4000", "homeassistant.local:8123"); code != http.StatusOK {
		t.Fatalf("Ingress proxy with the browser's Host: %d", code)
	}
	if code, _ := getHost(t, addon.Handler(), status, "192.168.1.20:4000", "homeassistant.local:8123"); code != http.StatusForbidden {
		t.Fatalf("another client with a DNS name: %d", code)
	}
	// An administrator write through the proxy: the browser's Origin
	// equals the Host the proxy forwarded, so go-fabric-ui's Origin check
	// passes; a foreign Origin does not.
	for origin, want := range map[string]int{"https://homeassistant.local:8123": http.StatusOK, "https://evil.example": http.StatusForbidden} {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/operation/log/level", strings.NewReader(`{"level":"warn"}`))
		req.Host, req.RemoteAddr = "homeassistant.local:8123", "172.30.32.2:4000"
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(fabricui.RequestHeader, "1")
		req.Header.Set("Origin", origin)
		rec := httptest.NewRecorder()
		addon.Handler().ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("admin write with Origin %s: %d, want %d: %.200s", origin, rec.Code, want, rec.Body.String())
		}
	}
	if _, err := New(Options{UI: fabricui.Options{AllowedHosts: []string{"bridge.lan:8482"}}}); err == nil {
		t.Fatal("an AllowedHosts entry with a port was accepted")
	}
}

func TestAdmin(t *testing.T) {
	ingress := From("172.30.32.2")
	for remote, want := range map[string]bool{"172.30.32.2:4000": true, "172.30.32.3:4000": false, "garbage": false} {
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		req.RemoteAddr = remote
		if ingress(req) != want || ReadOnly(req) {
			t.Errorf("%s: From %v, want %v", remote, ingress(req), want)
		}
	}
}

func TestRun(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	ctx, cancel := context.WithCancel(context.Background())
	s := mustNew(t, Options{Listen: addr})
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		res, err := http.Get("http://" + addr + "/api/v1/node/status") //nolint:noctx // a test against its own server
		if err == nil {
			_ = res.Body.Close()
			if res.StatusCode != http.StatusOK {
				t.Fatalf("status %d", res.StatusCode)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := mustNew(t, Options{Listen: "256.0.0.1:1"}).Run(context.Background()); err == nil {
		t.Fatal("a bad address served")
	}
}

// TestIngressAdmin: the add-on's administrator is a request of the
// Ingress proxy whose user's groups hold system-admin; any other request
// is not.
func TestIngressAdmin(t *testing.T) {
	a := IngressAdmin("172.30.32.2")
	for _, c := range []struct {
		remote string
		groups []string
		want   bool
	}{
		{"172.30.32.2:4000", []string{"system-admin"}, true},
		{"172.30.32.2:4000", []string{"system-users,system-admin"}, true},
		{"172.30.32.2:4000", []string{"system-users", " system-admin "}, true},
		{"172.30.32.2:4000", nil, false},
		{"172.30.32.2:4000", []string{"system-users"}, false},
		{"172.30.32.2:4000", []string{"system-admins"}, false},
		{"172.30.32.2:4000", []string{""}, false},
		{"192.168.1.20:4000", []string{"system-admin"}, false},
		{"garbage", []string{"system-admin"}, false},
	} {
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		req.RemoteAddr = c.remote
		for _, g := range c.groups {
			req.Header.Add(UserGroupsHeader, g)
		}
		if got := a(req); got != c.want {
			t.Errorf("%s %q: %v, want %v", c.remote, c.groups, got, c.want)
		}
	}
}
