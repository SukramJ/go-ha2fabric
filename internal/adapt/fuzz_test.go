// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package adapt_test

import (
	"encoding/json"
	"testing"

	hamapping "github.com/SukramJ/go-hamapping"
	"github.com/SukramJ/go-hamqtt/model"

	"github.com/SukramJ/go-ha2fabric/internal/adapt"
	"github.com/SukramJ/go-ha2fabric/internal/ha"
)

// FuzzEntityState holds the translation of an entity's state and
// attributes (hostile entity data from any integration) into go-hamqtt's
// model against any value: Entity and State never panic, and the entity
// keeps its id and domain.
func FuzzEntityState(f *testing.F) {
	f.Add("light.a", "on", []byte(`{"brightness":180,"color_temp_kelvin":2702,"hs_color":[28.4,65.7],"xy_color":[0.5,0.4],"color_mode":"color_temp","supported_color_modes":["hs","color_temp"]}`))
	f.Add("climate.a", "heat", []byte(`{"hvac_modes":["off","heat",7],"min_temp":"x","target_temp_low":null,"supported_features":1e30}`))
	f.Add("sensor.a", "unavailable", []byte(`{"device_class":["x"],"unit_of_measurement":{}}`))
	f.Add("cover.a", "open", []byte(`{"current_position":-5,"supported_features":-1}`))
	f.Add("", "", []byte(`null`))
	f.Fuzz(func(t *testing.T, id, state string, attrs []byte) {
		var a map[string]any
		_ = json.Unmarshal(attrs, &a)
		st := &ha.State{EntityID: id, State: state, Attributes: a}
		e := adapt.Entity(adapt.Input{EntityID: id, State: st, Device: adapt.OwnDevice(id, id), TemperatureUnit: "°C"})
		if e == nil || e.Key() != id || string(e.Platform()) != ha.Domain(id) {
			t.Fatalf("Entity(%q) = %+v", id, e)
		}
		_ = adapt.State(st)
	})
}

// FuzzService holds the translation of a controller's command into a
// call_service (go-hamapping's rendered payload and method, the entity's
// current attributes) against any value: it never panics, an accepted
// command calls a service of the entity's own domain on the entity, and a
// light's brightness is within 0..255.
func FuzzService(f *testing.F) {
	attrs := []byte(`{"supported_features":65535,"hvac_modes":["off","heat","cool","heat_cool"],"target_temp_low":18,"target_temp_high":24,"operation_list":["eco","off"],"is_volume_muted":false}`)
	for _, c := range [][3]string{
		{"light.a", "", `{"state":"ON","brightness":300,"color_temp":2700,"color":{"h":30,"s":50}}`},
		{"light.a", "", `{"state":"ON","color_temp":1e300}`},
		{"light.a", "", `{"state":"OFF"}`},
		{"switch.a", "", "ON"},
		{"cover.a", hamapping.MethodSetPosition, "150"},
		{"cover.a", hamapping.MethodTilt, "-1e400"},
		{"climate.a", hamapping.MethodTurnOn, ""},
		{"fan.a", hamapping.MethodToggle, "x"},
		{"media_player.a", hamapping.MethodMediaPlay, ""},
	} {
		f.Add(c[0], c[1], []byte(c[2]), attrs)
	}
	f.Fuzz(func(t *testing.T, id, method string, payload, attrs []byte) {
		var a map[string]any
		_ = json.Unmarshal(attrs, &a)
		st := &ha.State{EntityID: id, State: "on", Attributes: a}
		e := adapt.Entity(adapt.Input{EntityID: id, State: st, Device: adapt.OwnDevice(id, id), TemperatureUnit: "°C"})
		sc, err := adapt.Service(model.Command{Entity: e, Method: method, Payload: payload}, st)
		if err != nil {
			return
		}
		if sc.Domain != ha.Domain(id) || sc.EntityID != id || sc.Service == "" {
			t.Fatalf("Service(%q, %q, %q) = %v", id, method, payload, sc)
		}
		if bri, ok := sc.Data["brightness"].(int); ok && (bri < 0 || bri > 255) {
			t.Fatalf("Service(%q, %q, %q): brightness %d", id, method, payload, bri)
		}
	})
}
