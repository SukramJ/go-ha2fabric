// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package ha_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"thingfabric.dev/go-ha2fabric/internal/ha"
)

// failing is a Source whose read number fail fails; every other read
// answers empty.
type failing struct {
	fail  int
	reads int
}

var errRead = errors.New("boom")

func (f *failing) read() error {
	f.reads++
	if f.reads == f.fail {
		return errRead
	}
	return nil
}

func (f *failing) ListEntities(context.Context) ([]ha.EntityEntry, error) { return nil, f.read() }
func (f *failing) ListDevices(context.Context) ([]ha.DeviceEntry, error)  { return nil, f.read() }
func (f *failing) ListAreas(context.Context) ([]ha.Area, error)           { return nil, f.read() }
func (f *failing) ListLabels(context.Context) ([]ha.Label, error)         { return nil, f.read() }
func (f *failing) GetStates(context.Context) ([]ha.State, error)          { return nil, f.read() }
func (f *failing) GetConfig(context.Context) (ha.Config, error)           { return ha.Config{}, f.read() }

func (f *failing) ListExposed(context.Context) (ha.Exposed, error) { return ha.Exposed{}, f.read() }

// TestReadSnapshot: each read's failure names its command, the states
// are read before the registries (HR-P1-data-67: an entity registered
// between the two reads has no state yet and stays out until its
// state_changed, rather than being planned once as unregistered), and the
// exposure list is read only for a rule that follows Assist.
func TestReadSnapshot(t *testing.T) {
	for i, cmd := range []string{
		"get_states", "config/entity_registry/list", "config/device_registry/list", "config/area_registry/list",
		"config/label_registry/list", "get_config", "homeassistant/expose_entity/list",
	} {
		_, err := ha.ReadSnapshot(context.Background(), &failing{fail: i + 1}, true)
		if !errors.Is(err, errRead) || !strings.HasPrefix(err.Error(), cmd+": ") {
			t.Errorf("read %d: %v, want %s", i+1, err, cmd)
		}
	}
	src := &failing{fail: 7}
	if s, err := ha.ReadSnapshot(context.Background(), src, false); err != nil || s == nil || src.reads != 6 {
		t.Fatalf("without Assist: %v, %d reads", err, src.reads)
	}
}

// gatedSource answers every read empty; GetStates counts the snapshots
// and waits for gate while it is set.
type gatedSource struct {
	gate  chan struct{}
	reads atomic.Int32
}

func (*gatedSource) ListEntities(context.Context) ([]ha.EntityEntry, error) { return nil, nil }

func (*gatedSource) ListDevices(context.Context) ([]ha.DeviceEntry, error) { return nil, nil }

func (*gatedSource) ListAreas(context.Context) ([]ha.Area, error) { return nil, nil }

func (*gatedSource) ListLabels(context.Context) ([]ha.Label, error) { return nil, nil }

func (*gatedSource) GetConfig(context.Context) (ha.Config, error) { return ha.Config{}, nil }

func (*gatedSource) ListExposed(context.Context) (ha.Exposed, error) { return ha.Exposed{}, nil }

func (g *gatedSource) GetStates(context.Context) ([]ha.State, error) {
	g.reads.Add(1)
	if g.gate != nil {
		<-g.gate
	}
	return nil, nil
}

// TestSnapshotsShared (HR-P2-bound-62): the connection's bridges share
// one read. A caller joins the read in flight, or the last finished one,
// when it began after the caller's trigger; a read for the exposure list
// serves a caller without it, not the other way round; a trigger after a
// read began starts a new one.
func TestSnapshotsShared(t *testing.T) {
	src := &gatedSource{gate: make(chan struct{})}
	snaps := ha.NewSnapshots(src)
	ctx := context.Background()
	trigger := time.Now()
	var wg sync.WaitGroup
	got := make([]*ha.Snapshot, 9)
	for i := range got {
		wg.Go(func() {
			s, err := snaps.Read(ctx, trigger, i%2 == 0)
			if err != nil {
				t.Error(err)
			}
			got[i] = s
		})
	}
	time.Sleep(50 * time.Millisecond)
	close(src.gate)
	wg.Wait()
	// The first caller may start a read without the exposure list before
	// one with it starts; never more than the two.
	if n := src.reads.Load(); n > 2 {
		t.Fatalf("9 callers of one trigger read %d snapshots", n)
	}
	before := src.reads.Load()
	if s, _ := snaps.Read(ctx, trigger, true); s != got[0] {
		t.Fatal("a caller of the same trigger did not get the finished read")
	}
	if s, _ := snaps.Read(ctx, trigger, false); s == nil {
		t.Fatal("no snapshot")
	}
	if n := src.reads.Load(); n != before {
		t.Fatalf("a finished read late enough was read again: %d, then %d", before, n)
	}
	if _, err := snaps.Read(ctx, time.Now(), false); err != nil || src.reads.Load() != before+1 {
		t.Fatalf("a later trigger did not read again: %v, %d read(s)", err, src.reads.Load())
	}
}

// TestSnapshotsWaitEndsWithCaller: a caller whose context ends stops
// waiting for the shared read, which goes on for the others.
func TestSnapshotsWaitEndsWithCaller(t *testing.T) {
	src := &gatedSource{gate: make(chan struct{})}
	snaps := ha.NewSnapshots(src)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := snaps.Read(ctx, time.Now(), false); !errors.Is(err, context.Canceled) {
		t.Fatalf("Read with an ended context: %v", err)
	}
	close(src.gate)
}

// TestFanoutSnapshots: the connection's fan-out hands every node the one
// reader.
func TestFanoutSnapshots(t *testing.T) {
	var f ha.Fanout
	src := &gatedSource{}
	if a, b := f.Snapshots(src), f.Snapshots(src); a == nil || a != b {
		t.Fatal("the fan-out's reader is not one")
	}
}
