// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package adapt

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	keypadinputdef "github.com/SukramJ/go-fabric/cluster/spec/keypadinput"
	levelcontroldef "github.com/SukramJ/go-fabric/cluster/spec/levelcontrol"
	hacatalog "github.com/SukramJ/go-ha-catalog"
	hamapping "github.com/SukramJ/go-hamapping"
	"github.com/SukramJ/go-hamqtt/model"

	"github.com/SukramJ/go-ha2fabric/internal/ha"
)

// ErrCommand is the error of a command the daemon cannot turn into a
// service call.
var ErrCommand = errors.New("adapt: command has no service")

// The daemon's one service table: a model.Command — the command binding's
// payload, or a named method with its payload, as go-hamapping's command
// rows render them — becomes one call_service. Services and data keys are
// Home Assistant core's at 2026.10.3, cited per entry from the domain's
// services.yaml (the action's fields) and services.py (its schema).

// payloads are the command binding's raw payloads per domain: the
// catalog's payload key (go-ha-catalog data/mqtt.json, the default the
// row reads with catalog: true) and the service it calls.
var payloads = map[string][]struct{ key, service string }{
	// switch/services.yaml:3,8 (turn_on, turn_off); rows switch/plug.
	"switch": {{"payload_on", "turn_on"}, {"payload_off", "turn_off"}},
	// fan/services.yaml:31,56 (turn_on, turn_off); row fan/fan.
	"fan": {{"payload_on", "turn_on"}, {"payload_off", "turn_off"}},
	// cover/services.yaml:3,19,66 (open_cover, close_cover, stop_cover);
	// row cover/window_covering.
	"cover": {{"payload_open", "open_cover"}, {"payload_close", "close_cover"}, {"payload_stop", "stop_cover"}},
	// lock/services.yaml:3,25 (lock, unlock); row lock/door_lock.
	"lock": {{"payload_lock", "lock"}, {"payload_unlock", "unlock"}},
}

// services are the named methods that call one service without data, per
// domain: the method go-hamapping's row delivers (rows/<domain>.yaml at
// the version in go.mod) and the service. The payload is the row's
// constant and carries nothing the service needs.
var services = map[string]map[string]string{
	// button/services.py:11-15 press, no fields; row button/plug.
	"button": {hamapping.MethodPress: "press"},
	// climate/services.py:170-181 turn_on, turn_off, no fields; row
	// climate/air_conditioner.
	"climate": {hamapping.MethodTurnOn: "turn_on", hamapping.MethodTurnOff: "turn_off"},
	// vacuum/services.py:104-121 start, pause, return_to_base, no fields;
	// row vacuum/robotic_vacuum_cleaner (Resume starts again).
	"vacuum": {hamapping.MethodPause: "pause", hamapping.MethodResume: "start", hamapping.MethodGoHome: "return_to_base"},
	// valve/services.py:22-31 open_valve, close_valve, no fields; row
	// valve/water_valve.
	"valve": {hamapping.MethodOpenValve: "open_valve", hamapping.MethodCloseValve: "close_valve"},
	// media_player/services.py:125-130 turn_on, turn_off; :155-163
	// media_play, media_pause, media_stop; no fields. Rows
	// media_player/video_player, speaker.
	"media_player": {
		hamapping.MethodTurnOn: "turn_on", hamapping.MethodTurnOff: "turn_off",
		hamapping.MethodMediaPlay: "media_play", hamapping.MethodMediaPause: "media_pause", hamapping.MethodMediaStop: "media_stop",
	},
}

