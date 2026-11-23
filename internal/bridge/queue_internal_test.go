// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package bridge

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	hamapping "github.com/SukramJ/go-hamapping"
	"thingfabric.dev/go-fabric/device"

	"github.com/SukramJ/go-ha2fabric/internal/adapt"

	"github.com/SukramJ/go-ha2fabric/internal/ha"
	"github.com/SukramJ/go-ha2fabric/internal/plan"
)

// fakeUp is an upstream whose connection the test sets and whose every
// read fails, counted: a sync that reaches Home Assistant is one read.
type fakeUp struct {
	connected atomic.Bool
	reads     atomic.Int32
	call      func(ha.ServiceCall) error
	calls     chan ha.ServiceCall
}

var errFakeRead = errors.New("fake: no snapshot")

func (f *fakeUp) ListEntities(context.Context) ([]ha.EntityEntry, error) {
	f.reads.Add(1)
	return nil, errFakeRead
}
func (f *fakeUp) ListDevices(context.Context) ([]ha.DeviceEntry, error) { return nil, errFakeRead }
func (f *fakeUp) ListAreas(context.Context) ([]ha.Area, error)          { return nil, errFakeRead }
func (f *fakeUp) ListLabels(context.Context) ([]ha.Label, error)        { return nil, errFakeRead }
func (f *fakeUp) GetStates(context.Context) ([]ha.State, error)         { return nil, errFakeRead }
func (f *fakeUp) GetConfig(context.Context) (ha.Config, error) { return ha.Config{}, errFakeRead }

func (f *fakeUp) ListExposed(context.Context) (ha.Exposed, error) { return ha.Exposed{}, errFakeRead }

func (f *fakeUp) Connected() bool { return f.connected.Load() }

func (f *fakeUp) CallService(_ context.Context, sc ha.ServiceCall) error {
	if f.calls != nil {
		f.calls <- sc
	}
	if f.call != nil {
		return f.call(sc)
	}
	return nil
}

// floodBridge is a bridge on up whose worker is not running, with one
// served device for light.a and a queue the test fills.
func floodBridge(t *testing.T, up *fakeUp) (*Bridge, *device.Device) {
	t.Helper()
	b := New(nil, up, Options{Debounce: 5 * time.Millisecond, Logger: slog.New(slog.DiscardHandler), Input: func() plan.Input { return plan.Input{} }})
	b.connected.Store(up.Connected())
	plug, err := device.NewOnOffPlugInUnit(device.Options{StableKey: device.StringKey("ha:dev/a"), Name: "a"})
	if err != nil {
		t.Fatal(err)
	}
	// known without an owner: draining a state_changed arms nothing.
	b.known["light.a"] = true
	hp, _ := hamapping.Map(adapt.OwnDevice("light.a", "a"), nil)
	b.groups["dev"] = &group{Group: &plan.Group{Key: "dev"}, plan: hp, devices: []*device.Device{plug.Device}, keys: []device.SourceKey{device.StringKey("ha:dev/a")}}
	return b, plug.Device
}

// flood fills the worker's queue past its capacity with state_changed
// events of light.a.
func flood(b *Bridge) {
	ev := ha.Event{EventType: ha.EventStateChanged, Data: []byte(`{"entity_id":"light.a","new_state":{"entity_id":"light.a","state":"on"}}`)}
	for range cap(b.work) + 76 {
		b.Event(ev)
	}
}

// run starts the worker and stops it at the test's end.
func run(t *testing.T, b *Bridge) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = b.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("never: %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// TestQueueFullResyncIsOwed (HR-P1-conc-32, G-B's probe): the overflow
// promises a re-sync; its debounce fires while the queue is still full,
// and the sync runs all the same once the worker has drained the queue.
func TestQueueFullResyncIsOwed(t *testing.T) {
	up := &fakeUp{}
	up.connected.Store(true)
	b, _ := floodBridge(t, up)
	flood(b)
	time.Sleep(50 * time.Millisecond) // the debounce fires on a full queue
	run(t, b)
	waitFor(t, "the queue drained", func() bool { return len(b.work) == 0 })
	waitFor(t, "a sync after the overflow", func() bool { return up.reads.Load() > 0 })
}

// TestQueueFullDisconnected (HR-P1-conc-32): a Disconnected while the
// queue is full still leaves every endpoint unreachable.
func TestQueueFullDisconnected(t *testing.T) {
	up := &fakeUp{}
	up.connected.Store(true)
	b, d := floodBridge(t, up)
	flood(b)
	up.connected.Store(false)
	b.Disconnected(errors.New("dropped"))
	run(t, b)
	waitFor(t, "the endpoint unreachable", func() bool { return !d.Reachable() })
}

// TestQueueFullRemoveIsForgotten (HR-P1-conc-32): an entity removed from
// the registry while the queue is full is still forgotten at the next
// sync, not only removed.
func TestQueueFullRemoveIsForgotten(t *testing.T) {
	up := &fakeUp{}
	up.connected.Store(true)
	b, _ := floodBridge(t, up)
	b.last = &ha.Snapshot{Entities: []ha.EntityEntry{{ID: "reg-a", EntityID: "light.a"}}}
	flood(b)
	b.Event(ha.Event{EventType: ha.EventEntityRegistryUpdated, Data: []byte(`{"action":"remove","entity_id":"light.a"}`)})
	run(t, b)
	waitFor(t, "the queue drained", func() bool { return len(b.work) == 0 })
	b.mu.Lock()
	k := b.forget["light.a"]
	b.mu.Unlock()
	if k == nil || k.String() != "ha-entity:light.a/reg-a" {
		t.Fatalf("forget %v", k)
	}
}
