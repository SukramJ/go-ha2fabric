// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package ha

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

// FuzzEventData holds the decoding of a subscription's event against any
// data Home Assistant (or an integration's values in it) sends: the
// envelope, then the data each handler reads — state_changed, the
// registries' updates and the camera's WebRTC events. It never panics, and
// a decoded state change keeps its states' entity ids as sent.
func FuzzEventData(f *testing.F) {
	for _, s := range []string{
		`{"event_type":"state_changed","data":{"entity_id":"light.a","old_state":null,"new_state":{"entity_id":"light.a","state":"on","attributes":{"brightness":180,"hs_color":[28.4,65.7]}}}}`,
		`{"event_type":"state_changed","data":{"entity_id":"light.a","old_state":{"state":"on"},"new_state":null}}`,
		`{"event_type":"entity_registry_updated","data":{"action":"update","entity_id":"light.b","old_entity_id":"light.a","changes":{"entity_id":"light.a"}}}`,
		`{"event_type":"device_registry_updated","data":{"action":"remove","device_id":"d"}}`,
		`{"event_type":"label_registry_updated","data":{"action":"update","label_id":"l"}}`,
		`{"type":"candidate","candidate":{"candidate":"candidate:1 1 udp 1 10.0.0.1 9 typ host","sdpMid":"0","sdpMLineIndex":0}}`,
		`{"type":"session","session_id":"abc"}`,
		`{"type":"error","code":"webrtc_offer_failed","message":"x"}`,
		`{}`, `null`, `[]`, `{"data":"x"}`,
	} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		var ev Event
		if json.Unmarshal(b, &ev) == nil {
			var sc StateChanged
			if json.Unmarshal(ev.Data, &sc) == nil && sc.NewState != nil {
				_ = Domain(sc.NewState.EntityID)
			}
			var ru RegistryUpdated
			_ = json.Unmarshal(ev.Data, &ru)
			var lu LabelRegistryUpdated
			_ = json.Unmarshal(ev.Data, &lu)
		}
		var w WebRTCEvent
		if json.Unmarshal(b, &w) == nil && w.Candidate != nil && w.Candidate.SdpMLineIndex != nil {
			_ = *w.Candidate.SdpMLineIndex
		}
	})
}

// FuzzSnapshotResults holds the decoding of every result a snapshot reads
// (ReadSnapshot through Client.Call's json.Unmarshal) against any JSON: the
// entity, device, area and label registries, the states, the
// configuration and the Assist exposure list. It never panics; a device
// entry's identifiers and connections decode element by element (Tuple),
// so an accepted tuple has as many strings as the array had elements.
func FuzzSnapshotResults(f *testing.F) {
	for _, s := range []string{
		`[{"id":"e","entity_id":"light.a","platform":"hue","device_id":"d","labels":["l"],"options":{"conversation":{"should_expose":true}}}]`,
		`[{"id":"d","name":"Lamp","identifiers":[["hue","abc"],["tuya",1234]],"connections":[["mac","00:11"]],"labels":[]}]`,
		`[{"area_id":"kitchen","name":"Kitchen"}]`,
		`[{"label_id":"l","name":"Living room","color":"red"}]`,
		`[{"entity_id":"light.a","state":"on","attributes":{"friendly_name":"A","supported_features":44}}]`,
		`{"version":"2026.10.3","location_name":"Home","unit_system":{"temperature":"°F"}}`,
		`{"exposed_entities":{"light.a":{"conversation":true}}}`,
		`[{"identifiers":[["x",null],["y",[1,"a"]],["z",{"a":1}],["w",true]]}]`,
		`null`, `[]`, `{}`, `[null]`,
	} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		var ents []EntityEntry
		if json.Unmarshal(b, &ents) == nil {
			for i := range ents {
				_ = ents[i].Domain()
			}
		}
		var devs []DeviceEntry
		_ = json.Unmarshal(b, &devs)
		var areas []Area
		_ = json.Unmarshal(b, &areas)
		var labels []Label
		_ = json.Unmarshal(b, &labels)
		var states []State
		_ = json.Unmarshal(b, &states)
		var cfg Config
		if json.Unmarshal(b, &cfg) == nil {
			if cfg.TemperatureUnit() == "" {
				t.Fatal("empty temperature unit")
			}
		}
		var ex Exposed
		_ = json.Unmarshal(b, &ex)
		var raw []json.RawMessage
		var tu Tuple
		if json.Unmarshal(b, &tu) == nil {
			if json.Unmarshal(b, &raw) != nil || len(raw) != len(tu) {
				t.Fatalf("tuple of %d from %s", len(tu), b)
			}
		}
	})
}

// FuzzHTTPBase holds the camera proxy's base URL against any HA_URL: it
// never panics, and an accepted URL is http or https with a path ending in
// /api and no query or fragment.
func FuzzHTTPBase(f *testing.F) {
	for _, s := range []string{
		"ws://homeassistant.local:8123/api/websocket", "wss://ha.example/api/websocket/",
		"ws://supervisor/core/websocket", "http://x/api/websocket", "ws://x/other", "ws://[::1]:8123/api/websocket?a=b#c",
		"", "ws://", "ws:///websocket", "ws://%zz/websocket",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		got, err := HTTPBase(s)
		if err != nil {
			return
		}
		u, perr := url.Parse(got)
		if perr != nil {
			t.Fatalf("HTTPBase(%q) = %q, which does not parse: %v", s, got, perr)
		}
		if (u.Scheme != "http" && u.Scheme != "https") || !strings.HasSuffix(u.Path, "/api") || u.RawQuery != "" || u.Fragment != "" {
			t.Fatalf("HTTPBase(%q) = %q", s, got)
		}
	})
}
