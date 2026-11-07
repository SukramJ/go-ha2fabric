// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package ha_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SukramJ/go-ha2fabric/internal/ha"
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

// TestReadSnapshot: each read's failure names its command, and the
// exposure list is read only for a rule that follows Assist.
func TestReadSnapshot(t *testing.T) {
	for i, cmd := range []string{
		"config/entity_registry/list", "config/device_registry/list", "config/area_registry/list",
		"config/label_registry/list", "get_states", "get_config", "homeassistant/expose_entity/list",
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
