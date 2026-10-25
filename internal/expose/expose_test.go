// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package expose

import (
	"slices"
	"testing"
)

func TestDecide(t *testing.T) {
	assist := Rule{Assist: true, Include: []string{"sensor.*_temperature", "switch.extra"}, Exclude: []string{"light.private*"}}
	plain := Rule{Include: []string{"light.kitchen"}}
	for _, c := range []struct {
		rule Rule
		e    Entity
		want bool
		why  string
	}{
		{assist, Entity{EntityID: "light.kitchen", Registered: true, ExposedToAssist: true}, true, "exposed to Assist"},
		{assist, Entity{EntityID: "light.kitchen", Registered: true}, false, "not exposed to Assist and not included"},
		{assist, Entity{EntityID: "sensor.living_temperature"}, true, "included by sensor.*_temperature"},
		{assist, Entity{EntityID: "switch.extra"}, true, "included by switch.extra"},
		{assist, Entity{EntityID: "light.private_room", ExposedToAssist: true}, false, "excluded by light.private*"},
		{assist, Entity{EntityID: "light.cellar", DisabledBy: "user", ExposedToAssist: true}, false, "disabled in Home Assistant (by user)"},
		{assist, Entity{EntityID: "sensor.x_temperature", HiddenBy: "integration"}, false, "hidden in Home Assistant (by integration)"},
		{plain, Entity{EntityID: "light.kitchen"}, true, "included by light.kitchen"},
		{plain, Entity{EntityID: "light.other", ExposedToAssist: true}, false, "not included (Assist exposure is not followed)"},
	} {
		d := c.rule.Decide(c.e)
		if d.Exposed != c.want || d.Reason != c.why {
			t.Errorf("%s: %+v, want %v %q", c.e.EntityID, d, c.want, c.why)
		}
	}
}

func TestValidateAndSplit(t *testing.T) {
	if err := (Rule{Include: []string{"light.["}}).Validate(); err == nil {
		t.Fatal("a malformed glob passed")
	}
	if err := (Rule{Include: []string{"light.*"}, Exclude: []string{"a.b"}}).Validate(); err != nil {
		t.Fatal(err)
	}
	if got := Split(" light.a, sensor.*\nswitch.b ,,"); !slices.Equal(got, []string{"light.a", "sensor.*", "switch.b"}) {
		t.Fatalf("Split %q", got)
	}
	if got := Split(""); len(got) != 0 {
		t.Fatalf("Split empty %q", got)
	}
}
