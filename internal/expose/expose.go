// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

// Package expose is the daemon's exposure rule (CONCEPT.md, Open
// decisions, decided in brief H2 §1.2): which Home Assistant entities are
// candidates for the bridge.
//
// An entity is exposed when it is exposed to Assist ("conversation") in
// homeassistant/expose_entity/list, or named in the include list (entity
// ids or globs), and it is not named in the exclude list; a disabled or
// hidden registry entry never is. The daemon never writes Home
// Assistant's exposure settings.
package expose

import (
	"fmt"
	"path"
	"strings"
)

// Rule is the configured rule.
type Rule struct {
	// Assist follows Home Assistant's "expose to Assist" setting.
	Assist bool
	// Include and Exclude are entity ids or globs (path.Match syntax,
	// "light.*", "sensor.*_temperature").
	Include, Exclude []string
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
}

// Decision is the rule's verdict with a reason an operator reads.
type Decision struct {
	Exposed bool
	Reason  string
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

// Decide applies the rule to one entity.
func (r Rule) Decide(e Entity) Decision {
	switch {
	case e.DisabledBy != "":
		return Decision{Reason: "disabled in Home Assistant (by " + e.DisabledBy + ")"}
	case e.HiddenBy != "":
		return Decision{Reason: "hidden in Home Assistant (by " + e.HiddenBy + ")"}
	}
	if p, ok := match(r.Exclude, e.EntityID); ok {
		return Decision{Reason: "excluded by " + p}
	}
	if p, ok := match(r.Include, e.EntityID); ok {
		return Decision{Exposed: true, Reason: "included by " + p}
	}
	if r.Assist && e.ExposedToAssist {
		return Decision{Exposed: true, Reason: "exposed to Assist"}
	}
	if r.Assist {
		return Decision{Reason: "not exposed to Assist and not included"}
	}
	return Decision{Reason: "not included (Assist exposure is not followed)"}
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
