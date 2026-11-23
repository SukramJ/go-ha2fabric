// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package ha_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SukramJ/go-ha2fabric/internal/ha"
	"github.com/SukramJ/go-ha2fabric/internal/ha/hatest"
)

// syncBuffer is a log sink the client's goroutines write to.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// TestClientLogsURLWithoutQuery: the URL in the log and in a dial error
// carries no query, which may hold a proxy's credential (HR-P0-leak-25).
func TestClientLogsURLWithoutQuery(t *testing.T) {
	srv := hatest.New(t)
	var logs syncBuffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	_, r, _ := start(t, ha.Options{URL: srv.URL() + "?key=s3cret", Token: hatest.Token, Logger: logger})
	r.wait(t, "connected")
	if !strings.Contains(logs.String(), "ha.connected") || strings.Contains(logs.String(), "s3cret") {
		t.Fatalf("log:\n%s", logs.String())
	}

	var dl syncBuffer
	_, r2, _ := start(t, ha.Options{URL: "ws://127.0.0.1:1/api/websocket?key=s3cret", Token: hatest.Token, MinBackoff: time.Hour, Logger: slog.New(slog.NewTextHandler(&dl, nil))})
	r2.wait(t, "disconnected")
	r2.mu.Lock()
	err := r2.errs[0]
	r2.mu.Unlock()
	if err == nil || strings.Contains(err.Error(), "s3cret") {
		t.Fatalf("dial error: %v", err)
	}
	if got := ha.LogURL("wss://ha.local:8123/api/websocket?key=s3cret#f"); got != "wss://ha.local:8123/api/websocket" {
		t.Fatalf("LogURL = %q", got)
	}
}

// TestClientRefusesRedirect: the WebSocket dial follows no redirect, so
// the token is sent only to the configured endpoint (HR-P0-ha-31).
func TestClientRefusesRedirect(t *testing.T) {
	target := hatest.New(t)
	to := "http" + strings.TrimPrefix(target.URL(), "ws")
	redir := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, to, http.StatusFound)
	}))
	t.Cleanup(redir.Close)
	_, r, _ := start(t, ha.Options{URL: "ws" + strings.TrimPrefix(redir.URL, "http") + "/api/websocket", Token: hatest.Token, MinBackoff: time.Hour, Logger: slog.New(slog.DiscardHandler)})
	r.wait(t, "disconnected")
	r.mu.Lock()
	err := r.errs[0]
	r.mu.Unlock()
	if target.Authenticated() != 0 || err == nil || !strings.Contains(err.Error(), "redirect") {
		t.Fatalf("authenticated %d times through a redirect: %v", target.Authenticated(), err)
	}
}

// TestImageRefusesRedirect: the camera proxy's request with the Bearer
// token follows no redirect either (HR-P0-ha-56).
func TestImageRefusesRedirect(t *testing.T) {
	var reached atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached.Add(1)
		_, _ = w.Write([]byte("jpeg"))
	}))
	t.Cleanup(target.Close)
	redir := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+r.URL.Path, http.StatusFound)
	}))
	t.Cleanup(redir.Close)
	c := ha.New(ha.Options{URL: "ws" + strings.TrimPrefix(redir.URL, "http") + "/api/websocket", Token: "tok"})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _, err := c.Image(ctx, "camera.front", 0, 0)
	if err == nil || !strings.Contains(err.Error(), "redirect") || reached.Load() != 0 {
		t.Fatalf("Image through a redirect: %v, target reached %d times", err, reached.Load())
	}
}