// Service is the call_service of a controller's command. cur is the
// entity's current state, which a climate range setpoint completes (the
// other bound of the range), a water heater's turning on and a media
// player's mute key read.
func Service(cmd model.Command, cur *ha.State) (ha.ServiceCall, error) {
	if cmd.Entity == nil {
		return ha.ServiceCall{}, fmt.Errorf("%w: no entity", ErrCommand)
	}
	id := cmd.Entity.Key()
	domain := ha.Domain(id)
	call := func(service string, data map[string]any) (ha.ServiceCall, error) {
		return ha.ServiceCall{Domain: domain, Service: service, EntityID: id, Data: data}, nil
	}
	payload := strings.TrimSpace(string(cmd.Payload))
	var attrs map[string]any
	if cur != nil {
		attrs = cur.Attributes
	}
	if service, ok := services[domain][cmd.Method]; ok && cmd.Method != "" {
		return call(service, nil)
	}
	switch {
	case domain == "light" && cmd.Method == "":
		return light(id, cmd.Payload)
	case cmd.Method == "":
		for _, p := range payloads[domain] {
			if want, ok := catalogPayload(cmd.Entity, p.key); ok && payload == want {
				return call(p.service, nil)
			}
		}
	case domain == "cover" && cmd.Method == hamapping.MethodSetPosition:
		// cover/services.yaml:43-56 set_cover_position, position 0 to 100.
		if n, ok := integer(payload, 0, 100); ok {
			return call("set_cover_position", map[string]any{"position": n})
		}
	case domain == "cover" && cmd.Method == hamapping.MethodTilt:
		// cover/services.yaml:95-108 set_cover_tilt_position, tilt_position.
		if n, ok := integer(payload, 0, 100); ok {
			return call("set_cover_tilt_position", map[string]any{"tilt_position": n})
		}
	case domain == "climate" && cmd.Method == hamapping.MethodMode:
		// climate/services.yaml:92-102 set_hvac_mode, hvac_mode.
		if payload != "" {
			return call("set_hvac_mode", map[string]any{"hvac_mode": payload})
		}
	case domain == "climate" && cmd.Method == hamapping.MethodTemperature:
		// climate/services.yaml:17-34 set_temperature, temperature.
		if f, ok := number(payload); ok {
			return call("set_temperature", map[string]any{"temperature": f})
		}
	case domain == "climate" && (cmd.Method == hamapping.MethodTemperatureLow || cmd.Method == hamapping.MethodTemperatureHigh):
		// climate/services.py:49-62: target_temp_low and target_temp_high
		// are Inclusive — both or neither; the other bound is the current.
		f, ok := number(payload)
		if !ok {
			break
		}
		set, other := "target_temp_low", "target_temp_high"
		if cmd.Method == hamapping.MethodTemperatureHigh {
			set, other = other, set
		}
		o, ok := toFloat(attrs[other])
		if !ok {
			return ha.ServiceCall{}, fmt.Errorf("%w: %s %s without a current %s", ErrCommand, id, set, other)
		}
		return call("set_temperature", map[string]any{set: f, other: o})
	case domain == "fan" && cmd.Method == hamapping.MethodPercentage:
		// fan/services.yaml:16-29 set_percentage, percentage 0 to 100.
		if n, ok := integer(payload, 0, 100); ok {
			return call("set_percentage", map[string]any{"percentage": n})
		}
	case domain == "fan" && cmd.Method == hamapping.MethodPresetMode:
		// fan/services.yaml:2-14 set_preset_mode, preset_mode.
		if payload != "" {
			return call("set_preset_mode", map[string]any{"preset_mode": payload})
		}
	case domain == "fan" && cmd.Method == hamapping.MethodOscillation:
		// fan/services.yaml:63-73 oscillate, oscillating (boolean).
		if on, ok := catalogPayload(cmd.Entity, "payload_oscillation_on"); ok && payload == on {
			return call("oscillate", map[string]any{"oscillating": true})
		}
		if off, ok := catalogPayload(cmd.Entity, "payload_oscillation_off"); ok && payload == off {
			return call("oscillate", map[string]any{"oscillating": false})
		}
	case domain == "fan" && cmd.Method == hamapping.MethodDirection:
		// fan/services.yaml:80-94 set_direction, direction forward|reverse.
		if payload == "forward" || payload == "reverse" {
			return call("set_direction", map[string]any{"direction": payload})
		}
	case domain == "climate" && cmd.Method == hamapping.MethodFanMode:
		// climate/services.py:214-219 set_fan_mode, fan_mode.
		if payload != "" {
			return call("set_fan_mode", map[string]any{"fan_mode": payload})
		}
	case domain == "select" && cmd.Method == hamapping.MethodSelectOption:
		// select/services.py:42-46 select_option, option.
		if payload != "" {
			return call("select_option", map[string]any{"option": payload})
		}
	case domain == "vacuum" && cmd.Method == hamapping.MethodRunMode:
		// Row vacuum/robotic_vacuum_cleaner: run mode 0 Idle, 1 Cleaning;
		// vacuum/services.py:104-109 start. Idle is return_to_base where
		// the vacuum has RETURN_HOME (:116-121), else stop where it has
		// STOP (:144-147); the bits are vacuum/const.py:53-69.
		switch payload {
		case "0":
			features, _ := toInt(attrs["supported_features"])
			switch {
			case features&vacuumReturnHome != 0:
				return call("return_to_base", nil)
			case features&vacuumStop != 0:
				return call("stop", nil)
			}
		case "1":
			return call("start", nil)
		}
	case domain == "water_heater" && cmd.Method == hamapping.MethodOperationMode:
		return waterHeaterMode(id, payload, attrs)
	case domain == "water_heater" && cmd.Method == hamapping.MethodTemperature:
		// water_heater/services.py:34-37,90-95 set_temperature, temperature.
		if f, ok := number(payload); ok {
			return call("set_temperature", map[string]any{"temperature": f})
		}
	case domain == "media_player" && cmd.Method == hamapping.MethodVolumeMute:
		// media_player/services.py:193-203 volume_mute, is_volume_muted.
		if mute, err := strconv.ParseBool(payload); err == nil {
			return call("volume_mute", map[string]any{"is_volume_muted": mute})
		}
	case domain == "media_player" && cmd.Method == hamapping.MethodVolumeLevel:
		// media_player/services.py:182-192 volume_set, volume_level 0 to 1
		// (cv.small_float): the level 0 to 254 over LevelControl's range,
		// level / 254 as go-fabric's UnitFromLevel (cluster/spec/
		// levelcontrol/values.go:21,41-44), to two places.
		if f, ok := number(payload); ok {
			level := min(max(math.Round(f), 0), float64(levelcontroldef.LevelMax))
			v := math.Round(level/float64(levelcontroldef.LevelMax)*100) / 100
			return call("volume_set", map[string]any{"volume_level": v})
		}
	case domain == "media_player" && cmd.Method == hamapping.MethodKey:
		return mediaKey(id, payload, attrs)
	}
	return ha.ServiceCall{}, fmt.Errorf("%w: %s method %q payload %q", ErrCommand, id, cmd.Method, payload)
}

