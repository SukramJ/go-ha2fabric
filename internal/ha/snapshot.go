// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package ha

import (
	"context"
	"fmt"
)

// Source is the reads one snapshot takes from Home Assistant; *Client is
// the daemon's.
type Source interface {
	ListEntities(ctx context.Context) ([]EntityEntry, error)
	ListDevices(ctx context.Context) ([]DeviceEntry, error)
	ListAreas(ctx context.Context) ([]Area, error)
	ListLabels(ctx context.Context) ([]Label, error)
	GetStates(ctx context.Context) ([]State, error)
	GetConfig(ctx context.Context) (Config, error)
	ListExposed(ctx context.Context) (Exposed, error)
}

// Snapshot is what one read of Home Assistant returned: the registries,
// the states, the configuration and the Assist exposure list.
type Snapshot struct {
	Entities []EntityEntry
	Devices  []DeviceEntry
	Areas    []Area
	Labels   []Label
	States   []State
	Config   Config
	Exposed  Exposed
}

// ReadSnapshot reads the registries, the states, the configuration and —
// when assist is set, the exposure rule following Assist — the exposure
// list.
func ReadSnapshot(ctx context.Context, src Source, assist bool) (*Snapshot, error) {
	var s Snapshot
	var err error
	if s.Entities, err = src.ListEntities(ctx); err != nil {
		return nil, fmt.Errorf("config/entity_registry/list: %w", err)
	}
	if s.Devices, err = src.ListDevices(ctx); err != nil {
		return nil, fmt.Errorf("config/device_registry/list: %w", err)
	}
	if s.Areas, err = src.ListAreas(ctx); err != nil {
		return nil, fmt.Errorf("config/area_registry/list: %w", err)
	}
	if s.Labels, err = src.ListLabels(ctx); err != nil {
		return nil, fmt.Errorf("config/label_registry/list: %w", err)
	}
	if s.States, err = src.GetStates(ctx); err != nil {
		return nil, fmt.Errorf("get_states: %w", err)
	}
	if s.Config, err = src.GetConfig(ctx); err != nil {
		return nil, fmt.Errorf("get_config: %w", err)
	}
	if assist {
		if s.Exposed, err = src.ListExposed(ctx); err != nil {
			return nil, fmt.Errorf("homeassistant/expose_entity/list: %w", err)
		}
	}
	return &s, nil
}

var _ Source = (*Client)(nil)
