// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package adapt_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/SukramJ/go-fabric/eligibility"
	hamapping "github.com/SukramJ/go-hamapping"
	"github.com/SukramJ/go-hamqtt/model"

	"github.com/SukramJ/go-ha2fabric/internal/adapt"
	"github.com/SukramJ/go-ha2fabric/internal/ha"
)

func state(id, s string, attrs map[string]any) *ha.State {
	return &ha.State{EntityID: id, State: s, Attributes: attrs}
}

func entity(id string, st *ha.State) *model.Basic {
	dev := adapt.OwnDevice(id, id)
	return adapt.Entity(adapt.Input{EntityID: id, State: st, Device: dev, TemperatureUnit: "°C"})
}

// TestEntitiesMeetTheirRows is the adapter's table: every domain
// go-hamapping maps, each light colour mode, the sensor and binary_sensor
// device classes; each entity reaches the row the table means for it.
func TestEntitiesMeetTheirRows(t *testing.T) {
	for _, c := range []struct {
		name  string
		st    *ha.State
		row   string
		state eligibility.State
	}{
		{"light hs", state("light.a", "on", map[string]any{"supported_color_modes": []any{"hs"}, "color_mode": "hs", "hs_color": []any{30.0, 50.0}, "brightness": 128.0}), "light/extended_color", eligibility.StateMappable},
		{"light xy+ct", state("light.b", "on", map[string]any{"supported_color_modes": []any{"color_temp", "xy"}, "min_color_temp_kelvin": 2000.0, "max_color_temp_kelvin": 6500.0}), "light/extended_color", eligibility.StateMappable},
		{"light rgb", state("light.c", "on", map[string]any{"supported_color_modes": []any{"rgb"}}), "light/rgb", eligibility.StatePartial},
		{"light rgbww", state("light.c2", "on", map[string]any{"supported_color_modes": []any{"rgbww"}}), "light/rgb", eligibility.StatePartial},
		{"light ct", state("light.d", "on", map[string]any{"supported_color_modes": []any{"color_temp"}, "min_color_temp_kelvin": 2200.0, "max_color_temp_kelvin": 6500.0, "color_temp_kelvin": 2700.0}), "light/color_temperature", eligibility.StateMappable},
		{"light brightness", state("light.e", "off", map[string]any{"supported_color_modes": []any{"brightness"}}), "light/dimmable", eligibility.StateMappable},
		{"light white", state("light.e2", "off", map[string]any{"supported_color_modes": []any{"white", "hs"}}), "light/extended_color", eligibility.StateMappable},
		{"light onoff", state("light.f", "on", map[string]any{"supported_color_modes": []any{"onoff"}}), "light/onoff", eligibility.StateMappable},
		{"switch outlet", state("switch.a", "on", map[string]any{"device_class": "outlet"}), "switch/plug", eligibility.StateMappable},
		{"switch plain", state("switch.b", "off", map[string]any{}), "switch/plug", eligibility.StateMappable},
		{"temperature", state("sensor.t", "21.5", map[string]any{"device_class": "temperature", "unit_of_measurement": "°C", "state_class": "measurement"}), "sensor/temperature", eligibility.StateMappable},
		{"temperature F", state("sensor.tf", "70", map[string]any{"device_class": "temperature", "unit_of_measurement": "°F"}), "sensor/temperature", eligibility.StateMappable},
		{"humidity", state("sensor.h", "40", map[string]any{"device_class": "humidity", "unit_of_measurement": "%"}), "sensor/humidity", eligibility.StateMappable},
		{"pressure", state("sensor.p", "1013", map[string]any{"device_class": "atmospheric_pressure", "unit_of_measurement": "hPa"}), "sensor/pressure", eligibility.StateMappable},
		{"illuminance", state("sensor.l", "300", map[string]any{"device_class": "illuminance", "unit_of_measurement": "lx"}), "sensor/illuminance", eligibility.StateMappable},
		{"power", state("sensor.w", "60", map[string]any{"device_class": "power", "unit_of_measurement": "W"}), "sensor/power", eligibility.StateMappable},
		{"energy", state("sensor.kwh", "1.5", map[string]any{"device_class": "energy", "unit_of_measurement": "kWh", "state_class": "total_increasing"}), "sensor/energy", eligibility.StateMappable},
		{"battery alone", state("sensor.b", "80", map[string]any{"device_class": "battery", "unit_of_measurement": "%"}), "sensor/battery", eligibility.StatePartial},
		{"no device class", state("sensor.cpu", "4", map[string]any{"unit_of_measurement": "%"}), "sensor/no_device_class", eligibility.StateUnmappable},
		{"co2", state("sensor.co2", "400", map[string]any{"device_class": "carbon_dioxide", "unit_of_measurement": "ppm"}), "sensor/carbon_dioxide", eligibility.StateMappable},
		{"door", state("binary_sensor.d", "off", map[string]any{"device_class": "door"}), "binary_sensor/contact", eligibility.StateMappable},
		{"motion", state("binary_sensor.m", "on", map[string]any{"device_class": "motion"}), "binary_sensor/motion", eligibility.StateMappable},
		{"occupancy", state("binary_sensor.o", "on", map[string]any{"device_class": "occupancy"}), "binary_sensor/occupancy", eligibility.StateMappable},
		{"moisture", state("binary_sensor.w", "off", map[string]any{"device_class": "moisture"}), "binary_sensor/water_leak", eligibility.StateMappable},
		{"smoke", state("binary_sensor.s", "off", map[string]any{"device_class": "smoke"}), "binary_sensor/smoke", eligibility.StateMappable},
		{"generic binary", state("binary_sensor.g", "off", map[string]any{}), "binary_sensor/generic", eligibility.StateMappable},
		{"connectivity", state("binary_sensor.c", "on", map[string]any{"device_class": "connectivity"}), "binary_sensor/no_device_type", eligibility.StateUnmappable},
		{"cover", state("cover.a", "open", map[string]any{"device_class": "blind", "current_position": 70.0, "supported_features": 15.0}), "cover/window_covering", eligibility.StateMappable},
		// A cover without SET_POSITION: go-fabric v0.11.0's WindowCovering
		// server refuses Lift without PositionAwareLift (report H2, Q).
		{"garage", state("cover.g", "closed", map[string]any{"device_class": "garage", "supported_features": 3.0}), "cover/window_covering", eligibility.StatePartial},
		{"climate heat", state("climate.a", "heat", map[string]any{"hvac_modes": []any{"off", "heat"}, "min_temp": 7.0, "max_temp": 35.0, "current_temperature": 20.0, "temperature": 21.0, "supported_features": 1.0}), "climate/thermostat", eligibility.StateMappable},
		{"climate presets", state("climate.b", "heat_cool", map[string]any{"hvac_modes": []any{"off", "heat_cool", "auto"}, "preset_modes": []any{"eco"}, "fan_modes": []any{"low"}}), "climate/thermostat", eligibility.StatePartial},
		{"climate fan only", state("climate.c", "fan_only", map[string]any{"hvac_modes": []any{"off", "fan_only"}}), "climate/no_heating_or_cooling", eligibility.StateUnmappable},
		{"lock", state("lock.a", "locked", map[string]any{"supported_features": 0.0}), "lock/door_lock", eligibility.StateMappable},
		{"lock open", state("lock.b", "locked", map[string]any{"supported_features": 1.0}), "lock/door_lock", eligibility.StatePartial},
		{"fan", state("fan.a", "on", map[string]any{"percentage": 50.0, "oscillating": false, "supported_features": 51.0}), "fan/fan", eligibility.StateMappable},
		{"fan presets", state("fan.b", "on", map[string]any{"percentage": 50.0, "preset_modes": []any{"auto", "sleep"}, "supported_features": 9.0}), "fan/fan", eligibility.StatePartial},
		{"button", state("button.a", "unknown", map[string]any{}), "button/plug", eligibility.StateMappable},
		{"media player", state("media_player.tv", "off", map[string]any{}), "media_player/unmapped", eligibility.StateUnmappable},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := entity(c.st.EntityID, c.st)
			p, verdicts := hamapping.Map(adapt.OwnDevice(c.st.EntityID, c.name), []model.Entity{e})
			if len(verdicts) != 1 {
				t.Fatalf("verdicts %+v", verdicts)
			}
			v := verdicts[0]
			if v.Row != c.row || v.State != c.state {
				t.Fatalf("verdict %s %v (%s), want %s %v", v.Row, v.State, v.Reason, c.row, c.state)
			}
			if c.state == eligibility.StateUnmappable {
				return
			}
			devs, err := p.Build(context.Background())
			if c.name == "garage" {
				if err == nil {
					t.Fatal("the kit now builds a Lift-only cover: drop the bridge's note and Q")
				}
				return
			}
			if err != nil || len(devs) == 0 {
				t.Fatalf("build: %v", err)
			}
			for _, patch := range p.Apply(context.Background(), e.Key(), adapt.State(c.st)) {
				if patch.Err != nil {
					t.Errorf("apply %s: %v", patch.Cluster, patch.Err)
				}
			}
			p.Close()
		})
	}
}

