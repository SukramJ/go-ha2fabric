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
	"github.com/SukramJ/go-hamqtt/model"
	"thingfabric.dev/go-fabric/device"

	"github.com/SukramJ/go-ha2fabric/internal/adapt"

	"github.com/SukramJ/go-ha2fabric/internal/ha"
	"github.com/SukramJ/go-ha2fabric/internal/plan"
)

// fakeUp is an upstream whose connection the test sets and whose every
// read fails; a sync that reaches Home Assistant counts one read.
type fakeUp struct {
	connected atomic.Bool
	reads     atomic.Int32
	call      func(ha.ServiceCall) error
	calls     chan ha.ServiceCall
}

var errFakeRead = errors.New("fake: no snapshot")

func (f *fakeUp) ListEntities(context.Context) ([]ha.EntityEntry, error) { return nil, errFakeRead }
func (f *fakeUp) ListDevices(context.Context) ([]ha.DeviceEntry, error)  { return nil, errFakeRead }
func (f *fakeUp) ListAreas(context.Context) ([]ha.Area, error)           { return nil, errFakeRead }
func (f *fakeUp) ListLabels(context.Context) ([]ha.Label, error)         { return nil, errFakeRead }

// GetStates is a snapshot's first read (HR-P1-data-67), so it is the one
// counted.
func (f *fakeUp) GetStates(context.Context) ([]ha.State, error) {
	f.reads.Add(1)
	return nil, errFakeRead
}

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

// TestOwedAfterDrain (HR-P1-conc-96): the overflow's failed send and its
// mark are two steps, so the worker can drain the queue between them and
// wait with a sync owed. Built here by marking the sync owed while the
// worker waits on an empty queue: the sync still runs.
func TestOwedAfterDrain(t *testing.T) {
	up := &fakeUp{}
	up.connected.Store(true)
	b, _ := floodBridge(t, up)
	run(t, b)
	waitFor(t, "the worker waiting", func() bool { return len(b.work) == 0 })
	b.owe()
	waitFor(t, "the owed sync", func() bool { return up.reads.Load() > 0 })
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

// rangeBridge is a bridge whose climate.a is in heat_cool with the range
// 18 to 24, on an upstream that refuses a low above the high, as Home
// Assistant does (climate/services.py:121-131 at 2026.10.3).
func rangeBridge(t *testing.T) (*Bridge, *fakeUp) {
	t.Helper()
	up := &fakeUp{calls: make(chan ha.ServiceCall, 16)}
	up.connected.Store(true)
	up.call = func(sc ha.ServiceCall) error {
		lo, okL := sc.Data["target_temp_low"].(float64)
		hi, okH := sc.Data["target_temp_high"].(float64)
		if okL && okH && lo > hi {
			return errors.New("low_temp_higher_than_high_temp")
		}
		return nil
	}
	b := New(nil, up, Options{Logger: slog.New(slog.DiscardHandler), Input: func() plan.Input { return plan.Input{} }})
	b.states["climate.a"] = &ha.State{EntityID: "climate.a", State: "heat_cool", Attributes: map[string]any{"target_temp_low": 18.0, "target_temp_high": 24.0}}
	b.serving = map[string]bool{"climate.a": true}
	// served: the bridge keeps a served entity's states only.
	b.owner["climate.a"] = &group{Group: &plan.Group{Key: "a"}}
	return b, up
}

func rangeCmd(method, payload string) model.Command {
	st := &ha.State{EntityID: "climate.a", State: "heat_cool", Attributes: map[string]any{}}
	e := adapt.Entity(adapt.Input{EntityID: "climate.a", State: st, Device: adapt.OwnDevice("climate.a", "a"), TemperatureUnit: "°C"})
	return model.Command{Entity: e, Method: method, Payload: []byte(payload)}
}

// rangeCalls runs the commands in order and returns the calls made.
func rangeCalls(t *testing.T, b *Bridge, up *fakeUp, cmds ...model.Command) []ha.ServiceCall {
	t.Helper()
	var out []ha.ServiceCall
	for i := range cmds {
		c := &cmds[i]
		b.command(t.Context(), *c)
		select {
		case sc := <-up.calls:
			out = append(out, sc)
		default:
			t.Fatalf("no call for %s %s", c.Method, c.Payload)
		}
	}
	return out
}

func bounds(sc ha.ServiceCall) [2]any {
	return [2]any{sc.Data["target_temp_low"], sc.Data["target_temp_high"]}
}

// TestRangeSetpointsInSequence (HR-P1-adpt-30): a controller's two
// setpoint writes in Auto are two commands; the second completes from
// the bounds the first sent, not from the state Home Assistant has not
// reported yet, and no call carries a low above its high.
func TestRangeSetpointsInSequence(t *testing.T) {
	t.Run("raise past the old high", func(t *testing.T) {
		b, up := rangeBridge(t)
		calls := rangeCalls(t, b, up, rangeCmd(hamapping.MethodTemperatureLow, "25"), rangeCmd(hamapping.MethodTemperatureHigh, "28"))
		for _, c := range calls {
			if lo, hi := c.Data["target_temp_low"].(float64), c.Data["target_temp_high"].(float64); lo > hi {
				t.Errorf("a call with low %v above high %v", lo, hi)
			}
		}
		if got := bounds(calls[len(calls)-1]); got != [2]any{25.0, 28.0} {
			t.Fatalf("last call %v", got)
		}
		if s := b.Status(); s.CommandErrs != 0 {
			t.Fatalf("status %+v", s)
		}
	})
	t.Run("state not yet reported", func(t *testing.T) {
		b, up := rangeBridge(t)
		calls := rangeCalls(t, b, up, rangeCmd(hamapping.MethodTemperatureLow, "20"), rangeCmd(hamapping.MethodTemperatureHigh, "26"))
		if got := bounds(calls[1]); got != [2]any{20.0, 26.0} {
			t.Fatalf("last call %v", got)
		}
	})
	t.Run("state shows them, then moves", func(t *testing.T) {
		b, up := rangeBridge(t)
		rangeCalls(t, b, up, rangeCmd(hamapping.MethodTemperatureLow, "20"))
		// Home Assistant reports the call, then the range moves there.
		report := func(low, high float64) {
			b.stateChanged(t.Context(), ha.StateChanged{EntityID: "climate.a", NewState: &ha.State{EntityID: "climate.a", State: "heat_cool", Attributes: map[string]any{"target_temp_low": low, "target_temp_high": high}}})
		}
		report(20, 24)
		rangeCalls(t, b, up, rangeCmd(hamapping.MethodTemperatureHigh, "25"))
		report(20, 25)
		report(17, 25)
		calls := rangeCalls(t, b, up, rangeCmd(hamapping.MethodTemperatureHigh, "23"))
		if got := bounds(calls[0]); got != [2]any{17.0, 23.0} {
			t.Fatalf("after the state moved %v", got)
		}
	})
}
