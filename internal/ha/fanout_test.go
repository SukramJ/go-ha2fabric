// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package ha_test

import (
	"errors"
	"slices"
	"sync"
	"testing"

	"thingfabric.dev/go-ha2fabric/internal/ha"
)

// callLog is a Handler that records its calls.
type callLog struct {
	mu    sync.Mutex
	calls []string
}

func (r *callLog) add(s string) {
	r.mu.Lock()
	r.calls = append(r.calls, s)
	r.mu.Unlock()
}

func (r *callLog) Connected(v string) { r.add("connected " + v) }
func (r *callLog) Event(ev ha.Event)  { r.add("event " + ev.EventType) }
func (r *callLog) Disconnected(error) { r.add("disconnected") }
func (r *callLog) got() []string      { r.mu.Lock(); defer r.mu.Unlock(); return slices.Clone(r.calls) }

func TestFanout(t *testing.T) {
	var f ha.Fanout
	a, b := &callLog{}, &callLog{}
	f.Add(a)
	f.Add(a) // twice is once
	f.Connected("2026.10.3")
	f.Event(ha.Event{EventType: ha.EventStateChanged})
	// b joins a connection that is up: it is told at once.
	f.Add(b)
	f.Event(ha.Event{EventType: ha.EventLabelRegistryUpdated})
	if f.Len() != 2 {
		t.Fatalf("Len %d", f.Len())
	}
	f.Remove(a)
	f.Disconnected(errors.New("gone"))
	// A handler added while the connection is down waits for Connected.
	c := &callLog{}
	f.Add(c)
	f.Connected("2026.10.4")

	want := map[*callLog][]string{
		a: {"connected 2026.10.3", "event state_changed", "event label_registry_updated"},
		b: {"connected 2026.10.3", "event label_registry_updated", "disconnected", "connected 2026.10.4"},
		c: {"connected 2026.10.4"},
	}
	for r, w := range want {
		if got := r.got(); !slices.Equal(got, w) {
			t.Errorf("got %q, want %q", got, w)
		}
	}
}

// TestFanoutConcurrent (HR-P1-test-70): Add and Remove racing Event —
// the registry nodes joining and leaving a running connection — keep each
// handler's order: Connected first, nothing after it was removed.
func TestFanoutConcurrent(t *testing.T) {
	f := &ha.Fanout{}
	f.Connected("2026.10.3")
	var wg sync.WaitGroup
	stop := make(chan struct{})
	wg.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
				f.Event(ha.Event{EventType: "state_changed"})
			}
		}
	})
	logs := make([]*callLog, 50)
	for i := range logs {
		wg.Go(func() {
			r := &callLog{}
			logs[i] = r
			f.Add(r)
			f.Remove(r)
		})
	}
	close(stop)
	wg.Wait()
	for i, r := range logs {
		got := r.got()
		n := len(got)
		f.Event(ha.Event{EventType: "after"})
		if len(r.got()) != n {
			t.Fatalf("handler %d got an event after its Remove", i)
		}
		if n == 0 || got[0] != "connected 2026.10.3" {
			t.Fatalf("handler %d began with %q", i, got[0])
		}
	}
	if f.Len() != 0 {
		t.Fatalf("%d handlers left", f.Len())
	}
}