func TestEntityDescription(t *testing.T) {
	reg := &ha.EntityEntry{EntityID: "light.kitchen", HasEntityName: true, EntityCategory: "config"}
	dev := adapt.Device(&ha.DeviceEntry{
		ID: "dev1", Name: "Kitchen ceiling", NameByUser: "Ceiling", Manufacturer: "Signify", Model: "LCA001",
		SWVersion: "1.1", AreaID: "kitchen", Identifiers: []ha.Tuple{{"hue", "abc"}, {"bad"}}, Connections: []ha.Tuple{{"mac", "00:11"}},
	},
		map[string]string{"kitchen": "Kitchen"})
	if dev.UID() == "" || dev.Name.Default != "Ceiling" || dev.SuggestedArea != "Kitchen" || len(dev.Identity.IDs) != 2 || len(dev.Identity.Connections) != 1 {
		t.Fatalf("device %+v", dev)
	}
	e := adapt.Entity(adapt.Input{EntityID: "light.kitchen", Registry: reg, Device: dev, State: state("light.kitchen", "on", map[string]any{
		"supported_color_modes": []any{"color_temp", "xy"}, "min_color_temp_kelvin": 2000.0, "max_color_temp_kelvin": 6535.0,
		"supported_features": 44.0, "friendly_name": "Ceiling",
	})})
	d := e.Desc()
	want := map[string]any{
		"supported_features": int64(44), "supported_color_modes": []string{"color_temp", "xy"}, "color_temp_kelvin": true,
		"min_kelvin": 2000.0, "max_kelvin": 6535.0, "brightness_scale": 255,
	}
	if !reflect.DeepEqual(d.Extra, want) {
		t.Fatalf("extra %#v", d.Extra)
	}
	if d.Name.Default != "" || d.Category != "config" || e.Platform() != "light" || e.Key() != "light.kitchen" {
		t.Fatalf("description %+v", d)
	}
	if b, ok := model.Bind(e, model.RoleCommand); !ok || b.Slot.Channel != "light.kitchen" || b.Slot.Address != dev.UID() {
		t.Fatalf("command binding %+v", b)
	}
	// The user's name wins; without has_entity_name the friendly name.
	if n := adapt.Entity(adapt.Input{EntityID: "x.y", Registry: &ha.EntityEntry{Name: "Mine", HasEntityName: true}, Device: dev}).Desc().Name.Default; n != "Mine" {
		t.Fatalf("name %q", n)
	}
	if n := adapt.Entity(adapt.Input{EntityID: "x.y", State: state("x.y", "1", map[string]any{"friendly_name": "Friendly"}), Device: dev}).Desc().Name.Default; n != "Friendly" {
		t.Fatalf("name %q", n)
	}
	// A climate takes the installation's temperature unit and its modes.
	cl := adapt.Entity(adapt.Input{EntityID: "climate.a", Device: dev, TemperatureUnit: "°F", State: state("climate.a", "heat", map[string]any{
		"hvac_modes": []any{"off", "heat"}, "min_temp": 45.0, "max_temp": 95.0, "target_temp_step": 1.0, "fan_modes": []any{}, "preset_modes": []any{"eco"},
	})})
	if cl.Desc().Unit != "°F" || !slices.Equal(cl.Desc().Extra["modes"].([]string), []string{"off", "heat"}) || cl.Desc().Extra["fan_modes"] != nil || cl.Desc().Extra["preset_modes"] == nil || cl.Desc().Extra["temp_step"] != 1.0 {
		t.Fatalf("climate %+v", cl.Desc())
	}
	// A water heater takes the installation's unit, its bounds and its
	// operation list.
	wh := adapt.Entity(adapt.Input{EntityID: "water_heater.a", Device: dev, TemperatureUnit: "°C", State: state("water_heater.a", "eco", map[string]any{
		"min_temp": 43.3, "max_temp": 60.0, "operation_list": []any{"off", "eco", "performance"}, "supported_features": 11.0,
	})})
	if wd := wh.Desc(); wd.Unit != "°C" || wd.Extra["min_temp"] != 43.3 || wd.Extra["max_temp"] != 60.0 || !slices.Equal(wd.Extra["operation_list"].([]string), []string{"off", "eco", "performance"}) {
		t.Fatalf("water heater %+v", wd)
	}
	// A select's options, in Home Assistant's order, and a number's bounds.
	sel := adapt.Entity(adapt.Input{EntityID: "number.n", Device: dev, State: state("number.n", "3", map[string]any{"options": []any{"b", "a"}, "min": 1.0, "max": 9.0, "step": 0.5})})
	if sel.Desc().Options == nil || !slices.Equal(sel.Desc().Options.Codes, []string{"b", "a"}) || *sel.Desc().Min != 1 || *sel.Desc().Max != 9 || *sel.Desc().Step != 0.5 {
		t.Fatalf("options/bounds %+v", sel.Desc())
	}
}

