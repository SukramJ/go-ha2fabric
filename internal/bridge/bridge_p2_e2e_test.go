// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package bridge_test

import (
	"testing"

	"github.com/SukramJ/go-ha2fabric/internal/bridge"
	"github.com/SukramJ/go-ha2fabric/internal/expose"
)

// TestStatesHeldOnce (HR-P2-bound-38, the bridge half): the bridge holds
// beside its snapshot only the states of the entities it serves; every
// other state is the snapshot's alone, and a state_changed of an entity
// it does not serve adds nothing.
func TestStatesHeldOnce(t *testing.T) {
	e := newEnv(t, expose.Rule{Include: []string{"light.kitchen"}})
	e.commission()
	if _, ok := e.endpoint("light.kitchen"); !ok {
		t.Fatal("light.kitchen not served")
	}
	all := len(e.br.View().Snapshot.States)
	if n := bridge.HeldStates(e.br); n != 1 {
		t.Fatalf("the bridge holds %d states beside the snapshot's %d; it serves one entity", n, all)
	}
	e.ha.SetState("binary_sensor.front_door", "on", map[string]any{"device_class": "door"})
	e.ha.SetState("light.kitchen", "off", e.ha.State("light.kitchen")["attributes"].(map[string]any))
	e.eventually("light.kitchen's state applied", func() bool {
		_, ok := e.endpoint("light.kitchen")
		return ok && !e.onOff(e.mustEndpoint("light.kitchen"))
	})
	if n := bridge.HeldStates(e.br); n != 1 {
		t.Fatalf("after a state of an entity it does not serve, the bridge holds %d states", n)
	}
}
