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
		d := c.rule.Decide(c.e, nil)
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

// labels is a label registry: two labels, one whose name differs from
// its id only in case and one whose id looks like another's name.
var labels = Labels{"matter": "Matter", "not_on_bridge": "Not on bridge", "kitchen": "Bridge"}

func TestDecideLabels(t *testing.T) {
	inc := Rule{IncludeLabels: []string{"MATTER"}}
	exc := Rule{Assist: true, Include: []string{"light.*"}, ExcludeLabels: []string{"not_on_bridge"}}
	byID := Rule{IncludeLabels: []string{"kitchen"}}
	byName := Rule{IncludeLabels: []string{"bridge"}}
	unknown := Rule{IncludeLabels: []string{"later"}}
	over := Rule{Assist: true, Exclude: []string{"light.a"}, Overrides: map[string]bool{"light.a": true, "light.b": false, "light.c": true}}
	for _, c := range []struct {
		name string
		rule Rule
		e    Entity
		want bool
		why  string
		src  string
	}{
		{"name, case-insensitive", inc, Entity{EntityID: "sensor.t", Labels: []string{"matter"}}, true, "included by label 'Matter'", SourceLabel},
		{"no label", inc, Entity{EntityID: "sensor.t"}, false, "not included (Assist exposure is not followed)", SourceInclude},
		{"exclude label beats include glob", exc, Entity{EntityID: "light.x", Labels: []string{"not_on_bridge"}}, false, "excluded by label 'Not on bridge'", SourceExclude},
		{"exclude label beats Assist", exc, Entity{EntityID: "switch.x", ExposedToAssist: true, Labels: []string{"not_on_bridge"}}, false, "excluded by label 'Not on bridge'", SourceExclude},
		{"include glob", exc, Entity{EntityID: "light.x"}, true, "included by light.*", SourceInclude},
		{"Assist", exc, Entity{EntityID: "switch.x", ExposedToAssist: true}, true, "exposed to Assist", SourceAssist},
		{"label id when no name matches", byID, Entity{EntityID: "light.k", Labels: []string{"kitchen"}}, true, "included by label 'Bridge'", SourceLabel},
		{"name before id", byName, Entity{EntityID: "light.k", Labels: []string{"kitchen"}}, true, "included by label 'Bridge'", SourceLabel},
		{"unresolved value used as an id", unknown, Entity{EntityID: "light.k", Labels: []string{"later"}}, true, "included by label 'later'", SourceLabel},
		{"override on beats exclude", over, Entity{EntityID: "light.a"}, true, "turned on in the web UI", SourceOverrideOn},
		{"override off beats Assist", over, Entity{EntityID: "light.b", ExposedToAssist: true}, false, "turned off in the web UI", SourceOverrideOff},
		{"disabled beats override", over, Entity{EntityID: "light.c", DisabledBy: "user"}, false, "disabled in Home Assistant (by user)", SourceDisabled},
		{"hidden beats override", over, Entity{EntityID: "light.c", HiddenBy: "user"}, false, "hidden in Home Assistant (by user)", SourceHidden},
	} {
		d := c.rule.Decide(c.e, labels)
		if d.Exposed != c.want || d.Reason != c.why || d.Source != c.src {
			t.Errorf("%s: %+v, want %v %q %s", c.name, d, c.want, c.why, c.src)
		}
	}
}

func TestResolveAndUnresolved(t *testing.T) {
	for v, want := range map[string][2]any{
		"Matter": {"matter", true}, "\tmatter\t": {"matter", true}, "not_on_bridge": {"not_on_bridge", true},
		"BRIDGE": {"kitchen", true}, "Later": {"Later", false},
	} {
		id, ok := labels.Resolve(v)
		if id != want[0] || ok != want[1] {
			t.Errorf("Resolve(%q) = %q %v, want %v %v", v, id, ok, want[0], want[1])
		}
	}
	r := Rule{IncludeLabels: []string{"Matter", "", "later"}, ExcludeLabels: []string{"gone"}}
	if got := r.Unresolved(labels); !slices.Equal(got, []string{"later", "gone"}) {
		t.Errorf("Unresolved %q", got)
	}
	if got := r.Unresolved(nil); len(got) != 3 {
		t.Errorf("Unresolved without a registry %q", got)
	}
}