func TestState(t *testing.T) {
	st := adapt.State(state("light.a", "on", map[string]any{
		"brightness": 180.0, "color_temp_kelvin": 2702.0, "color_temp": 370.0, "hs_color": []any{28.4, 65.7}, "xy_color": []any{0.526, 0.387}, "color_mode": "color_temp",
	}))
	if st.Value != "on" || !st.Available || st.Extra["color_temp"] != 2702.0 {
		t.Fatalf("light state %+v", st)
	}
	if c := st.Extra["color"].(map[string]any); c["h"] != 28.4 || c["s"] != 65.7 || c["x"] != 0.526 || c["y"] != 0.387 {
		t.Fatalf("color %+v", c)
	}
	off := adapt.State(state("light.a", "off", map[string]any{"color_temp": 370.0, "hs_color": nil}))
	if _, ok := off.Extra["color_temp"]; ok || off.Extra["color"] != nil {
		t.Fatalf("off light %+v", off.Extra)
	}
	for _, s := range []string{ha.StateUnavailable, ha.StateUnknown} {
		if adapt.State(state("sensor.a", s, nil)).Available {
			t.Fatalf("%s is available", s)
		}
	}
	if st := adapt.State(nil); st.Available || st.Value != nil {
		t.Fatalf("nil state %+v", st)
	}
	fan := adapt.State(state("fan.a", "on", map[string]any{"percentage": 50.0, "oscillating": true}))
	if fan.Extra["percentage"] != 50.0 || fan.Extra["oscillating"] != true {
		t.Fatalf("fan state %+v", fan)
	}
}

