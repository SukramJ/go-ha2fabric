// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package bridge

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	fabricui "github.com/SukramJ/go-fabric-ui/fabricui"
	"github.com/SukramJ/go-fabric/eligibility"

	"github.com/SukramJ/go-ha2fabric/internal/expose"
	"github.com/SukramJ/go-ha2fabric/internal/ha"
	"github.com/SukramJ/go-ha2fabric/internal/selection"
)

// Exposure is the web UI's selection over the bridge
// (go-fabric-ui docs/provider.md at v0.1.0): every candidate entity of
// the last sync with go-hamapping's verdict, whether the rule exposes it
// and which layer decided; a change is an override in the bridge's
// [selection.Store]. The handler calls [Bridge.Resync] (Options.Reassemble)
// after every apply, so the endpoints follow before the response.
func (b *Bridge) Exposure() fabricui.ExposureProvider { return provider{b} }

type provider struct{ b *Bridge }

// The item fields: the schema's columns and the fixed filters.
const (
	fieldEntityID = "entity_id"
	fieldName     = "name"
	fieldDomain   = "domain"
	fieldDevice   = "device"
	fieldArea     = "area"
	fieldLabels   = "labels"
	fieldAssist   = "assist"
	fieldPlatform = "platform"
	fieldVerdict  = "verdict"
	fieldSource   = "source"
)

var schema = fabricui.Schema{
	Columns: []fabricui.Column{
		{Key: fieldEntityID, Label: "Entity"},
		{Key: fieldName, Label: "Name"},
		{Key: fieldDomain, Label: "Domain", Filter: true},
		{Key: fieldDevice, Label: "Device"},
		{Key: fieldArea, Label: "Area", Filter: true},
		// Multi: an entity's labels are a list (Item.Lists); the UI filters
		// by one label, not by the combination (H3 Q2, go-fabric-ui v0.1.1,
		// list fields v0.1.3).
		{Key: fieldLabels, Label: "Labels", Filter: true, Multi: true},
		{Key: fieldAssist, Label: "Exposed to Assist"},
		{Key: fieldPlatform, Label: "Integration"},
	},
	Groups: true,
	Filters: []fabricui.Filter{
		{Field: fieldVerdict, Label: "Verdict", Options: []fabricui.FilterOption{
			{Value: eligibility.StateMappable.String(), Label: "Mappable"},
			{Value: eligibility.StatePartial.String(), Label: "Partially mappable"},
			{Value: eligibility.StateUnmappable.String(), Label: "Unmappable"},
		}},
		{Field: fieldSource, Label: "Decided by", Options: []fabricui.FilterOption{
			{Value: expose.SourceAssist, Label: "Assist"},
			{Value: expose.SourceInclude, Label: "Include list"},
			{Value: expose.SourceLabel, Label: "Label"},
			{Value: expose.SourceExclude, Label: "Exclude list"},
			{Value: expose.SourceOverrideOn, Label: "Turned on here"},
			{Value: expose.SourceOverrideOff, Label: "Turned off here"},
			{Value: expose.SourceDisabled, Label: "Disabled in Home Assistant"},
			{Value: expose.SourceHidden, Label: "Hidden in Home Assistant"},
		}},
	},
}

func (provider) Schema(context.Context) (fabricui.Schema, error) { return schema, nil }

// view is the last sync as the provider reads it.
type view struct {
	snap     *Snapshot
	verdicts []Verdict
	all      map[string]*Verdict
	reg      map[string]*ha.EntityEntry
	devs     map[string]*ha.DeviceEntry
	areas    map[string]string
	names    map[string]string
}

func (p provider) view() *view {
	p.b.mu.Lock()
	v := &view{snap: p.b.last, verdicts: slices.Clone(p.b.verdicts), all: p.b.all}
	p.b.mu.Unlock()
	v.reg, v.devs, v.areas = map[string]*ha.EntityEntry{}, map[string]*ha.DeviceEntry{}, map[string]string{}
	if v.snap != nil {
		for i := range v.snap.Entities {
			v.reg[v.snap.Entities[i].EntityID] = &v.snap.Entities[i]
		}
		for i := range v.snap.Devices {
			v.devs[v.snap.Devices[i].ID] = &v.snap.Devices[i]
		}
		for _, a := range v.snap.Areas {
			v.areas[a.AreaID] = a.Name
		}
	}
	v.names = p.b.names()
	return v
}

// item is one candidate as the page shows it.
func (v *view) item(c *Verdict) fabricui.Item {
	id := c.EntityID
	it := fabricui.Item{Key: id, Name: c.Name, Group: c.Device, Enabled: c.Exposed, Source: c.Source}
	if n := v.names[id]; n != "" {
		it.Name = n
	}
	if it.Name == "" {
		it.Name = id
	}
	switch {
	case c.Exposed:
		it.Verdict = c.verdict
	case v.all[id] != nil && v.all[id].Exposed:
		it.Verdict = v.all[id].verdict
	default:
		// Disabled, hidden or without a state: nothing go-hamapping could
		// map now.
		var reason string
		if v.all[id] != nil {
			reason = v.all[id].ExposeReason
		}
		if reason == "" {
			reason = c.ExposeReason
		}
		it.Verdict = eligibility.Verdict{State: eligibility.StateUnmappable, Reason: reason}
	}
	var platform, area string
	if r := v.reg[id]; r != nil {
		platform, area = r.Platform, v.areas[r.AreaID]
		if d := v.devs[r.DeviceID]; d != nil && area == "" {
			area = v.areas[d.AreaID]
		}
	}
	assist := "no"
	if v.snap != nil && v.snap.Exposed.Entities[id][ha.AssistantConversation] {
		assist = "yes"
	}
	it.Fields = map[string]string{
		fieldEntityID: id, fieldName: it.Name, fieldDomain: ha.Domain(id), fieldDevice: c.Device,
		fieldArea: area, fieldLabels: strings.Join(c.Labels, ", "), fieldAssist: assist,
		fieldPlatform: platform, fieldVerdict: it.Verdict.State.String(), fieldSource: c.Source,
	}
	// The labels column is Multi: its values are a list, so a label name
	// with a comma stays one value (go-fabric-ui v0.1.3 Item.Lists; the
	// owner's decision on F1 Q2). Fields keeps the display string.
	it.Lists = map[string][]string{fieldLabels: append([]string{}, c.Labels...)}
	return it
}

