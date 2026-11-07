// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package plan

import (
	"fmt"
	"maps"

	"github.com/SukramJ/go-hamqtt/model"

	"github.com/SukramJ/go-ha2fabric/internal/expose"
)

// Input is what every assessment takes besides the snapshot: the
// effective exposure rule, the web UI's names, the camera option and
// the camera ports.
type Input struct {
	// Rule is the configured rule, its Overrides the web UI's.
	Rule expose.Rule
	// Names are the web UI's name overrides by entity id; nil is none.
	Names map[string]string
	// Cameras is the camera option (config.Config.Camera). Off, Assess
	// gives an exposed camera the verdict CameraOff and plans it not.
	Cameras bool
	// Ports is go-hamapping's Options.Ports (go-hamapping plan.go:120-128
	// at v0.4.3): a camera entity's host object, asked once per entity
	// during Map. The composition root sets camera.DryPorts for
	// --dump-plan and the web UI's previews; the bridge maps with its
	// live hosts. MapOptions hands it through unchanged.
	Ports func(model.Entity) any
}

// Overrides are the web UI's overrides as the rule reads them;
// *selection.Store implements it.
type Overrides interface {
	// Enabled is each overridden entity's exposure.
	Enabled() map[string]bool
	// Names is each named entity's name.
	Names() map[string]string
}

// Effective is the configured rule with the overrides o: their
// exposures become the rule's Overrides and their names the input's.
// A nil o is none, and the rule's own Overrides stay; pass a literal
// nil, not a nil *selection.Store, which an interface does not hold as
// nil.
func Effective(rule expose.Rule, o Overrides, cameras bool, ports func(model.Entity) any) Input {
	in := Input{Rule: rule, Cameras: cameras, Ports: ports}
	if o != nil {
		in.Rule.Overrides = o.Enabled()
		in.Names = o.Names()
	}
	return in
}

// With is in with the proposed overrides on top: enabled into the rule's
// Overrides, names into the names. Neither of in's maps is written.
func (in Input) With(enabled map[string]bool, names map[string]string) Input {
	out := in.clone()
	maps.Copy(out.Rule.Overrides, enabled)
	maps.Copy(out.Names, names)
	return out
}

// Without is in with the overrides of keys dropped: what the rule
// decides for them by itself. Neither of in's maps is written.
func (in Input) Without(keys ...string) Input {
	out := in.clone()
	for _, k := range keys {
		delete(out.Rule.Overrides, k)
	}
	return out
}

// clone is in with maps of its own.
func (in Input) clone() Input {
	out := in
	out.Rule.Overrides = maps.Clone(in.Rule.Overrides)
	if out.Rule.Overrides == nil {
		out.Rule.Overrides = map[string]bool{}
	}
	out.Names = maps.Clone(in.Names)
	if out.Names == nil {
		out.Names = map[string]string{}
	}
	return out
}

// Warnings are the input's findings against the label registry: each
// label value that names no Home Assistant label, which is used as a
// label_id and matches once such a label exists. nil is none.
func Warnings(in Input, labels expose.Labels) []string {
	unresolved := in.Rule.Unresolved(labels)
	if len(unresolved) == 0 {
		return nil
	}
	out := make([]string, 0, len(unresolved))
	for _, v := range unresolved {
		out = append(out, fmt.Sprintf("label %q names no Home Assistant label; it matches a label with that id once one exists", v))
	}
	return out
}
