// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

// Package adapt translates Home Assistant's registry and states, as the
// WebSocket API carries them, into go-hamqtt's model vocabulary, which
// go-hamapping's table reads; and a model.Command back into a service
// call. It holds no mapping of its own: which entity becomes which device
// type is go-hamapping's Map.
//
// The table's rows read the entity in the vocabulary of Home Assistant's
// MQTT platforms (go-ha-catalog data/mqtt.json): the JSON light's
// color_temp, color.h/.s/.x/.y and min_kelvin, the climate's modes. A
// WebSocket entity names a few of these differently (hvac_modes,
// hs_color, min_color_temp_kelvin); the adapter states them under the
// catalog's names. Every such key is in this file, with the Home
// Assistant core line it reads (tag 2026.10.3) and the row that reads it.
package adapt

import (
	"maps"
	"strings"

	hacatalog "github.com/SukramJ/go-ha-catalog"
	"github.com/SukramJ/go-hamqtt/model"

	"github.com/SukramJ/go-ha2fabric/internal/ha"
)

// Identifier namespaces of the devices the adapter makes.
const (
	// NamespaceDevice is a registry device: its id, Home Assistant's
	// stable device identity (helpers/device_registry.py DeviceEntry.id).
	NamespaceDevice = "ha"
	// NamespaceEntity is the device of an entity that has none: its
	// entity id.
	NamespaceEntity = "ha-entity"
)

// Device is the model.Device of a registry device: name (the user's
// name over the integration's), manufacturer, model, versions, serial,
// configuration URL, area and identifiers
// (helpers/device_registry.py:589-629 at 2026.10.3). Its first identifier
// is the registry id, so its UID survives a rename.
func Device(d *ha.DeviceEntry, areas map[string]string) *model.Device {
	name := d.NameByUser
	if name == "" {
		name = d.Name
	}
	ids := []model.Identifier{{Namespace: NamespaceDevice, Value: d.ID}}
	for _, id := range d.Identifiers {
		if len(id) == 2 && id[1] != "" {
			ids = append(ids, model.Identifier{Namespace: NamespaceDevice + ":" + id[0], Value: id[1]})
		}
	}
	var conns []model.Connection
	for _, c := range d.Connections {
		if len(c) == 2 {
			conns = append(conns, model.Connection{Type: c[0], Value: c[1]})
		}
	}
	return &model.Device{
		Identity:      model.Identity{IDs: ids, Connections: conns},
		Name:          model.Localized{Default: name},
		Manufacturer:  d.Manufacturer,
		Model:         d.Model,
		ModelID:       d.ModelID,
		SWVersion:     d.SWVersion,
		HWVersion:     d.HWVersion,
		SerialNumber:  d.SerialNumber,
		SuggestedArea: areas[d.AreaID],
		ConfigURL:     d.ConfigurationURL,
	}
}

// OwnDevice is the device of an entity without a registry device: one
// device per entity, named as the entity.
func OwnDevice(entityID, name string) *model.Device {
	return &model.Device{
		Identity: model.Identity{IDs: []model.Identifier{{Namespace: NamespaceEntity, Value: entityID}}},
		Name:     model.Localized{Default: name},
	}
}

// Input is one entity as the daemon read it.
type Input struct {
	// EntityID is the entity's id; its domain is the platform.
	EntityID string
	// Registry is the entity's registry entry; nil for an entity without
	// one (a YAML entity without unique_id).
	Registry *ha.EntityEntry
	// State is the entity's state; its attributes carry the description.
	State *ha.State
	// Device is the entity's device (its UID addresses the bindings).
	Device *model.Device
	// TemperatureUnit is the installation's temperature unit
	// (get_config unit_system.temperature), the unit of a climate's
	// temperatures.
	TemperatureUnit string
}

// Entity is the model.Basic of one entity: EntityKey the entity id,
// EntityPlatform its domain, the Description from the state's attributes
// and the registry entry, and a state and a command binding addressed by
// the device's UID and the entity id. The registry list carries no
// device_class, unit or supported_features (helpers/entity_registry.py:
// 369-403 as_partial_dict); the state's attributes do, the registry's
// device_class already over the integration's (helpers/entity.py:
// 1131-1176), so the adapter reads them there.
func Entity(in Input) *model.Basic {
	domain := ha.Domain(in.EntityID)
	attrs := map[string]any{}
	if in.State != nil && in.State.Attributes != nil {
		attrs = in.State.Attributes
	}
	d := model.Description{
		Name:        model.Localized{Default: entityName(in.Registry, attrs)},
		DeviceClass: model.DeviceClass(str(attrs["device_class"])),
		StateClass:  hacatalog.StateClass(str(attrs["state_class"])),
		Unit:        model.Unit(str(attrs["unit_of_measurement"])),
		Extra:       description(domain, attrs),
	}
	if domain == "climate" {
		// climate/__init__.py:296-316: the temperatures are shown in the
		// installation's unit.
		d.Unit = model.Unit(in.TemperatureUnit)
	}
	if in.Registry != nil && in.Registry.EntityCategory != "" {
		d.Category = hacatalog.EntityCategory(in.Registry.EntityCategory)
	}
	if opts := strings_(attrs["options"]); len(opts) > 0 {
		d.Options = &model.Enum{Codes: opts}
	}
	d.Min, d.Max, d.Step = num(attrs["min"]), num(attrs["max"]), num(attrs["step"])
	slot := func(role string) model.Slot {
		return model.Slot{Address: in.Device.UID(), Channel: in.EntityID, Bucket: model.BucketValues, Path: []string{role}}
	}
	return &model.Basic{
		EntityKey:      in.EntityID,
		EntityPlatform: hacatalog.Platform(domain),
		Description:    d,
		Binds: []model.Binding{
			{Role: model.RoleState, Slot: slot(model.RoleState), Mode: model.Read},
			{Role: model.RoleCommand, Slot: slot(model.RoleCommand), Mode: model.Write},
		},
	}
}

