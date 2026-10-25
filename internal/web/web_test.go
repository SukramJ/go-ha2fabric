// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package web

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	fabricui "github.com/SukramJ/go-fabric-ui/fabricui"
)

func get(t *testing.T, h http.Handler, path, remote string) (code int, body []byte) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
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
	s := New(Options{UI: fabricui.Options{Admin: ReadOnly}})
	for path, want := range map[string]string{"/": "<html", "/#/node/pair": "<html", "/api/v1/node/status": `"enabled"`, "/api/v1/nope": "not_found"} {
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

func TestAllow(t *testing.T) {
	s := New(Options{Allow: []string{"172.30.32.2", "nonsense"}})
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
	s := New(Options{Listen: addr})
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
	if err := New(Options{Listen: "256.0.0.1:1"}).Run(context.Background()); err == nil {
		t.Fatal("a bad address served")
	}
}
