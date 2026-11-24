// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package bridge_test

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	hamapping "github.com/SukramJ/go-hamapping"
	"thingfabric.dev/go-fabric/eligibility"

	"github.com/SukramJ/go-ha2fabric/internal/adapt"
	"github.com/SukramJ/go-ha2fabric/internal/camera"
	"github.com/SukramJ/go-ha2fabric/internal/expose"
	"github.com/SukramJ/go-ha2fabric/internal/ha"
	"github.com/SukramJ/go-ha2fabric/internal/plan"
)

// knownRefusal reports a build failure go-hamapping v0.7.0 is known to
// call mappable, the thermostat shapes of go-hamapping #45 (decision 49):
// (a) one of min_temp and max_temp stated, beyond go-fabric's default for
// the other; (b) a heat_cool or auto range narrower than go-fabric's
// deadband. Skipped by name until the pin moves to the release that
// decides them; nothing else is.
func knownRefusal(id string, attrs map[string]any, err error) bool {
	platform, _, _ := strings.Cut(id, ".")
	if platform != "climate" && platform != "water_heater" {
		return false
	}
	msg := err.Error()
	_, hasMin := attrs["min_temp"]
	_, hasMax := attrs["max_temp"]
	switch {
	case hasMin != hasMax && strings.Contains(msg, "setpoint configuration cannot be fixed"):
		return true
	case platform == "climate" && strings.Contains(msg, "setpoint limits"):
		modes, _ := attrs["hvac_modes"].([]any)
		return slices.Contains(modes, any("heat_cool")) || slices.Contains(modes, any("auto"))
	}
	return false
}

// FuzzPlanBuild (HR-P2-test-42) takes one entity's hostile state,
// attributes and device text the whole way a sync takes it without a
// node: plan.Assess, go-hamapping's Map, Build and Apply. Nothing panics,
// and what Map calls mappable builds.
func FuzzPlanBuild(f *testing.F) {
	f.Add("select.a", "x", []byte(`{"options":["a","b"],"friendly_name":"S"}`), "Dev", "ACME", "SN1")
	f.Add("light.a", "on", []byte(`{"brightness":180,"color_temp_kelvin":2702,"hs_color":[28.4,65.7],"supported_color_modes":["hs","color_temp"],"min_color_temp_kelvin":0,"max_color_temp_kelvin":1e30}`), "D\x00", "V\u202e", "")
	f.Add("climate.a", "heat", []byte(`{"hvac_modes":["off","heat","cool","heat_cool"],"min_temp":-1e30,"max_temp":1e30,"target_temp_step":0,"temperature":1e300,"fan_modes":["auto"],"supported_features":4095}`), "", "", "")
	f.Add("sensor.a", "1e400", []byte(`{"device_class":"temperature","unit_of_measurement":"°C","state_class":"measurement"}`), "x", "", " ")
	f.Add("cover.a", "open", []byte(`{"current_position":-5,"current_tilt_position":1e9,"supported_features":255}`), "x", "y", "z")
	f.Add("fan.a", "on", []byte(`{"percentage":1e9,"preset_modes":["auto",""],"supported_features":63}`), "x", "y", "z")
	f.Add("media_player.a", "playing", []byte(`{"volume_level":-3,"is_volume_muted":"x","supported_features":1e18,"device_class":"tv"}`), "x", "y", "z")
	f.Add("water_heater.a", "eco", []byte(`{"min_temp":1e9,"max_temp":-1e9,"operation_list":["off"],"supported_features":15,"temperature":null}`), "x", "y", "z")
	f.Add("climate.b", "heat", []byte(`{"hvac_modes":["off","heat"],"min_temp":30,"max_temp":7,"temperature":20,"supported_features":1}`), "x", "y", "z")
	// The shapes of go-hamapping #45 (knownRefusal).
	f.Add("climate.c", "heat", []byte(`{"hvac_modes":["off","heat"],"min_temp":50,"temperature":55,"supported_features":1}`), "x", "y", "z")
	f.Add("climate.d", "heat_cool", []byte(`{"hvac_modes":["off","heat_cool"],"min_temp":20,"max_temp":21,"target_temp_low":20,"target_temp_high":21,"supported_features":2}`), "x", "y", "z")
	f.Add("number.a", "5", []byte(`{"min":1e308,"max":-1e308,"step":0}`), "x", "y", "z")
	f.Add("vacuum.a", "cleaning", []byte(`{"supported_features":65535,"battery_level":1e9}`), "x", "y", "z")
	f.Add("lock.a", "jammed", []byte(`{"supported_features":1}`), "x", "y", "z")
	f.Add("valve.a", "open", []byte(`{"current_position":1e9,"supported_features":15}`), "x", "y", "z")
	f.Add("binary_sensor.a", "on", []byte(`{"device_class":"door"}`), "x", "y", "z")
	f.Add("event.a", "x", []byte(`{"event_types":["a",1],"event_type":"a","device_class":"button"}`), "x", "y", "z")
	f.Add("camera.a", "idle", []byte(`{"supported_features":3,"frontend_stream_type":"web_rtc"}`), "x", "y", "z")
	f.Fuzz(func(t *testing.T, id, state string, attrs []byte, dev, vendor, serial string) {
		if !strings.Contains(id, ".") || len(attrs) > 1<<16 {
			return
		}
		var a map[string]any
		_ = json.Unmarshal(attrs, &a)
		snap := &ha.Snapshot{
			States:   []ha.State{{EntityID: id, State: state, Attributes: a}},
			Entities: []ha.EntityEntry{{ID: "reg1", EntityID: id, DeviceID: "d1", HasEntityName: true, OriginalName: dev}},
			Devices:  []ha.DeviceEntry{{ID: "d1", Name: dev, Manufacturer: vendor, SerialNumber: serial}},
		}
		in := plan.Input{Rule: expose.Rule{Include: []string{"*"}}, Cameras: true, Ports: camera.DryPorts}
		verdicts, groups := plan.Assess(snap, in)
		for _, g := range groups {
			hp, mv := hamapping.Map(g.Device, g.Entities, plan.MapOptions(g.Regs, g.Names, in.Ports))
			plan.MergeVerdicts(verdicts, mv)
			if _, err := hp.Build(context.Background()); err != nil {
				for _, v := range mv {
					if v.State != eligibility.StateUnmappable && !knownRefusal(v.Key, a, err) {
						t.Fatalf("%s is %s and does not build: %v", v.Key, v.State, err)
					}
				}
				hp.Close()
				continue
			}
			for _, p := range hp.Apply(context.Background(), id, adapt.State(&snap.States[0])) {
				_ = p.Err
			}
			hp.Close()
		}
	})
}
