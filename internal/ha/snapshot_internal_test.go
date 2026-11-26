// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package ha

import (
	"context"
	"errors"
	"testing"
	"time"
)

// failing is a Source whose every read fails.
type failing struct{}

var errFailing = errors.New("failing source")

func (failing) ListEntities(context.Context) ([]EntityEntry, error) { return nil, errFailing }
func (failing) ListDevices(context.Context) ([]DeviceEntry, error)  { return nil, errFailing }
func (failing) ListAreas(context.Context) ([]Area, error)           { return nil, errFailing }
func (failing) ListLabels(context.Context) ([]Label, error)         { return nil, errFailing }
func (failing) GetStates(context.Context) ([]State, error)          { return nil, errFailing }
func (failing) GetConfig(context.Context) (Config, error)           { return Config{}, errFailing }
func (failing) ListExposed(context.Context) (Exposed, error)        { return Exposed{}, errFailing }

// TestSnapshotsSameTickIsReused (HR-P4-conc-69, decision 79): a read
// that began in the trigger's clock tick is shared — on a coarse clock
// every read of one trigger does — and one that began before it is not.
func TestSnapshotsSameTickIsReused(t *testing.T) {
	at := time.Now()
	done := make(chan struct{})
	close(done)
	old := &Snapshot{}
	s := NewSnapshots(failing{})
	s.last[0] = &flight{began: at, done: done, snap: old}
	if got, err := s.Read(context.Background(), at, false); got != old || err != nil {
		t.Fatalf("a read of the trigger's tick was not shared: %p %v", got, err)
	}
	s.last[0] = &flight{began: at.Add(-time.Nanosecond), done: done, snap: old}
	got, err := s.Read(context.Background(), at, false)
	if got == old || !errors.Is(err, errFailing) {
		t.Fatalf("a read before the trigger was reused: %p %v", got, err)
	}
}