// entityName is the entity's own name: the user's registry name, else the
// integration's name for an entity with has_entity_name (empty for the
// device's main feature, which go-hamapping names after the device), else
// the friendly_name (helpers/entity_registry.py:369-403, helpers/entity.py
// :1173).
func entityName(reg *ha.EntityEntry, attrs map[string]any) string {
	switch {
	case reg != nil && reg.Name != "":
		return reg.Name
	case reg != nil && reg.HasEntityName:
		return reg.OriginalName
	default:
		return str(attrs["friendly_name"])
	}
}

// description is the Description.Extra of an entity: the keys the rows'
// predicates and initial values read, under the catalog's names.
func description(domain string, attrs map[string]any) map[string]any {
	ex := map[string]any{}
	if v, ok := toInt(attrs["supported_features"]); ok {
		// helpers/entity.py:1176; go-hamapping decodes the mask with
		// go-ha-catalog data/features.json (binding.go features).
		ex["supported_features"] = v
	}
	switch domain {
	case "light":
		// light/__init__.py:447-469 (capability attributes). Rows
		// light/*: extra.supported_color_modes, extra.color_temp_kelvin,
		// extra.min_kelvin, extra.max_kelvin, extra.brightness_scale.
		if m := strings_(attrs["supported_color_modes"]); m != nil {
			ex["supported_color_modes"] = m
		}
		// The WebSocket light states its colour temperature in kelvin
		// (light/const.py:115 color_temp_kelvin); the state's color_temp
		// carries it (State below).
		ex["color_temp_kelvin"] = true
		copyNum(ex, "min_kelvin", attrs, "min_color_temp_kelvin")
		copyNum(ex, "max_kelvin", attrs, "max_color_temp_kelvin")
		// light/const.py:151 VALID_BRIGHTNESS: brightness is 0 to 255.
		ex["brightness_scale"] = 255
	case "climate":
		// climate/const.py:114-136 (fan_modes, preset_modes, max_temp,
		// min_temp, hvac_modes, swing_modes). Rows climate/*:
		// extra.modes, extra.min_temp, extra.max_temp, and the presence
		// of fan_modes, preset_modes, swing_modes.
		if m := strings_(attrs["hvac_modes"]); m != nil {
			ex["modes"] = m
		}
		copyNum(ex, "min_temp", attrs, "min_temp")
		copyNum(ex, "max_temp", attrs, "max_temp")
		copyNum(ex, "temp_step", attrs, "target_temp_step")
		for _, k := range []string{"fan_modes", "preset_modes", "swing_modes"} {
			if m := strings_(attrs[k]); len(m) > 0 {
				ex[k] = m
			}
		}
	case "fan":
		// fan/const.py:31 preset_modes; row fan/fan: extra.preset_modes.
		if m := strings_(attrs["preset_modes"]); len(m) > 0 {
			ex["preset_modes"] = m
		}
	}
	return ex
}

// State is the model.State of a state object: Value the state string,
// Extra its attributes with the state keys the rows read under the
// catalog's names (a light's color_temp and color), Available false for
// unavailable and unknown (homeassistant/const.py:303,313 at 2026.10.3).
func State(st *ha.State) model.State {
	if st == nil {
		return model.State{}
	}
	ex := maps.Clone(st.Attributes)
	if ex == nil {
		ex = map[string]any{}
	}
	if ha.Domain(st.EntityID) == "light" {
		// Rows light/*: state.color_temp (kelvin, extra.color_temp_kelvin
		// true), state.color.h/.s from hs_color, state.color.x/.y from
		// xy_color (light/__init__.py:609-660, light/const.py:110-115).
		delete(ex, "color_temp")
		if k, ok := toFloat(ex["color_temp_kelvin"]); ok {
			ex["color_temp"] = k
		}
		color := map[string]any{}
		if hs := floats(ex["hs_color"]); len(hs) == 2 {
			color["h"], color["s"] = hs[0], hs[1]
		}
		if xy := floats(ex["xy_color"]); len(xy) == 2 {
			color["x"], color["y"] = xy[0], xy[1]
		}
		if len(color) > 0 {
			ex["color"] = color
		} else {
			delete(ex, "color")
		}
	}
	available := st.State != ha.StateUnavailable && st.State != ha.StateUnknown
	return model.State{Value: st.State, Available: available, Extra: ex}
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func strings_(v any) []string { //nolint:revive // "strings" is the package
	l, ok := v.([]any)
	if !ok {
		if s, ok := v.([]string); ok {
			return s
		}
		return nil
	}
	out := make([]string, 0, len(l))
	for _, x := range l {
		if s, ok := x.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}

func toFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	}
	return 0, false
}

func toInt(v any) (int64, bool) {
	f, ok := toFloat(v)
	return int64(f), ok
}

func num(v any) *float64 {
	if f, ok := toFloat(v); ok {
		return &f
	}
	return nil
}

func floats(v any) []float64 {
	l, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]float64, 0, len(l))
	for _, x := range l {
		f, ok := toFloat(x)
		if !ok {
			return nil
		}
		out = append(out, f)
	}
	return out
}

func copyNum(dst map[string]any, key string, src map[string]any, from string) {
	if f, ok := toFloat(src[from]); ok {
		dst[key] = f
	}
}
