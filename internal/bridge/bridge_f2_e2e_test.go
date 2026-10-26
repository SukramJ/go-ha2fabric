// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package bridge_test

import (
	"testing"

	"github.com/SukramJ/go-ha2fabric/internal/expose"
)

// TestRemovedEntityIsForgotten: an entity removed from Home Assistant's
// registry loses its endpoint number — re-added with its old registry id,
// it gets a fresh one — while a disabled and re-enabled entity keeps its
// number (issue #10, the owner's decision on F1 Q1).
func TestRemovedEntityIsForgotten(t *testing.T) {
	e := newEnv(t, expose.Rule{Assist: true})
	e.commission()
	plug, ok := e.endpoint("switch.coffee")
	if !ok {
		t.Fatal("no plug")
	}
	key := "ha:dev-shelly-1/" + e.regID("switch.coffee")

	// Disabled, then enabled again: the same number.
	e.ha.UpdateEntity("switch.coffee", map[string]any{"disabled_by": "user"})
	e.eventually("the disabled plug is gone", func() bool {
		_, ok := e.endpoint("switch.coffee")
		return !ok
	})
	e.ha.UpdateEntity("switch.coffee", map[string]any{"disabled_by": nil})
	e.eventually("the re-enabled plug is back", func() bool {
		_, ok := e.endpoint("switch.coffee")
		return ok
	})
	if again, _ := e.endpoint("switch.coffee"); again != plug {
		t.Fatalf("the re-enabled plug moved from %d to %d", plug, again)
	}

	// Removed, then created again with the same registry id: a fresh
	// number, as a new device would get.
	e.ha.RemoveEntity("switch.coffee")
	e.eventually("the removed plug is gone", func() bool {
		_, ok := e.dev.EndpointOf(key)
		return !ok
	})
	e.ha.RestoreEntity("switch.coffee")
	e.eventually("the re-added plug is back", func() bool {
		_, ok := e.endpoint("switch.coffee")
		return ok
	})
	again, _ := e.endpoint("switch.coffee")
	if again == plug {
		t.Fatalf("the re-added plug got its old number %d", plug)
	}
	if got, ok := e.dev.EndpointOf(key); !ok || got != again {
		t.Fatalf("stable key %s: %d %v, plan %d", key, got, ok, again)
	}

	// An entity the bridge does not serve when it is removed (disabled)
	// has its stored number freed all the same.
	light, ok := e.endpoint("light.kitchen")
	if !ok {
		t.Fatal("no light")
	}
	e.ha.UpdateEntity("light.kitchen", map[string]any{"disabled_by": "user"})
	e.eventually("the disabled light is gone", func() bool {
		_, ok := e.endpoint("light.kitchen")
		return !ok
	})
	synced := e.br.Status().LastSync
	e.ha.RemoveEntity("light.kitchen")
	e.eventually("the removal is synced", func() bool { return e.br.Status().LastSync.After(synced) })
	e.ha.RestoreEntity("light.kitchen")
	e.ha.UpdateEntity("light.kitchen", map[string]any{"disabled_by": nil})
	e.eventually("the re-added light is back", func() bool {
		_, ok := e.endpoint("light.kitchen")
		return ok
	})
	if got, _ := e.endpoint("light.kitchen"); got == light {
		t.Fatalf("the re-added light got its old number %d", light)
	}
}
