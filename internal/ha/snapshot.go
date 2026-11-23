// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package ha

import (
	"context"
	"errors"
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
	// Skipped are the registry entries that did not decode, by id
	// ([SkippedError]): the entities and the devices whose entities a sync
	// cannot judge, and the labels a rule cannot resolve.
	Skipped Skipped
}

// Skipped are a snapshot's registry entries that did not decode, by id.
type Skipped struct {
	Entities, Devices, Labels []string
}

// ReadSnapshot reads the states, the registries, the configuration and —
// when assist is set, the exposure rule following Assist — the exposure
// list. A registry entry that does not decode is in Skipped by its id,
// not in the lists.
func ReadSnapshot(ctx context.Context, src Source, assist bool) (*Snapshot, error) {
	var s Snapshot
	var err error
	// The states first: an entity registered between the reads then has
	// no state yet and stays out until its state_changed re-syncs, instead
	// of being planned once without its registry entry (HR-P1-data-67).
	if s.States, err = src.GetStates(ctx); err != nil {
		return nil, fmt.Errorf("get_states: %w", err)
	}
	if s.Entities, err = src.ListEntities(ctx); err != nil {
		if s.Skipped.Entities, err = skippedIDs(err); err != nil {
			return nil, fmt.Errorf("config/entity_registry/list: %w", err)
		}
	}
	if s.Devices, err = src.ListDevices(ctx); err != nil {
		if s.Skipped.Devices, err = skippedIDs(err); err != nil {
			return nil, fmt.Errorf("config/device_registry/list: %w", err)
		}
	}
	if s.Areas, err = src.ListAreas(ctx); err != nil {
		return nil, fmt.Errorf("config/area_registry/list: %w", err)
	}
	if s.Labels, err = src.ListLabels(ctx); err != nil {
		if s.Skipped.Labels, err = skippedIDs(err); err != nil {
			return nil, fmt.Errorf("config/label_registry/list: %w", err)
		}
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

// skippedIDs is a list's skipped ids when err is a [*SkippedError], else
// err.
func skippedIDs(err error) ([]string, error) {
	if se, ok := errors.AsType[*SkippedError](err); ok {
		return se.IDs, nil
	}
	return nil, err
}

var _ Source = (*Client)(nil)