// VacuumEntityFeature.STOP and RETURN_HOME (vacuum/const.py:53-69).
const (
	vacuumStop       = 8
	vacuumReturnHome = 16
)

// waterHeaterOnOff is WaterHeaterEntityFeature.ON_OFF
// (water_heater/const.py:61-67).
const waterHeaterOnOff = 8

// waterHeaterMode is row water_heater/thermostat's SystemMode write,
// "off" or "on" (water_heater/services.py:40-42,96-101 set_operation_mode,
// operation_mode; :78-83 turn_on, turn_off, which need ON_OFF). The
// operation mode must be one of the entity's operation_list
// (water_heater/__init__.py:309-330), so "off" is the operation mode
// "off" where the list has it, else turn_off; "on" is turn_on where the
// heater has ON_OFF, else the first operation mode of the list that is
// not "off".
func waterHeaterMode(id, payload string, attrs map[string]any) (ha.ServiceCall, error) {
	call := func(service string, data map[string]any) (ha.ServiceCall, error) {
		return ha.ServiceCall{Domain: "water_heater", Service: service, EntityID: id, Data: data}, nil
	}
	features, _ := toInt(attrs["supported_features"])
	onOff := features&waterHeaterOnOff != 0
	modes := strings_(attrs["operation_list"])
	switch payload {
	case "off":
		if slices.Contains(modes, "off") {
			return call("set_operation_mode", map[string]any{"operation_mode": "off"})
		}
		if onOff {
			return call("turn_off", nil)
		}
	case "on":
		if onOff {
			return call("turn_on", nil)
		}
		for _, m := range modes {
			if m != "off" {
				return call("set_operation_mode", map[string]any{"operation_mode": m})
			}
		}
	}
	return ha.ServiceCall{}, fmt.Errorf("%w: %s operation mode %q: no operation mode and no ON_OFF", ErrCommand, id, payload)
}