func (p provider) List(context.Context) ([]fabricui.Item, error) {
	v := p.view()
	out := make([]fabricui.Item, 0, len(v.verdicts))
	for i := range v.verdicts {
		out = append(out, v.item(&v.verdicts[i]))
	}
	return out, nil
}

// check refuses an update the provider cannot apply: an unknown entity,
// or enabling one go-hamapping cannot map (disabled, hidden or without a
// state included).
func (v *view) check(updates []fabricui.Update) (map[string]fabricui.Item, error) {
	if v.snap == nil {
		return nil, errors.New("bridge: Home Assistant is not read yet")
	}
	items := map[string]fabricui.Item{}
	for i := range v.verdicts {
		items[v.verdicts[i].EntityID] = v.item(&v.verdicts[i])
	}
	for _, u := range updates {
		it, ok := items[u.Key]
		if !ok {
			return nil, fmt.Errorf("%w: %q", fabricui.ErrUnknownItem, u.Key)
		}
		if u.Enabled && it.Verdict.State == eligibility.StateUnmappable {
			return nil, fmt.Errorf("%w: %q cannot be exposed: %s", fabricui.ErrInvalidUpdate, u.Key, it.Verdict.Reason)
		}
	}
	return items, nil
}

// proposed is the rule and the names with the updates applied.
func (p provider) proposed(v *view, updates []fabricui.Update) (rule expose.Rule, names map[string]string) {
	rule = p.b.rule()
	rule.Overrides = maps.Clone(rule.Overrides)
	if rule.Overrides == nil {
		rule.Overrides = map[string]bool{}
	}
	names = maps.Clone(v.names)
	if names == nil {
		names = map[string]string{}
	}
	for _, u := range updates {
		rule.Overrides[u.Key] = u.Enabled
		if u.Name != nil {
			names[u.Key] = strings.TrimSpace(*u.Name)
		}
	}
	return rule, names
}

// Preview is the plan with the updates as overrides, through
// go-hamapping's Map without Build: the updated items and every item
// whose exposure or verdict the change moves (a battery that rides on
// another entity's endpoint).
func (p provider) Preview(_ context.Context, updates []fabricui.Update) ([]fabricui.Item, error) {
	v := p.view()
	before, err := v.check(updates)
	if err != nil {
		return nil, err
	}
	rule, names := p.proposed(v, updates)
	after := &view{snap: v.snap, verdicts: Dry(v.snap, rule, names, p.b.opts.Camera), all: v.all, reg: v.reg, devs: v.devs, areas: v.areas, names: names}
	touched := map[string]bool{}
	for _, u := range updates {
		touched[u.Key] = true
	}
	var out []fabricui.Item
	for i := range after.verdicts {
		c := &after.verdicts[i]
		it := after.item(c)
		old := before[c.EntityID]
		if touched[c.EntityID] || old.Enabled != it.Enabled || old.Verdict.State != it.Verdict.State || old.Verdict.DeviceType != it.Verdict.DeviceType {
			out = append(out, it)
		}
	}
	return out, nil
}

// Apply stores the updates as overrides. An update that asks for what
// the rule decides without an override, and names nothing, removes the
// entity's override: it follows the rule again. The endpoints follow on
// the handler's Reassemble ([Bridge.Resync]).
func (p provider) Apply(_ context.Context, updates []fabricui.Update) (int, error) {
	if p.b.opts.Selection == nil {
		return 0, errors.New("bridge: no selection store")
	}
	v := p.view()
	if _, err := v.check(updates); err != nil {
		return 0, err
	}
	base := p.b.rule()
	base.Overrides = maps.Clone(base.Overrides)
	for _, u := range updates {
		delete(base.Overrides, u.Key)
	}
	plain := map[string]bool{}
	dry := Dry(v.snap, base, nil, p.b.opts.Camera)
	for i := range dry {
		plain[dry[i].EntityID] = dry[i].Exposed
	}
	current := p.b.opts.Selection.Entries()
	set := map[string]selection.Entry{}
	var remove []string
	for _, u := range updates {
		name := current[u.Key].Name
		if u.Name != nil {
			name = strings.TrimSpace(*u.Name)
		}
		if u.Enabled == plain[u.Key] && name == "" {
			remove = append(remove, u.Key)
			delete(set, u.Key)
			continue
		}
		set[u.Key] = selection.Entry{Enabled: u.Enabled, Name: name}
		remove = slices.DeleteFunc(remove, func(id string) bool { return id == u.Key })
	}
	if err := p.b.opts.Selection.Set(set, remove); err != nil {
		return 0, err
	}
	p.b.log.Info("bridge.selection_applied", "set", len(set), "removed", len(remove))
	return len(updates), nil
}
