// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

// Package expose is the daemon's exposure rule (CONCEPT.md, Open
// decisions, decided in brief H2 §1.2, extended in brief H3 §2.2 and
// §2.8): which Home Assistant entities are candidates for the bridge.
//
// An entity is exposed when it is exposed to Assist ("conversation") in
// homeassistant/expose_entity/list, or named in the include list (entity
// ids or globs) or carries an include label, and it is not named in the
// exclude list and carries no exclude label; a disabled or hidden
// registry entry never is. An override the web UI stored sits above the
// lists and below disabled and hidden. The daemon never writes Home
// Assistant's exposure settings.
//
// A label value matches a Home Assistant label by its name,
// case-insensitively, else as its label_id (owner decision D1 of
// 2026-10-25); an entity's labels are its own and its device's.
package expose

import (
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"
)

// Rule is the configured rule.
type Rule struct {
	// Assist follows Home Assistant's "expose to Assist" setting.
	Assist bool
	// Include and Exclude are entity ids or globs (path.Match syntax,
	// "light.*", "sensor.*_temperature").
	Include, Exclude []string
	// IncludeLabels and ExcludeLabels are Home Assistant labels, by name
	// or label_id ([Labels.Resolve]).
	IncludeLabels, ExcludeLabels []string
	// Overrides are the web UI's per-entity choices, by entity id: true
	// exposes, false hides, absent leaves the entity to the lists.
	Overrides map[string]bool
}

// Entity is what the rule reads about one entity.
type Entity struct {
	EntityID string
	// Registered is whether the entity has a registry entry.
	Registered bool
	// DisabledBy and HiddenBy are the registry entry's
	// (homeassistant/helpers/entity_registry.py:137-151 at 2026.10.3).
	DisabledBy, HiddenBy string
	// ExposedToAssist is the entity's Assist exposure from
	// homeassistant/expose_entity/list.
	ExposedToAssist bool
	// Labels are the label ids of the entity's registry entry and of its
	// device's (helpers/entity_registry.py:264, helpers/device_registry.py:412).
	Labels []string
}

// Sources say which layer of the rule decided ([Decision].Source).
const (
	SourceDisabled    = "disabled"
	SourceHidden      = "hidden"
	SourceOverrideOn  = "override:on"
	SourceOverrideOff = "override:off"
	SourceExclude     = "rule:exclude"
	SourceInclude     = "rule:include"
	SourceLabel       = "rule:label"
	SourceAssist      = "rule:assist"
)

// Decision is the rule's verdict with a reason an operator reads and the
// layer that decided.
type Decision struct {
	Exposed bool
	Reason  string
	Source  string
}

// Labels is the label registry: label_id to name
// (config/label_registry/list).
type Labels map[string]string

// Resolve is the label_id a configured value names: the label whose name
// equals it case-insensitively, else the label whose id it is. known is
// false when neither exists; id is then the value as given, so a label
// created later matches by that id (D1; matter-hub slugifies an unknown
// name instead, docs/by_design.md).
func (l Labels) Resolve(value string) (id string, known bool) {
	value = strings.TrimSpace(value)
	for _, k := range slices.Sorted(maps.Keys(l)) {
		if strings.EqualFold(l[k], value) {
			return k, true
		}
	}
	_, known = l[value]
	return value, known
}

// name is a label's display name: the registry's, else the id.
func (l Labels) name(id string) string {
	if n := l[id]; n != "" {
		return n
	}
	return id
}

// Validate refuses a malformed pattern.
func (r Rule) Validate() error {
	for _, p := range append(append([]string(nil), r.Include...), r.Exclude...) {
		if _, err := path.Match(p, ""); err != nil {
			return fmt.Errorf("pattern %q: %w", p, err)
		}
	}
	return nil
}

// Unresolved are the rule's label values that name no label of l: a
// warning for the operator, not an error (a label may be created later).
func (r Rule) Unresolved(l Labels) []string {
	var out []string
	for _, v := range append(append([]string(nil), r.IncludeLabels...), r.ExcludeLabels...) {
		if strings.TrimSpace(v) == "" {
			continue
		}
		if _, ok := l.Resolve(v); !ok {
			out = append(out, v)
		}
	}
	return out
}

// Decide applies the rule to one entity: disabled, hidden, the override,
// exclude (ids, globs, labels), include (ids, globs, labels), Assist.
func (r Rule) Decide(e Entity, l Labels) Decision {
	switch {
	case e.DisabledBy != "":
		return Decision{Reason: "disabled in Home Assistant (by " + e.DisabledBy + ")", Source: SourceDisabled}
	case e.HiddenBy != "":
		return Decision{Reason: "hidden in Home Assistant (by " + e.HiddenBy + ")", Source: SourceHidden}
	}
	if on, ok := r.Overrides[e.EntityID]; ok {
		if on {
			return Decision{Exposed: true, Reason: "turned on in the web UI", Source: SourceOverrideOn}
		}
		return Decision{Reason: "turned off in the web UI", Source: SourceOverrideOff}
	}
	if p, ok := match(r.Exclude, e.EntityID); ok {
		return Decision{Reason: "excluded by " + p, Source: SourceExclude}
	}
	if id, ok := label(r.ExcludeLabels, e.Labels, l); ok {
		return Decision{Reason: "excluded by label '" + l.name(id) + "'", Source: SourceExclude}
	}
	if p, ok := match(r.Include, e.EntityID); ok {
		return Decision{Exposed: true, Reason: "included by " + p, Source: SourceInclude}
	}
	if id, ok := label(r.IncludeLabels, e.Labels, l); ok {
		return Decision{Exposed: true, Reason: "included by label '" + l.name(id) + "'", Source: SourceLabel}
	}
	if r.Assist && e.ExposedToAssist {
		return Decision{Exposed: true, Reason: "exposed to Assist", Source: SourceAssist}
	}
	if r.Assist {
		return Decision{Reason: "not exposed to Assist and not included", Source: SourceAssist}
	}
	return Decision{Reason: "not included (Assist exposure is not followed)", Source: SourceInclude}
}

// label is the first configured value whose label the entity carries.
func label(values, carried []string, l Labels) (string, bool) {
	for _, v := range values {
		if strings.TrimSpace(v) == "" {
			continue
		}
		if id, _ := l.Resolve(v); slices.Contains(carried, id) {
			return id, true
		}
	}
	return "", false
}

func match(patterns []string, id string) (string, bool) {
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if p == id {
			return p, true
		}
		if ok, _ := path.Match(p, id); ok {
			return p, true
		}
	}
	return "", false
}

// Split reads a comma- or whitespace-separated list.
func Split(s string) []string {
	f := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == '\n' || r == '\t' })
	out := f[:0]
	for _, x := range f {
		if x != "" {
			out = append(out, x)
		}
	}
	return out
}