func cmd(id, method, payload string) model.Command {
	e := entity(id, state(id, "on", map[string]any{}))
	return model.Command{Entity: e, Method: method, Payload: []byte(payload)}
}

func TestService(t *testing.T) {
	climate := state("climate.a", "heat_cool", map[string]any{"target_temp_low": 19.0, "target_temp_high": 24.0})
	// water_heater/const.py:61-67: TARGET_TEMPERATURE 1, OPERATION_MODE 2,
	// ON_OFF 8.
	heater := state("water_heater.a", "eco", map[string]any{"supported_features": 11.0, "operation_list": []any{"off", "eco", "performance"}})
	heaterNoOnOff := state("water_heater.a", "off", map[string]any{"supported_features": 3.0, "operation_list": []any{"off", "eco"}})
	heaterNoOffMode := state("water_heater.a", "eco", map[string]any{"supported_features": 11.0, "operation_list": []any{"eco"}})
	muted := state("media_player.a", "on", map[string]any{"is_volume_muted": true})
	for _, c := range []struct {
		cmd  model.Command
		cur  *ha.State
		want ha.ServiceCall
	}{
		{cmd("light.a", "", `{"state":"OFF"}`), nil, ha.ServiceCall{Domain: "light", Service: "turn_off", EntityID: "light.a"}},
		{cmd("light.a", "", `{"state":"ON"}`), nil, ha.ServiceCall{Domain: "light", Service: "turn_on", EntityID: "light.a"}},
		{cmd("light.a", "", `{"state":"ON","brightness":127.99}`), nil, ha.ServiceCall{Domain: "light", Service: "turn_on", EntityID: "light.a", Data: map[string]any{"brightness": 128}}},
		{cmd("light.a", "", `{"state":"ON","color_temp":2702.7}`), nil, ha.ServiceCall{Domain: "light", Service: "turn_on", EntityID: "light.a", Data: map[string]any{"color_temp_kelvin": 2703}}},
		{cmd("light.a", "", `{"state":"ON","color":{"h":30.5,"s":60}}`), nil, ha.ServiceCall{Domain: "light", Service: "turn_on", EntityID: "light.a", Data: map[string]any{"hs_color": []float64{30.5, 60}}}},
		{cmd("light.a", "", `{"state":"ON","color":{"x":0.3,"y":0.4}}`), nil, ha.ServiceCall{Domain: "light", Service: "turn_on", EntityID: "light.a", Data: map[string]any{"xy_color": []float64{0.3, 0.4}}}},
		{cmd("light.a", "", `{"state":"ON","color":{"r":255,"g":127.6,"b":-3}}`), nil, ha.ServiceCall{Domain: "light", Service: "turn_on", EntityID: "light.a", Data: map[string]any{"rgb_color": []int{255, 128, 0}}}},
		{cmd("switch.a", "", "ON"), nil, ha.ServiceCall{Domain: "switch", Service: "turn_on", EntityID: "switch.a"}},
		{cmd("switch.a", "", "OFF"), nil, ha.ServiceCall{Domain: "switch", Service: "turn_off", EntityID: "switch.a"}},
		{cmd("fan.a", "", "OFF"), nil, ha.ServiceCall{Domain: "fan", Service: "turn_off", EntityID: "fan.a"}},
		{cmd("fan.a", "percentage", "66"), nil, ha.ServiceCall{Domain: "fan", Service: "set_percentage", EntityID: "fan.a", Data: map[string]any{"percentage": 66}}},
		{cmd("fan.a", "preset_mode", "auto"), nil, ha.ServiceCall{Domain: "fan", Service: "set_preset_mode", EntityID: "fan.a", Data: map[string]any{"preset_mode": "auto"}}},
		{cmd("fan.a", "oscillation", "oscillate_on"), nil, ha.ServiceCall{Domain: "fan", Service: "oscillate", EntityID: "fan.a", Data: map[string]any{"oscillating": true}}},
		{cmd("fan.a", "oscillation", "oscillate_off"), nil, ha.ServiceCall{Domain: "fan", Service: "oscillate", EntityID: "fan.a", Data: map[string]any{"oscillating": false}}},
		{cmd("fan.a", "direction", "reverse"), nil, ha.ServiceCall{Domain: "fan", Service: "set_direction", EntityID: "fan.a", Data: map[string]any{"direction": "reverse"}}},
		{cmd("cover.a", "", "OPEN"), nil, ha.ServiceCall{Domain: "cover", Service: "open_cover", EntityID: "cover.a"}},
		{cmd("cover.a", "", "CLOSE"), nil, ha.ServiceCall{Domain: "cover", Service: "close_cover", EntityID: "cover.a"}},
		{cmd("cover.a", "", "STOP"), nil, ha.ServiceCall{Domain: "cover", Service: "stop_cover", EntityID: "cover.a"}},
		{cmd("cover.a", "set_position", "30"), nil, ha.ServiceCall{Domain: "cover", Service: "set_cover_position", EntityID: "cover.a", Data: map[string]any{"position": 30}}},
		{cmd("cover.a", "tilt", "120"), nil, ha.ServiceCall{Domain: "cover", Service: "set_cover_tilt_position", EntityID: "cover.a", Data: map[string]any{"tilt_position": 100}}},
		{cmd("lock.a", "", "LOCK"), nil, ha.ServiceCall{Domain: "lock", Service: "lock", EntityID: "lock.a"}},
		{cmd("lock.a", "", "UNLOCK"), nil, ha.ServiceCall{Domain: "lock", Service: "unlock", EntityID: "lock.a"}},
		{cmd("climate.a", "mode", "heat_cool"), nil, ha.ServiceCall{Domain: "climate", Service: "set_hvac_mode", EntityID: "climate.a", Data: map[string]any{"hvac_mode": "heat_cool"}}},
		{cmd("climate.a", "temperature", "21.5"), nil, ha.ServiceCall{Domain: "climate", Service: "set_temperature", EntityID: "climate.a", Data: map[string]any{"temperature": 21.5}}},
		{cmd("climate.a", "temperature_low", "18"), climate, ha.ServiceCall{Domain: "climate", Service: "set_temperature", EntityID: "climate.a", Data: map[string]any{"target_temp_low": 18.0, "target_temp_high": 24.0}}},
		{cmd("climate.a", "temperature_high", "25"), climate, ha.ServiceCall{Domain: "climate", Service: "set_temperature", EntityID: "climate.a", Data: map[string]any{"target_temp_high": 25.0, "target_temp_low": 19.0}}},
		{cmd("climate.a", hamapping.MethodMode, "dry"), nil, ha.ServiceCall{Domain: "climate", Service: "set_hvac_mode", EntityID: "climate.a", Data: map[string]any{"hvac_mode": "dry"}}},
		{cmd("climate.a", hamapping.MethodFanMode, "medium"), nil, ha.ServiceCall{Domain: "climate", Service: "set_fan_mode", EntityID: "climate.a", Data: map[string]any{"fan_mode": "medium"}}},
		{cmd("climate.a", hamapping.MethodTurnOn, "ON"), nil, ha.ServiceCall{Domain: "climate", Service: "turn_on", EntityID: "climate.a"}},
		{cmd("climate.a", hamapping.MethodTurnOff, "OFF"), nil, ha.ServiceCall{Domain: "climate", Service: "turn_off", EntityID: "climate.a"}},
		{cmd("button.a", hamapping.MethodPress, "PRESS"), nil, ha.ServiceCall{Domain: "button", Service: "press", EntityID: "button.a"}},
		{cmd("select.a", hamapping.MethodSelectOption, "Movie night"), nil, ha.ServiceCall{Domain: "select", Service: "select_option", EntityID: "select.a", Data: map[string]any{"option": "Movie night"}}},
		{cmd("vacuum.a", hamapping.MethodRunMode, "1"), nil, ha.ServiceCall{Domain: "vacuum", Service: "start", EntityID: "vacuum.a"}},
		{cmd("vacuum.a", hamapping.MethodRunMode, "0"), nil, ha.ServiceCall{Domain: "vacuum", Service: "return_to_base", EntityID: "vacuum.a"}},
		{cmd("vacuum.a", hamapping.MethodPause, "PAUSE"), nil, ha.ServiceCall{Domain: "vacuum", Service: "pause", EntityID: "vacuum.a"}},
		{cmd("vacuum.a", hamapping.MethodResume, "START"), nil, ha.ServiceCall{Domain: "vacuum", Service: "start", EntityID: "vacuum.a"}},
		{cmd("vacuum.a", hamapping.MethodGoHome, "RETURN_TO_BASE"), nil, ha.ServiceCall{Domain: "vacuum", Service: "return_to_base", EntityID: "vacuum.a"}},
		{cmd("valve.a", hamapping.MethodOpenValve, "OPEN"), nil, ha.ServiceCall{Domain: "valve", Service: "open_valve", EntityID: "valve.a"}},
		{cmd("valve.a", hamapping.MethodCloseValve, "CLOSE"), nil, ha.ServiceCall{Domain: "valve", Service: "close_valve", EntityID: "valve.a"}},
		{cmd("water_heater.a", hamapping.MethodOperationMode, "off"), heater, ha.ServiceCall{Domain: "water_heater", Service: "set_operation_mode", EntityID: "water_heater.a", Data: map[string]any{"operation_mode": "off"}}},
		{cmd("water_heater.a", hamapping.MethodOperationMode, "on"), heater, ha.ServiceCall{Domain: "water_heater", Service: "turn_on", EntityID: "water_heater.a"}},
		{cmd("water_heater.a", hamapping.MethodOperationMode, "on"), heaterNoOnOff, ha.ServiceCall{Domain: "water_heater", Service: "set_operation_mode", EntityID: "water_heater.a", Data: map[string]any{"operation_mode": "eco"}}},
		{cmd("water_heater.a", hamapping.MethodOperationMode, "off"), heaterNoOffMode, ha.ServiceCall{Domain: "water_heater", Service: "turn_off", EntityID: "water_heater.a"}},
		{cmd("water_heater.a", hamapping.MethodTemperature, "55.5"), nil, ha.ServiceCall{Domain: "water_heater", Service: "set_temperature", EntityID: "water_heater.a", Data: map[string]any{"temperature": 55.5}}},
		{cmd("media_player.a", hamapping.MethodTurnOn, "ON"), nil, ha.ServiceCall{Domain: "media_player", Service: "turn_on", EntityID: "media_player.a"}},
		{cmd("media_player.a", hamapping.MethodTurnOff, "OFF"), nil, ha.ServiceCall{Domain: "media_player", Service: "turn_off", EntityID: "media_player.a"}},
		{cmd("media_player.a", hamapping.MethodVolumeMute, "true"), nil, ha.ServiceCall{Domain: "media_player", Service: "volume_mute", EntityID: "media_player.a", Data: map[string]any{"is_volume_muted": true}}},
		{cmd("media_player.a", hamapping.MethodVolumeMute, "false"), nil, ha.ServiceCall{Domain: "media_player", Service: "volume_mute", EntityID: "media_player.a", Data: map[string]any{"is_volume_muted": false}}},
		{cmd("media_player.a", hamapping.MethodVolumeLevel, "127"), nil, ha.ServiceCall{Domain: "media_player", Service: "volume_set", EntityID: "media_player.a", Data: map[string]any{"volume_level": 0.5}}},
		{cmd("media_player.a", hamapping.MethodVolumeLevel, "254"), nil, ha.ServiceCall{Domain: "media_player", Service: "volume_set", EntityID: "media_player.a", Data: map[string]any{"volume_level": 1.0}}},
		{cmd("media_player.a", hamapping.MethodVolumeLevel, "200"), nil, ha.ServiceCall{Domain: "media_player", Service: "volume_set", EntityID: "media_player.a", Data: map[string]any{"volume_level": 0.79}}},
		{cmd("media_player.a", hamapping.MethodMediaPlay, "PLAY"), nil, ha.ServiceCall{Domain: "media_player", Service: "media_play", EntityID: "media_player.a"}},
		{cmd("media_player.a", hamapping.MethodMediaPause, "PAUSE"), nil, ha.ServiceCall{Domain: "media_player", Service: "media_pause", EntityID: "media_player.a"}},
		{cmd("media_player.a", hamapping.MethodMediaStop, "STOP"), nil, ha.ServiceCall{Domain: "media_player", Service: "media_stop", EntityID: "media_player.a"}},
		{cmd("media_player.a", hamapping.MethodKey, "65"), nil, ha.ServiceCall{Domain: "media_player", Service: "volume_up", EntityID: "media_player.a"}},
		{cmd("media_player.a", hamapping.MethodKey, "66"), nil, ha.ServiceCall{Domain: "media_player", Service: "volume_down", EntityID: "media_player.a"}},
		{cmd("media_player.a", hamapping.MethodKey, "67"), muted, ha.ServiceCall{Domain: "media_player", Service: "volume_mute", EntityID: "media_player.a", Data: map[string]any{"is_volume_muted": false}}},
		{cmd("media_player.a", hamapping.MethodKey, "67"), nil, ha.ServiceCall{Domain: "media_player", Service: "volume_mute", EntityID: "media_player.a", Data: map[string]any{"is_volume_muted": true}}},
		{cmd("media_player.a", hamapping.MethodKey, "64"), nil, ha.ServiceCall{Domain: "media_player", Service: "toggle", EntityID: "media_player.a"}},
	} {
		got, err := adapt.Service(c.cmd, c.cur)
		if err != nil || !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s %q %s: %+v %v, want %+v", c.cmd.Entity.Key(), c.cmd.Method, c.cmd.Payload, got, err, c.want)
		}
	}
	for _, c := range []model.Command{
		cmd("switch.a", "", "MAYBE"),
		cmd("light.a", "", "not json"),
		cmd("cover.a", "set_position", "x"),
		cmd("climate.a", "temperature_low", "18"),
		cmd("climate.a", "mode", ""),
		cmd("fan.a", "direction", "sideways"),
		cmd("fan.a", "oscillation", "maybe"),
		cmd("media_player.a", "", "ON"),
		cmd("climate.a", hamapping.MethodFanMode, ""),
		cmd("climate.a", hamapping.MethodPresetMode, "eco"),
		cmd("select.a", hamapping.MethodSelectOption, ""),
		cmd("vacuum.a", hamapping.MethodRunMode, "2"),
		cmd("water_heater.a", hamapping.MethodOperationMode, "on"),
		cmd("water_heater.a", hamapping.MethodOperationMode, "eco"),
		cmd("water_heater.a", hamapping.MethodTemperature, "hot"),
		cmd("media_player.a", hamapping.MethodVolumeMute, "maybe"),
		cmd("media_player.a", hamapping.MethodVolumeLevel, "loud"),
		cmd("media_player.a", hamapping.MethodKey, "68"),
		cmd("media_player.a", hamapping.MethodKey, "x"),
		cmd("button.a", hamapping.MethodTurnOn, "ON"),
		{},
	} {
		if _, err := adapt.Service(c, nil); !errors.Is(err, adapt.ErrCommand) {
			t.Errorf("%+v: %v", c, err)
		}
	}
	// A water heater with neither ON_OFF nor an operation mode to turn
	// on to, nor an "off" mode to turn off to.
	bare := state("water_heater.a", "off", map[string]any{"supported_features": 3.0, "operation_list": []any{"off"}})
	if _, err := adapt.Service(cmd("water_heater.a", hamapping.MethodOperationMode, "on"), bare); !errors.Is(err, adapt.ErrCommand) {
		t.Errorf("water heater on without a mode: %v", err)
	}
	bare = state("water_heater.a", "eco", map[string]any{"supported_features": 3.0, "operation_list": []any{"eco"}})
	if _, err := adapt.Service(cmd("water_heater.a", hamapping.MethodOperationMode, "off"), bare); !errors.Is(err, adapt.ErrCommand) {
		t.Errorf("water heater off without a mode: %v", err)
	}
}

// TestAppliedStateIsNotACommand: a state Home Assistant reports reaches the
// device without coming back as a command (no echo loop); the controller's
// side of the round trip is the end-to-end test's (internal/bridge).
func TestAppliedStateIsNotACommand(t *testing.T) {
	st := state("switch.coffee", "off", map[string]any{"device_class": "outlet"})
	e := entity("switch.coffee", st)
	p, _ := hamapping.Map(adapt.OwnDevice("switch.coffee", "Coffee"), []model.Entity{e})
	if _, err := p.Build(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	var got []model.Command
	p.OnChange(func(c model.Command) { got = append(got, c) })
	p.Apply(context.Background(), "switch.coffee", adapt.State(st))
	if len(got) != 0 {
		t.Fatalf("an applied state was handed out: %+v", got)
	}
}
