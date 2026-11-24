// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package bridge

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SukramJ/go-ha2fabric/internal/ha"

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

// records is a slog handler that keeps every record.
type records struct {
	mu  sync.Mutex
	all []slog.Record
}

func (r *records) Enabled(context.Context, slog.Level) bool { return true }
func (r *records) Handle(_ context.Context, rec slog.Record) error {
	r.mu.Lock()
	r.all = append(r.all, rec.Clone())
	r.mu.Unlock()
	return nil
}
func (r *records) WithAttrs([]slog.Attr) slog.Handler { return r }
func (r *records) WithGroup(string) slog.Handler      { return r }

// named are the records with message msg.
func (r *records) named(msg string) []slog.Record {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []slog.Record
	for _, rec := range r.all {
		if rec.Message == msg {
			out = append(out, rec)
		}
	}
	return out
}

// ctxUp is fakeUp whose read ends with its context.
type ctxUp struct{ *fakeUp }

func (c ctxUp) GetStates(ctx context.Context) ([]ha.State, error) {
	c.reads.Add(1)
	<-ctx.Done()
	return nil, ctx.Err()
}

// TestCancelledSyncIsNoFailure (HR-P2-err-67): a sync the stop, an erase
// or a delete cancels is no failure: no bridge.sync_failed at Error, no
// SyncError.
func TestCancelledSyncIsNoFailure(t *testing.T) {
	up := &fakeUp{}
	up.connected.Store(true)
	rec := &records{}
	b := New(nil, ctxUp{up}, Options{Logger: slog.New(rec), Input: func() plan.Input { return plan.Input{} }})
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(10*time.Millisecond, cancel)
	_ = b.sync(ctx)
	if n := len(rec.named("bridge.sync_failed")); n != 0 {
		t.Fatalf("a cancelled sync logged bridge.sync_failed %d time(s)", n)
	}
	if e := b.Status().SyncError; e != "" {
		t.Fatalf("a cancelled sync set SyncError %q", e)
	}
}

// TestCancelledCommandIsNoFailure (HR-P2-err-67): a call_service the
// stop cancels is not counted as a failed command.
func TestCancelledCommandIsNoFailure(t *testing.T) {
	b, up := rangeBridge(t)
	ctx, cancel := context.WithCancel(context.Background())
	up.call = func(ha.ServiceCall) error { cancel(); return context.Canceled }
	b.command(ctx, rangeCmd("temperature_low", "19"))
	if s := b.Status(); s.CommandErrs != 0 || s.LastCommand != "" {
		t.Fatalf("a cancelled call_service counted as a failure: %+v", s)
	}
}

// TestFailedSyncIsRetried (HR-P2-err-61): a sync that fails while Home
// Assistant stays connected is tried again, with a back-off; without a
// connection it is not (the reconnection syncs).
func TestFailedSyncIsRetried(t *testing.T) {
	up := &fakeUp{}
	up.connected.Store(true)
	b := syncBridge(t, up, func(o *Options) { o.Retry = 20 * time.Millisecond })
	b.Connected("2026.10.3")
	waitFor(t, "the failed sync tried twice more", func() bool { return up.reads.Load() >= 3 })
	up.connected.Store(false)
	b.Disconnected(nil)
	time.Sleep(50 * time.Millisecond)
	n := up.reads.Load()
	time.Sleep(500 * time.Millisecond)
	if m := up.reads.Load(); m != n {
		t.Fatalf("a sync was retried while Home Assistant was away: %d read(s), then %d", n, m)
	}
}

// TestResyncSaysWhetherItPlanned (HR-P2-err-65): a Resync whose sync
// found Home Assistant gone reports it, instead of a success.
func TestResyncSaysWhetherItPlanned(t *testing.T) {
	up := &fakeUp{}
	up.connected.Store(true)
	b := syncBridge(t, up)
	hold, held := make(chan struct{}), make(chan struct{})
	b.work <- func(context.Context) { close(held); <-hold }
	<-held
	errc := make(chan error, 1)
	go func() { errc <- b.Resync(context.Background()) }()
	waitFor(t, "the Resync queued", func() bool { return len(b.work) == 1 })
	up.connected.Store(false)
	close(hold)
	if err := <-errc; err == nil {
		t.Fatal("Resync reported success for a sync that did not plan")
	}
}

// TestSyncFailureInView (HR-P2-err-61, HR-P2-bound-37): a sync that keeps
// failing — its retry failed too — is in the read model with its error
// and since when; a sync that fails once is not.
func TestSyncFailureInView(t *testing.T) {
	up := &fakeUp{}
	up.connected.Store(true)
	b := New(nil, up, Options{Logger: slog.New(slog.DiscardHandler), Input: func() plan.Input { return plan.Input{} }})
	_ = b.sync(context.Background())
	if f := b.View().SyncFailure; f != nil {
		t.Fatalf("one failed sync is a warning: %+v", f)
	}
	_ = b.sync(context.Background())
	f := b.View().SyncFailure
	if f == nil || !strings.Contains(f.Message, errFakeRead.Error()) || f.Since.IsZero() {
		t.Fatalf("a sync that keeps failing is no warning: %+v", f)
	}
}
