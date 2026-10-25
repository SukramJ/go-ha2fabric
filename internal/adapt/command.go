// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package adapt

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	hacatalog "github.com/SukramJ/go-ha-catalog"
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

// Service is the call_service of a controller's command. cur is the
// entity's current state, which a climate range setpoint completes (the
// other bound of the range).
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
	switch {
	case domain == "light" && cmd.Method == "":
		return light(id, cmd.Payload)
	case cmd.Method == "":
		for _, p := range payloads[domain] {
			if want, ok := catalogPayload(cmd.Entity, p.key); ok && payload == want {
				return call(p.service, nil)
			}
		}
	case domain == "cover" && cmd.Method == "set_position":
		// cover/services.yaml:43-56 set_cover_position, position 0 to 100.
		if n, ok := integer(payload, 0, 100); ok {
			return call("set_cover_position", map[string]any{"position": n})
		}
	case domain == "cover" && cmd.Method == "tilt":
		// cover/services.yaml:95-108 set_cover_tilt_position, tilt_position.
		if n, ok := integer(payload, 0, 100); ok {
			return call("set_cover_tilt_position", map[string]any{"tilt_position": n})
		}
	case domain == "climate" && cmd.Method == "mode":
		// climate/services.yaml:92-102 set_hvac_mode, hvac_mode.
		if payload != "" {
			return call("set_hvac_mode", map[string]any{"hvac_mode": payload})
		}
	case domain == "climate" && cmd.Method == "temperature":
		// climate/services.yaml:17-34 set_temperature, temperature.
		if f, ok := number(payload); ok {
			return call("set_temperature", map[string]any{"temperature": f})
		}
	case domain == "climate" && (cmd.Method == "temperature_low" || cmd.Method == "temperature_high"):
		// climate/services.py:49-62: target_temp_low and target_temp_high
		// are Inclusive — both or neither; the other bound is the current.
		f, ok := number(payload)
		if !ok {
			break
		}
		set, other := "target_temp_low", "target_temp_high"
		if cmd.Method == "temperature_high" {
			set, other = other, set
		}
		var attrs map[string]any
		if cur != nil {
			attrs = cur.Attributes
		}
		o, ok := toFloat(attrs[other])
		if !ok {
			return ha.ServiceCall{}, fmt.Errorf("%w: %s %s without a current %s", ErrCommand, id, set, other)
		}
		return call("set_temperature", map[string]any{set: f, other: o})
	case domain == "fan" && cmd.Method == "percentage":
		// fan/services.yaml:16-29 set_percentage, percentage 0 to 100.
		if n, ok := integer(payload, 0, 100); ok {
			return call("set_percentage", map[string]any{"percentage": n})
		}
	case domain == "fan" && cmd.Method == "preset_mode":
		// fan/services.yaml:2-14 set_preset_mode, preset_mode.
		if payload != "" {
			return call("set_preset_mode", map[string]any{"preset_mode": payload})
		}
	case domain == "fan" && cmd.Method == "oscillation":
		// fan/services.yaml:63-73 oscillate, oscillating (boolean).
		if on, ok := catalogPayload(cmd.Entity, "payload_oscillation_on"); ok && payload == on {
			return call("oscillate", map[string]any{"oscillating": true})
		}
		if off, ok := catalogPayload(cmd.Entity, "payload_oscillation_off"); ok && payload == off {
			return call("oscillate", map[string]any{"oscillating": false})
		}
	case domain == "fan" && cmd.Method == "direction":
		// fan/services.yaml:80-94 set_direction, direction forward|reverse.
		if payload == "forward" || payload == "reverse" {
			return call("set_direction", map[string]any{"direction": payload})
		}
	}
	return ha.ServiceCall{}, fmt.Errorf("%w: %s method %q payload %q", ErrCommand, id, cmd.Method, payload)
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
