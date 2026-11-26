// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package bridge

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"thingfabric.dev/go-ha2fabric/internal/camera"
	"thingfabric.dev/go-ha2fabric/internal/ha"

	"thingfabric.dev/go-ha2fabric/internal/plan"
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
	for i := range r.all {
		if r.all[i].Message == msg {
			out = append(out, r.all[i])
		}
	}
	return out
}

// ctxUp is fakeUp whose read ends with its context, or after a second.
type ctxUp struct{ *fakeUp }

func (c ctxUp) GetStates(ctx context.Context) ([]ha.State, error) {
	c.reads.Add(1)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(time.Second):
		return nil, errFakeRead
	}
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
	_ = b.sync(ctx, time.Now())
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
	b.command(ctx, rangeCmd("temperature_range", `{"target_temp_low":19,"target_temp_high":24}`))
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
	_ = b.sync(context.Background(), time.Now())
	if f := b.View().SyncFailure; f != nil {
		t.Fatalf("one failed sync is a warning: %+v", f)
	}
	_ = b.sync(context.Background(), time.Now())
	f := b.View().SyncFailure
	if f == nil || !strings.Contains(f.Message, errFakeRead.Error()) || f.Since.IsZero() {
		t.Fatalf("a sync that keeps failing is no warning: %+v", f)
	}
}

// TestOneEventOneSnapshot (HR-P2-bound-62): one registry event on the
// daemon's one connection reads Home Assistant once, not once per node:
// the bridges share the connection's reader (ha.Fanout.Snapshots).
func TestOneEventOneSnapshot(t *testing.T) {
	up := &fakeUp{}
	up.connected.Store(true)
	fan := &ha.Fanout{}
	const nodes = 9 // the configured node and NODE_LIMIT's default of 8
	for range nodes {
		b := syncBridge(t, up, func(o *Options) { o.Snapshots = fan.Snapshots(up) })
		fan.Add(b)
	}
	fan.Event(ha.Event{EventType: ha.EventEntityRegistryUpdated, Data: json.RawMessage(`{"action":"update","entity_id":"light.a"}`)})
	waitFor(t, "a sync", func() bool { return up.reads.Load() > 0 })
	time.Sleep(200 * time.Millisecond)
	if n := up.reads.Load(); n != 1 {
		t.Fatalf("one entity_registry_updated read the snapshot %d times", n)
	}
}

// TestDroppedCommandRestoresTheState (HR-P2-err-63): a controller's
// command that never reaches call_service (the queue full, or the
// command not mappable) carries Home Assistant's state onto the device
// again, as a failed call_service does: the controller already saw its
// value applied.
func TestDroppedCommandRestoresTheState(t *testing.T) {
	t.Run("queue full", func(t *testing.T) {
		b, _ := rangeBridge(t)
		for range cap(b.cmds) {
			b.cmds <- rangeCmd("temperature_range", `{"target_temp_low":19,"target_temp_high":24}`)
		}
		b.onCommand(rangeCmd("temperature_range", `{"target_temp_low":20,"target_temp_high":24}`))
		if len(b.work) == 0 && !b.owed.Load() {
			t.Fatal("a dropped command leaves the device at the controller's value: no re-apply and no sync owed")
		}
	})
	t.Run("unmapped", func(t *testing.T) {
		b, _ := rangeBridge(t)
		b.command(context.Background(), rangeCmd("no_such_method", "1"))
		if len(b.work) == 0 && !b.owed.Load() {
			t.Fatalf("an unmapped command leaves the device at the controller's value: no re-apply (status %q)", b.Status().LastCommand)
		}
	})
}

// dropped sums the dropped attribute of recs, one for a record without.
func dropped(recs []slog.Record) int64 {
	var n int64
	for i := range recs {
		c := int64(1)
		recs[i].Attrs(func(a slog.Attr) bool {
			if a.Key == "dropped" {
				c = a.Value.Int64()
			}
			return true
		})
		n += c
	}
	return n
}

// TestQueueFullLogsASummary (HR-P2-bound-64): a burst beyond the queue
// writes one line at its start and one summary with the count, not one
// Error line per dropped item, which would push everything else out of
// the operation page's ring.
func TestQueueFullLogsASummary(t *testing.T) {
	up := &fakeUp{}
	up.connected.Store(true)
	b, _ := floodBridge(t, up)
	rec := &records{}
	b.log = slog.New(rec)
	b.drops.work.log, b.drops.work.every = b.log, 30*time.Millisecond
	flood(b)
	if recs := rec.named("bridge.queue_full"); len(recs) != 1 {
		t.Fatalf("a burst of 76 dropped events wrote %d queue_full lines", len(recs))
	}
	waitFor(t, "the summary", func() bool { return len(rec.named("bridge.queue_full")) == 2 })
	if n := dropped(rec.named("bridge.queue_full")); n != 76 {
		t.Fatalf("the lines count %d dropped events, want 76", n)
	}
	time.Sleep(100 * time.Millisecond)
	if recs := rec.named("bridge.queue_full"); len(recs) != 2 {
		t.Fatalf("a quiet queue wrote more lines: %d", len(recs))
	}
}

// TestCommandDroppedLogsASummary (HR-P2-bound-64): the same for the
// command queue.
func TestCommandDroppedLogsASummary(t *testing.T) {
	b, _ := rangeBridge(t)
	rec := &records{}
	b.log = slog.New(rec)
	b.drops.cmds.log, b.drops.cmds.every = b.log, 30*time.Millisecond
	for range cap(b.cmds) + 10 {
		b.onCommand(rangeCmd("temperature_range", `{"target_temp_low":19,"target_temp_high":24}`))
	}
	if recs := rec.named("bridge.command_dropped"); len(recs) != 1 {
		t.Fatalf("10 dropped commands wrote %d command_dropped lines", len(recs))
	}
	waitFor(t, "the summary", func() bool { return len(rec.named("bridge.command_dropped")) == 2 })
	if n := dropped(rec.named("bridge.command_dropped")); n != 10 {
		t.Fatalf("the lines count %d dropped commands, want 10", n)
	}
}

// TestCameraCloseOffTheSync (HR-P2-conc-60): a camera host that left the
// bridge is closed beside the worker, so a session of a controller that
// went away does not hold the sync, and with it every state; Run's end
// waits for the close.
func TestCameraCloseOffTheSync(t *testing.T) {
	release, closing := make(chan struct{}), make(chan struct{}, 1)
	defer func(f func(*camera.Host, context.Context)) { closeHost = f }(closeHost)
	closeHost = func(*camera.Host, context.Context) { closing <- struct{}{}; <-release }
	up := &fakeUp{}
	b := New(nil, up, Options{Logger: slog.New(slog.DiscardHandler), Input: func() plan.Input { return plan.Input{} }})
	b.hosts = map[string]*camera.Host{"camera.gone": camera.New("camera.gone", nil, camera.Options{})}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { b.closeCameras(ctx); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("the sync waited for a camera host's close")
	}
	<-closing
	ran := make(chan struct{})
	go func() { _ = b.Run(ctx); close(ran) }()
	cancel()
	select {
	case <-ran:
		t.Fatal("Run ended before the camera host's close")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	<-ran
}
