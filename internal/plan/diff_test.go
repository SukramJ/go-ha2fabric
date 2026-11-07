// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package plan_test

import (
	"fmt"
	"maps"
	"slices"
	"testing"

	"github.com/SukramJ/go-fabric/device"

	"github.com/SukramJ/go-ha2fabric/internal/plan"
)

// built is a group as the reconciler serves it; each key's endpoint is
// "<key>@<sig>", so an op shows which build it carries.
func built(key, sig string, keys ...string) *plan.Built[string] {
	b := &plan.Built[string]{Key: key, Sig: sig}
	for _, k := range keys {
		b.Keys = append(b.Keys, device.StringKey(k))
		b.Endpoints = append(b.Endpoints, k+"@"+sig)
	}
	return b
}

func served(groups ...*plan.Built[string]) map[string]*plan.Built[string] {
	out := map[string]*plan.Built[string]{}
	for _, g := range groups {
		out[g.Key] = g
	}
	return out
}

// ops is the batch as text, one op each.
func ops(ch plan.Changes[string]) []string {
	var out []string
	for _, op := range ch.Ops {
		switch op.Kind {
		case plan.Remove:
			out = append(out, "remove "+op.Key.String())
		case plan.Forget:
			out = append(out, "forget "+op.Key.String()+" "+op.Entity)
		case plan.Replace:
			out = append(out, "replace "+op.Key.String()+" "+op.Endpoint)
		case plan.Add:
			out = append(out, "add "+op.Key.String()+" "+op.Endpoint)
		default:
			out = append(out, fmt.Sprintf("kind %d", op.Kind))
		}
	}
	return out
}

func TestDiff(t *testing.T) {
	kitchen := built("dev-kitchen", "s1", "dev-kitchen/light", "dev-kitchen/sensor")
	hall := built("dev-hall", "s1", "dev-hall/lock")
	for _, c := range []struct {
		name                     string
		old                      map[string]*plan.Built[string]
		next                     []*plan.Built[string]
		forget                   map[string]device.SourceKey
		ops                      []string
		added, replaced, removed []string
		forgotten                map[string]string
	}{
		{
			name:  "added",
			next:  []*plan.Built[string]{kitchen},
			ops:   []string{"add dev-kitchen/light dev-kitchen/light@s1", "add dev-kitchen/sensor dev-kitchen/sensor@s1"},
			added: []string{"dev-kitchen"},
		},
		{
			name: "kept",
			old:  served(kitchen),
			next: []*plan.Built[string]{kitchen},
		},
		{
			name:     "replaced: a shared, a new and a vanished key",
			old:      served(kitchen),
			next:     []*plan.Built[string]{built("dev-kitchen", "s2", "dev-kitchen/light", "dev-kitchen/switch")},
			ops:      []string{"replace dev-kitchen/light dev-kitchen/light@s2", "add dev-kitchen/switch dev-kitchen/switch@s2", "remove dev-kitchen/sensor"},
			replaced: []string{"dev-kitchen"},
		},
		{
			name:    "removed",
			old:     served(kitchen),
			ops:     []string{"remove dev-kitchen/light", "remove dev-kitchen/sensor"},
			removed: []string{"dev-kitchen"},
		},
		{
			name:      "a forget whose key a removed group holds",
			old:       served(kitchen),
			forget:    map[string]device.SourceKey{"sensor.kitchen": device.StringKey("dev-kitchen/sensor")},
			ops:       []string{"remove dev-kitchen/light", "forget dev-kitchen/sensor sensor.kitchen"},
			removed:   []string{"dev-kitchen"},
			forgotten: map[string]string{"dev-kitchen/sensor": "sensor.kitchen"},
		},
		{
			name:      "a forget of a key never served",
			old:       served(kitchen),
			next:      []*plan.Built[string]{kitchen},
			forget:    map[string]device.SourceKey{"light.cellar": device.StringKey("dev-cellar/light")},
			ops:       []string{"forget dev-cellar/light light.cellar"},
			forgotten: map[string]string{"dev-cellar/light": "light.cellar"},
		},
		{
			name:   "a forget of a live key",
			old:    served(kitchen),
			next:   []*plan.Built[string]{kitchen},
			forget: map[string]device.SourceKey{"light.kitchen": device.StringKey("dev-kitchen/light")},
		},
		{
			name:     "a failed build replacing a served group",
			old:      served(kitchen),
			next:     []*plan.Built[string]{built("dev-kitchen", "s2")},
			ops:      []string{"remove dev-kitchen/light", "remove dev-kitchen/sensor"},
			replaced: []string{"dev-kitchen"},
		},
		{
			name:    "two removed groups, in key order",
			old:     served(kitchen, hall),
			ops:     []string{"remove dev-hall/lock", "remove dev-kitchen/light", "remove dev-kitchen/sensor"},
			removed: []string{"dev-hall", "dev-kitchen"},
		},
		{
			name: "the sections: removed, replaced, trailing forgets, added",
			old:  served(kitchen, hall, built("dev-attic", "s1", "dev-attic/light")),
			next: []*plan.Built[string]{
				built("dev-porch", "s1", "dev-porch/light"),
				built("dev-kitchen", "s2", "dev-kitchen/light"),
				hall,
			},
			forget: map[string]device.SourceKey{
				"sensor.kitchen": device.StringKey("dev-kitchen/sensor"),
				"switch.b":       device.StringKey("dev-b/switch"),
				"switch.a":       device.StringKey("dev-a/switch"),
			},
			ops: []string{
				"remove dev-attic/light",
				"replace dev-kitchen/light dev-kitchen/light@s2", "forget dev-kitchen/sensor sensor.kitchen",
				"forget dev-a/switch switch.a", "forget dev-b/switch switch.b",
				"add dev-porch/light dev-porch/light@s1",
			},
			added: []string{"dev-porch"}, replaced: []string{"dev-kitchen"}, removed: []string{"dev-attic"},
			forgotten: map[string]string{"dev-kitchen/sensor": "sensor.kitchen", "dev-a/switch": "switch.a", "dev-b/switch": "switch.b"},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			ch := plan.Diff(c.old, c.next, c.forget)
			if got := ops(ch); !slices.Equal(got, c.ops) {
				t.Errorf("ops\n got %q\nwant %q", got, c.ops)
			}
			if !slices.Equal(ch.Added, c.added) || !slices.Equal(ch.Replaced, c.replaced) || !slices.Equal(ch.Removed, c.removed) {
				t.Errorf("added %q replaced %q removed %q", ch.Added, ch.Replaced, ch.Removed)
			}
			if !maps.Equal(ch.Forgotten, c.forgotten) {
				t.Errorf("forgotten %v, want %v", ch.Forgotten, c.forgotten)
			}
		})
	}
}
