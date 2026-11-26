// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package bridge

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	hamapping "sukramj.dev/go-hamapping"
	"thingfabric.dev/go-fabric/device"
	"thingfabric.dev/go-fabric/eligibility"

	"thingfabric.dev/go-ha2fabric/internal/expose"
	"thingfabric.dev/go-ha2fabric/internal/ha"
	"thingfabric.dev/go-ha2fabric/internal/plan"
)

// TestBuildRefusedWholeDevice (HR-P3-test-15): a build that fails with
// an error go-hamapping does not attribute to one entity leaves the whole
// device without endpoints, and every entity of it unmappable with the
// refusal as its reason.
func TestBuildRefusedWholeDevice(t *testing.T) {
	refusal := errors.New("the sentinel refusal")
	buildPlan = func(*hamapping.Plan, context.Context) ([]*device.Device, error) { return nil, refusal }
	t.Cleanup(func() { buildPlan = (*hamapping.Plan).Build })

	snap := &ha.Snapshot{
		States: []ha.State{
			{EntityID: "light.a", State: "on", Attributes: map[string]any{"supported_color_modes": []any{"onoff"}, "color_mode": "onoff"}},
			{EntityID: "switch.b", State: "off"},
		},
		Entities: []ha.EntityEntry{
			{ID: "r1", EntityID: "light.a", DeviceID: "d1"},
			{ID: "r2", EntityID: "switch.b", DeviceID: "d1"},
		},
		Devices: []ha.DeviceEntry{{ID: "d1", Name: "Desk"}},
	}
	_, groups := plan.Assess(snap, plan.Input{Rule: expose.Rule{Include: []string{"*"}}})
	if len(groups) != 1 || len(groups[0].Entities) != 2 {
		t.Fatalf("groups %+v", groups)
	}
	b := New(nil, &fakeUp{}, Options{Logger: slog.New(slog.DiscardHandler), Input: func() plan.Input { return plan.Input{} }})
	g, mv := b.build(context.Background(), groups[0])
	if len(g.devices) != 0 {
		t.Fatalf("%d devices served after a refused build", len(g.devices))
	}
	if len(mv) != 2 {
		t.Fatalf("verdicts %+v", mv)
	}
	for _, v := range mv {
		if v.State != eligibility.StateUnmappable || !strings.Contains(v.Reason, "go-fabric refused the device: the sentinel refusal") {
			t.Errorf("%s: %s %q", v.Key, v.State, v.Reason)
		}
	}
}
