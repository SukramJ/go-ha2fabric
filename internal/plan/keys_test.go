// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package plan_test

import (
	"context"
	"testing"

	hamapping "sukramj.dev/go-hamapping"

	"thingfabric.dev/go-ha2fabric/internal/expose"
	"thingfabric.dev/go-ha2fabric/internal/ha"
	"thingfabric.dev/go-ha2fabric/internal/plan"
)

// TestNoEmptyKey (decision 75, phase 4 P's v0.27.0 question): every
// endpoint and part this bridge builds has a stable key that renders
// non-empty, also for hostile entity ids and an empty registry id —
// go-fabric refuses one at Add and Replace with ErrNoStableKey since
// v0.27.0. It pins the answer at every go-hamapping bump.
func TestNoEmptyKey(t *testing.T) {
	s := snapshot(t)
	attrs := map[string]any{"device_class": "smoke", "friendly_name": ""}
	for _, id := range []string{"", ".", "binary_sensor.", "binary_sensor", "light."} {
		s.States = append(s.States, ha.State{EntityID: id, State: "off", Attributes: attrs})
	}
	// A registry entry whose id is empty, on a registry device.
	s.Entities = append(s.Entities, ha.EntityEntry{EntityID: "binary_sensor.noid", DeviceID: s.Devices[0].ID})
	s.States = append(s.States, ha.State{EntityID: "binary_sensor.noid", State: "off", Attributes: attrs})
	in := plan.Input{Rule: expose.Rule{Include: []string{"*"}}}
	_, groups := plan.Assess(s, in)
	n := 0
	for _, g := range groups {
		p, _ := hamapping.Map(g.Device, g.Entities, plan.MapOptions(g.Regs, g.Names, nil))
		devs, err := p.Build(context.Background())
		if err != nil {
			continue
		}
		for _, d := range devs {
			spec := d.Spec()
			// Typed by Parts, so the test stays on go-fabric's facade.
			stack := spec.Parts[:0:0]
			stack = append(stack, spec)
			for len(stack) > 0 {
				sp := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				n++
				if sp.StableKey == nil || sp.StableKey.String() == "" {
					t.Errorf("group %q: an empty key", g.Key)
				}
				stack = append(stack, sp.Parts...)
			}
		}
		p.Close()
	}
	if n < 30 {
		t.Fatalf("%d endpoints and parts: the fixtures were not built", n)
	}
}