// mediaKey is row media_player/video_player's SendKey with the CEC key
// codes it lets through (go-fabric cluster/spec/keypadinput
// definition_gen.go:95-98): volume up and down (media_player/
// services.py:137-148 volume_up, volume_down), mute as the opposite of
// the current is_volume_muted (:193-203 volume_mute) and power as toggle
// (:131-136).
func mediaKey(id, payload string, attrs map[string]any) (ha.ServiceCall, error) {
	call := func(service string, data map[string]any) (ha.ServiceCall, error) {
		return ha.ServiceCall{Domain: "media_player", Service: service, EntityID: id, Data: data}, nil
	}
	code, err := strconv.ParseUint(payload, 10, 8)
	if err == nil {
		switch keypadinputdef.CecKeyCodeEnum(code) {
		case keypadinputdef.CecKeyCodeVolumeUp:
			return call("volume_up", nil)
		case keypadinputdef.CecKeyCodeVolumeDown:
			return call("volume_down", nil)
		case keypadinputdef.CecKeyCodeMute:
			muted, _ := attrs["is_volume_muted"].(bool)
			return call("volume_mute", map[string]any{"is_volume_muted": !muted})
		case keypadinputdef.CecKeyCodePower:
			return call("toggle", nil)
		default:
			// The row lets no other key through.
		}
	}
	return ha.ServiceCall{}, fmt.Errorf("%w: %s key %q", ErrCommand, id, payload)
}

// light is the JSON light payload of rows light/* (state, brightness,
// color_temp, color.h/.s/.x/.y/.r/.g/.b) as light.turn_off or
// light.turn_on (light/services.yaml:183-294 turn_on: brightness at :259,
// color_temp_kelvin :202, hs_color :249, xy_color :254, rgb_color :197;
// :295 turn_off).
func light(id string, payload []byte) (ha.ServiceCall, error) {
	var obj struct {
		State      string             `json:"state"`
		Brightness *float64           `json:"brightness"`
		ColorTemp  *float64           `json:"color_temp"`
		Color      map[string]float64 `json:"color"`
	}
	if err := json.Unmarshal(payload, &obj); err != nil {
		return ha.ServiceCall{}, fmt.Errorf("%w: %s payload %q: %w", ErrCommand, id, payload, err)
	}
	sc := ha.ServiceCall{Domain: "light", Service: "turn_on", EntityID: id}
	if strings.EqualFold(obj.State, "OFF") {
		sc.Service = "turn_off"
		return sc, nil
	}
	data := map[string]any{}
	if b := obj.Brightness; b != nil {
		data["brightness"] = clampInt(*b, 0, 255)
	}
	if k := obj.ColorTemp; k != nil && *k > 0 {
		// The entity states extra.color_temp_kelvin true, so the row's
		// color_temp is kelvin.
		data["color_temp_kelvin"] = int(math.Round(*k))
	}
	c := obj.Color
	if h, okh := c["h"]; okh {
		if s, oks := c["s"]; oks {
			data["hs_color"] = []float64{h, s}
		}
	}
	if x, okx := c["x"]; okx {
		if y, oky := c["y"]; oky {
			data["xy_color"] = []float64{x, y}
		}
	}
	if r, okr := c["r"]; okr {
		g, b := c["g"], c["b"]
		data["rgb_color"] = []int{clampInt(r, 0, 255), clampInt(g, 0, 255), clampInt(b, 0, 255)}
	}
	if len(data) > 0 {
		sc.Data = data
	}
	return sc, nil
}

// catalogPayload is the payload the row rendered for a catalog key: the
// entity's Extra[key], else the catalog's default for the platform
// (go-hamapping value.go extra/catalogDefault).
func catalogPayload(e model.Entity, key string) (string, bool) {
	if v, ok := e.Desc().Extra[key].(string); ok {
		return v, true
	}
	mqtt, err := hacatalog.LoadMQTT()
	if err != nil {
		return "", false
	}
	schema, ok := mqtt.Platforms[string(e.Platform())]
	if !ok {
		return "", false
	}
	if k, ok := schema.Keys[key]; ok {
		if s, ok := k.Default.(string); ok {
			return s, true
		}
	}
	return "", false
}

func number(s string) (float64, bool) {
	f, err := strconv.ParseFloat(s, 64)
	return f, err == nil && !math.IsNaN(f) && !math.IsInf(f, 0)
}

func integer(s string, lo, hi int) (int, bool) {
	f, ok := number(s)
	if !ok {
		return 0, false
	}
	return clampInt(f, lo, hi), true
}

func clampInt(f float64, lo, hi int) int {
	n := int(math.Round(f))
	return max(lo, min(hi, n))
}
