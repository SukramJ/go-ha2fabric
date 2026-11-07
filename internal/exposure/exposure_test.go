// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package exposure

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	fabricui "github.com/SukramJ/go-fabric-ui/fabricui"

	"github.com/SukramJ/go-ha2fabric/internal/bridge"
	"github.com/SukramJ/go-ha2fabric/internal/expose"
	"github.com/SukramJ/go-ha2fabric/internal/ha"
	"github.com/SukramJ/go-ha2fabric/internal/ha/hatest"
	"github.com/SukramJ/go-ha2fabric/internal/plan"
	"github.com/SukramJ/go-ha2fabric/internal/selection"
)

// model is a read model as a sync of the recorded fixtures leaves it.
type model struct{ v bridge.View }

func (m model) View() bridge.View { return m.v }

// synced is the fixtures through plan.Dry and plan.Candidates under in,
// with one label finding first seen at since.
func synced(t *testing.T, in plan.Input, since time.Time) model {
	t.Helper()
	var s ha.Snapshot
	for name, v := range map[string]any{
		"entity_registry_list.json": &s.Entities, "device_registry_list.json": &s.Devices, "area_registry_list.json": &s.Areas, "label_registry_list.json": &s.Labels,
		"get_states.json": &s.States, "get_config.json": &s.Config, "expose_entity_list.json": &s.Exposed,
	} {
		if err := json.Unmarshal(hatest.Fixture(t, name), v); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	return model{bridge.View{
		Snapshot: &s, Verdicts: plan.Dry(&s, in), All: plan.Candidates(&s, in),
		Warnings: []bridge.Warning{{Message: "label \"Later\" names no Home Assistant label", Since: since}},
	}}
}

func TestProvider(t *testing.T) {
	ctx := context.Background()
	in := func() plan.Input { return plan.Input{Rule: expose.Rule{Assist: true}} }
	since := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	m := synced(t, in(), since)
	p := New(m, Options{Input: in})

	items, err := p.List(ctx)
	if err != nil || len(items) != len(m.v.Verdicts) {
		t.Fatalf("list: %d items, %d verdicts, %v", len(items), len(m.v.Verdicts), err)
	}
	for _, it := range items {
		if it.Fields[fieldVerdict] == "" || it.Fields[fieldVerdict] != it.Verdict.State.String() {
			t.Errorf("%s: verdict field %q", it.Key, it.Fields[fieldVerdict])
		}
	}
	if _, err := p.Preview(ctx, []fabricui.Update{{Key: "light.nowhere", Enabled: true}}); !errors.Is(err, fabricui.ErrUnknownItem) {
		t.Fatalf("preview of an unknown key: %v", err)
	}
	if _, err := p.Apply(ctx, []fabricui.Update{{Key: "light.kitchen"}}); err == nil {
		t.Fatal("apply without a store")
	}
	ws := p.Warnings(ctx)
	if len(ws) != 1 || ws[0].Code != WarningLabelUnresolved || !ws[0].Since.Equal(since) {
		t.Fatalf("warnings %+v", ws)
	}
}

// TestProviderApply: an override that repeats the rule's word is no
// override; one that differs is stored.
func TestProviderApply(t *testing.T) {
	ctx := context.Background()
	sel, err := selection.Open("", nil)
	if err != nil {
		t.Fatal(err)
	}
	in := func() plan.Input { return plan.Effective(expose.Rule{Assist: true}, sel, false, nil) }
	p := New(synced(t, in(), time.Now()), Options{Input: in, Selection: sel})
	n, err := p.Apply(ctx, []fabricui.Update{{Key: "light.kitchen", Enabled: true}, {Key: "switch.coffee", Enabled: false}})
	if err != nil || n != 2 {
		t.Fatalf("apply %d %v", n, err)
	}
	if e := sel.Enabled(); len(e) != 1 || e["switch.coffee"] {
		t.Fatalf("overrides %v", e)
	}
	if ws := p.Warnings(ctx); len(ws) != 1 {
		t.Fatalf("warnings without a recovered file %+v", ws)
	}
}

// TestProviderBeforeSync: before the first sync there is nothing to
// list, and nothing to change.
func TestProviderBeforeSync(t *testing.T) {
	p := New(model{}, Options{Input: func() plan.Input { return plan.Input{} }})
	if items, err := p.List(context.Background()); err != nil || len(items) != 0 {
		t.Fatalf("list %v %v", items, err)
	}
	if _, err := p.Preview(context.Background(), []fabricui.Update{{Key: "light.kitchen"}}); err == nil {
		t.Fatal("preview before the first sync")
	}
	if ws := p.Warnings(context.Background()); ws != nil {
		t.Fatalf("warnings %+v", ws)
	}
}
