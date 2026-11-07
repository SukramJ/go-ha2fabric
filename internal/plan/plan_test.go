// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package plan_test

import (
	"encoding/json"
	"strings"
	"testing"

	hamapping "github.com/SukramJ/go-hamapping"
	"github.com/SukramJ/go-hamqtt/model"

	"github.com/SukramJ/go-ha2fabric/internal/camera"
	"github.com/SukramJ/go-ha2fabric/internal/expose"
	"github.com/SukramJ/go-ha2fabric/internal/ha"
	"github.com/SukramJ/go-ha2fabric/internal/ha/hatest"
	"github.com/SukramJ/go-ha2fabric/internal/plan"
)

// snapshot is the recorded fixtures as one sync reads them.
func snapshot(t *testing.T) *ha.Snapshot {
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
	return &s
}

func byID(vs []plan.Verdict) map[string]plan.Verdict {
	out := map[string]plan.Verdict{}
	for i := range vs {
		out[vs[i].EntityID] = vs[i]
	}
	return out
}

// TestDryPlan is --dump-plan on the fixtures: every candidate with the
// rule's word and go-hamapping's.
func TestDryPlan(t *testing.T) {
	v := byID(plan.Dry(snapshot(t), plan.Input{Rule: expose.Rule{Assist: true, Include: []string{"sensor.outdoor"}, Exclude: []string{"fan.*"}}}))
	for id, want := range map[string]struct {
		exposed bool
		why     string
		row     string
		state   string
	}{
		"light.kitchen":             {true, "exposed to Assist", "light/extended_color", "mappable"},
		"sensor.living_temperature": {true, "exposed to Assist", "sensor/temperature", "mappable"},
		"sensor.living_battery":     {false, "not exposed to Assist and not included", "", ""},
		"switch.coffee":             {true, "exposed to Assist", "switch/plug", "mappable"},
		"binary_sensor.front_door":  {true, "exposed to Assist", "binary_sensor/contact", "mappable"},
		"cover.living_blind":        {true, "exposed to Assist", "cover/window_covering", "mappable"},
		"climate.hall":              {true, "exposed to Assist", "climate/thermostat", "partially_mappable"},
		"lock.front":                {true, "exposed to Assist", "lock/door_lock", "partially_mappable"},
		"fan.ceiling":               {false, "excluded by fan.*", "", ""},
		"light.cellar":              {false, "disabled in Home Assistant (by user)", "", ""},
		"light.attic":               {false, "hidden in Home Assistant (by user)", "", ""},
		"sensor.processor_use":      {false, "not exposed to Assist and not included", "", ""},
		"sensor.outdoor":            {true, "included by sensor.outdoor", "sensor/temperature", "mappable"},
		"sensor.legacy_yaml":        {false, "not exposed to Assist and not included", "", ""},
		// The rows of go-hamapping v0.3.0 (R1-R5).
		"sensor.office_co2":            {true, "exposed to Assist", "sensor/carbon_dioxide", "mappable"},
		"sensor.mains_frequency":       {true, "exposed to Assist", "sensor/frequency", "mappable"},
		"sensor.garden_flow":           {true, "exposed to Assist", "sensor/volume_flow_rate", "mappable"},
		"button.router_restart":        {true, "exposed to Assist", "button/plug", "mappable"},
		"event.hall_remote":            {true, "exposed to Assist", "event/generic_switch", "partially_mappable"},
		"select.hall_scene":            {true, "exposed to Assist", "select/mode_select", "mappable"},
		"climate.bedroom_ac":           {true, "exposed to Assist", "climate/air_conditioner", "partially_mappable"},
		"vacuum.robo":                  {true, "exposed to Assist", "vacuum/robotic_vacuum_cleaner", "partially_mappable"},
		"valve.garden":                 {true, "exposed to Assist", "valve/water_valve", "mappable"},
		"water_heater.boiler":          {true, "exposed to Assist", "water_heater/thermostat", "partially_mappable"},
		"media_player.kitchen_speaker": {true, "exposed to Assist", "media_player/speaker", "mappable"},
		"media_player.living_tv":       {true, "exposed to Assist", "media_player/video_player", "partially_mappable"},
	} {
		got, ok := v[id]
		if !ok {
			t.Errorf("%s is no candidate", id)
			continue
		}
		if got.Exposed != want.exposed || got.ExposeReason != want.why || got.Row != want.row || got.State != want.state {
			t.Errorf("%s: %+v, want %+v", id, got, want)
		}
	}
	if d := v["light.kitchen"].Device; d != "Kitchen ceiling" {
		t.Errorf("light.kitchen's device %q", d)
	}
	if d := v["sensor.outdoor"].Device; d != "Outdoor temperature" {
		t.Errorf("sensor.outdoor's own device %q", d)
	}
	// An entity with a registry entry and no state is no endpoint yet.
	s := snapshot(t)
	s.States = s.States[1:] // light.kitchen
	if got := byID(plan.Dry(s, plan.Input{Rule: expose.Rule{Assist: true}}))["light.kitchen"]; got.Exposed || !strings.HasSuffix(got.ExposeReason, "no state yet") {
		t.Errorf("stateless light %+v", got)
	}
}

