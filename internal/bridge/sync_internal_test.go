// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package bridge

import (
	"log/slog"
	"testing"
	"time"

	"github.com/SukramJ/go-ha2fabric/internal/plan"
)

// syncBridge is a running bridge on up with a short debounce.
func syncBridge(t *testing.T, up *fakeUp, edit ...func(*Options)) *Bridge {
	t.Helper()
	opts := Options{Debounce: 20 * time.Millisecond, Logger: slog.New(slog.DiscardHandler), Input: func() plan.Input { return plan.Input{} }}
	for _, fn := range edit {
		fn(&opts)
	}
	b := New(nil, up, opts)
	b.connected.Store(up.Connected())
	run(t, b)
	return b
}

// TestDebounceHasALatestTime (HR-P2-data-30): a stream of re-sync
// requests closer than the debounce cannot put the sync off without end;
// it runs at the latest maxWait debounces after the first request.
func TestDebounceHasALatestTime(t *testing.T) {
	up := &fakeUp{}
	up.connected.Store(true)
	b := syncBridge(t, up)
	end := time.Now().Add(20 * b.opts.Debounce)
	for time.Now().Before(end) {
		b.resyncSoon()
		time.Sleep(b.opts.Debounce / 10)
	}
	if n := up.reads.Load(); n == 0 {
		t.Fatalf("a stream of requests every %s put the sync off for %s (debounce %s)", b.opts.Debounce/10, 20*b.opts.Debounce, b.opts.Debounce)
	}
}
