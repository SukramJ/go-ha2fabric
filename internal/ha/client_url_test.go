// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package ha_test

import (
	"bytes"
	"log/slog"
	"strings"
	"sync"
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