// TestCameraOff: with the option off, an exposed camera is skipped
// before go-hamapping, and says why.
func TestCameraOff(t *testing.T) {
	s := snapshot(t)
	s.States = append(s.States, ha.State{EntityID: "camera.porch", State: "idle", Attributes: map[string]any{"supported_features": 2, "friendly_name": "Porch"}})
	got := byID(plan.Dry(s, plan.Input{Rule: expose.Rule{Include: []string{"camera.porch"}}}))["camera.porch"]
	if !got.Exposed || got.State != "unmappable" || !strings.Contains(got.Reason, "CAMERA") || got.Row != "" {
		t.Fatalf("camera off %+v", got)
	}
	if got := byID(plan.Dry(s, plan.Input{Rule: expose.Rule{Include: []string{"camera.porch"}}, Cameras: true, Ports: camera.DryPorts}))["camera.porch"]; strings.Contains(got.Reason, "CAMERA") {
		t.Fatalf("camera on %+v", got)
	}
}

// overrides is a plan.Overrides as the web UI's store holds them.
type overrides struct {
	enabled map[string]bool
	names   map[string]string
}

func (o overrides) Enabled() map[string]bool { return o.enabled }
func (o overrides) Names() map[string]string { return o.names }

// TestEffective: the overrides become the rule's Overrides and the names;
// without them the rule's own Overrides stay; the ports are handed to
// go-hamapping as they are.
func TestEffective(t *testing.T) {
	rule := expose.Rule{Assist: true, Overrides: map[string]bool{"light.attic": true}}
	if in := plan.Effective(rule, nil, false, nil); !in.Rule.Overrides["light.attic"] || in.Names != nil || in.Cameras || !in.Rule.Assist {
		t.Fatalf("without overrides %+v", in)
	}
	asked := 0
	ports := func(model.Entity) any { asked++; return nil }
	in := plan.Effective(rule, overrides{enabled: map[string]bool{"light.cellar": true}, names: map[string]string{"light.cellar": "Cellar"}}, true, ports)
	if in.Rule.Overrides["light.attic"] || !in.Rule.Overrides["light.cellar"] || in.Names["light.cellar"] != "Cellar" || !in.Cameras || !in.Rule.Assist {
		t.Fatalf("with overrides %+v", in)
	}
	opts := plan.MapOptions(nil, in.Names, in.Ports)
	if opts.Ports == nil || opts.Ports(&model.Basic{EntityKey: "camera.porch", EntityPlatform: "camera"}) != nil || asked != 1 {
		t.Fatalf("the ports were not handed through (asked %d)", asked)
	}
	// The overrides' names reach the plan: a named entity's node label is
	// the name as typed.
	if n, verbatim := opts.Name(&model.Basic{EntityKey: "light.cellar"}); n != "Cellar" || !verbatim {
		t.Fatalf("name %q %v", n, verbatim)
	}
}

// TestWithWithout: the proposed input and the input without overrides
// never write the receiver's maps.
func TestWithWithout(t *testing.T) {
	in := plan.Input{Rule: expose.Rule{Overrides: map[string]bool{"light.a": true, "light.b": false}}, Names: map[string]string{"light.a": "A"}}
	with := in.With(map[string]bool{"light.c": true, "light.a": false}, map[string]string{"light.c": "C"})
	if !with.Rule.Overrides["light.c"] || with.Rule.Overrides["light.a"] || with.Names["light.c"] != "C" || with.Names["light.a"] != "A" {
		t.Fatalf("with %+v", with)
	}
	without := in.Without("light.a")
	if _, ok := without.Rule.Overrides["light.a"]; ok || len(without.Rule.Overrides) != 1 || without.Names["light.a"] != "A" {
		t.Fatalf("without %+v", without)
	}
	without.Names["light.z"] = "Z"
	if len(in.Rule.Overrides) != 2 || !in.Rule.Overrides["light.a"] || len(in.Names) != 1 {
		t.Fatalf("the receiver moved: %+v", in)
	}
	if empty := (plan.Input{}).With(nil, nil); empty.Rule.Overrides == nil || empty.Names == nil {
		t.Fatalf("an empty input's maps %+v", empty)
	}
}

// TestWarnings: a label value that names no label is a finding with the
// text the log and the web UI carry; none is nil.
func TestWarnings(t *testing.T) {
	labels := plan.Labels(snapshot(t))
	in := plan.Input{Rule: expose.Rule{IncludeLabels: []string{"MATTER", "Later"}}}
	got := plan.Warnings(in, labels)
	want := `label "Later" names no Home Assistant label; it matches a label with that id once one exists`
	if len(got) != 1 || got[0] != want {
		t.Fatalf("warnings %q", got)
	}
	if got := plan.Warnings(plan.Input{Rule: expose.Rule{IncludeLabels: []string{"MATTER"}}}, labels); got != nil {
		t.Fatalf("no finding %q", got)
	}
}

// TestRemovedKey: the key is the one go-hamapping gives the entity's
// endpoint, for an entity on a registry device and one on its own; an
// entity without a registry entry, and a missing snapshot, have none.
func TestRemovedKey(t *testing.T) {
	s := snapshot(t)
	_, groups := plan.Assess(s, plan.Input{Rule: expose.Rule{Include: []string{"*"}}})
	served := map[string]string{}
	for _, g := range groups {
		p, _ := hamapping.Map(g.Device, g.Entities, plan.MapOptions(g.Regs, g.Names, nil))
		for i := range p.Endpoints {
			served[p.Endpoints[i].Key] = p.Endpoints[i].Options.StableKey.String()
		}
	}
	for _, id := range []string{"light.kitchen", "sensor.outdoor"} {
		if k := plan.RemovedKey(s, id); k == nil || served[id] == "" || k.String() != served[id] {
			t.Errorf("%s: %v, served as %q", id, k, served[id])
		}
	}
	if k := plan.RemovedKey(s, "sensor.legacy_yaml"); k != nil {
		t.Errorf("an entity without a registry entry: %v", k)
	}
	if k := plan.RemovedKey(nil, "light.kitchen"); k != nil {
		t.Errorf("no snapshot: %v", k)
	}
}

// TestCandidates: an entity the rule leaves out still has go-hamapping's
// verdict; a disabled one stays out.
func TestCandidates(t *testing.T) {
	all := plan.Candidates(snapshot(t), plan.Input{Rule: expose.Rule{}})
	if v := all["sensor.living_battery"]; v == nil || !v.Exposed || v.State == "" || v.Full.State.String() != v.State {
		t.Fatalf("battery %+v", v)
	}
	if v := all["light.cellar"]; v == nil || v.Exposed {
		t.Fatalf("disabled %+v", v)
	}
}

// TestDescribe: a group's entity described with its own state is its
// recorded description; a capability that moves changes it.
func TestDescribe(t *testing.T) {
	s := snapshot(t)
	_, groups := plan.Assess(s, plan.Input{Rule: expose.Rule{Include: []string{"light.kitchen"}}})
	if len(groups) != 1 {
		t.Fatalf("groups %d", len(groups))
	}
	g := groups[0]
	var st ha.State
	for i := range s.States {
		if s.States[i].EntityID == "light.kitchen" {
			st = s.States[i]
		}
	}
	unit := s.Config.TemperatureUnit()
	if g.Describe("light.kitchen", &st, unit) != g.Descs["light.kitchen"] {
		t.Fatal("the same state describes differently")
	}
	moved := st
	moved.Attributes = map[string]any{"supported_color_modes": []any{"onoff"}}
	if g.Describe("light.kitchen", &moved, unit) == g.Descs["light.kitchen"] {
		t.Fatal("a moved capability describes the same")
	}
}
