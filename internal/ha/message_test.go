// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package ha

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func TestDecode(t *testing.T) {
	for _, c := range []struct {
		frame string
		n     int
		err   bool
	}{
		{`{"type":"auth_required","ha_version":"2026.10.3"}`, 1, false},
		{`{"id":3,"type":"result","success":true,"result":null}`, 1, false},
		{`[{"id":3,"type":"result","success":true},{"id":2,"type":"event","event":{"event_type":"state_changed","data":{}}}]`, 2, false},
		{`  {"id":1,"type":"pong"}  `, 1, false},
		{``, 0, true},
		{`null`, 0, true},
		{`{"id":1}`, 0, true},
		{`{"id":-1,"type":"result"}`, 0, true},
		{`[{"type":"pong"},{}]`, 0, true},
		{`{"type":`, 0, true},
		{`[`, 0, true},
		{`[]`, 0, true},
	} {
		msgs, err := Decode([]byte(c.frame))
		if c.err {
			if !errors.Is(err, ErrFrame) {
				t.Errorf("Decode(%q) = %v, want ErrFrame", c.frame, err)
			}
			continue
		}
		if err != nil || len(msgs) != c.n {
			t.Errorf("Decode(%q) = %d messages, %v", c.frame, len(msgs), err)
		}
	}
	msgs, _ := Decode([]byte(`{"id":12,"type":"result","success":false,"error":{"code":"invalid_format","message":"bad"}}`))
	if m := msgs[0]; m.Success == nil || *m.Success || m.Error == nil || m.Error.Code != "invalid_format" || m.Error.Error() != "home assistant: invalid_format: bad" {
		t.Fatalf("error result %+v", m)
	}
}

func TestDomainAndUnit(t *testing.T) {
	if Domain("binary_sensor.door") != "binary_sensor" || Domain("weird") != "weird" {
		t.Fatal("Domain")
	}
	e := EntityEntry{EntityID: "light.x"}
	if e.Domain() != "light" {
		t.Fatal("EntityEntry.Domain")
	}
	var nilCfg *Config
	if nilCfg.TemperatureUnit() != "°C" || (&Config{UnitSystem: map[string]string{"temperature": "°F"}}).TemperatureUnit() != "°F" {
		t.Fatal("TemperatureUnit")
	}
	if (ServiceCall{Domain: "light", Service: "turn_on", EntityID: "light.x"}).String() != "light.turn_on light.x" {
		t.Fatal("ServiceCall.String")
	}
}

// FuzzDecode holds the frame decoder against any input: it never panics,
// and what it accepts is a non-empty list of typed messages.
func FuzzDecode(f *testing.F) {
	for _, s := range []string{
		`{"type":"auth_required","ha_version":"2026.10.3"}`,
		`{"id":3,"type":"result","success":true,"result":[{"entity_id":"light.a"}]}`,
		`[{"id":3,"type":"result","success":true},{"id":2,"type":"event","event":{"event_type":"state_changed","data":{"entity_id":"light.a","old_state":null,"new_state":{"state":"on"}}}}]`,
		`{"id":9,"type":"result","success":false,"error":{"code":"not_found","message":"x"}}`,
		`[]`, `{}`, `[{}]`, `"x"`,
	} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		msgs, err := Decode(b)
		if err != nil {
			if !errors.Is(err, ErrFrame) {
				t.Fatalf("error %v is not ErrFrame", err)
			}
			return
		}
		if len(msgs) == 0 {
			t.Fatal("accepted a frame without a message")
		}
		for _, m := range msgs {
			if m.Type == "" || m.ID < 0 {
				t.Fatalf("accepted %+v", m)
			}
		}
	})
}

func TestTupleUnmarshal(t *testing.T) {
	for _, c := range []struct {
		in   string
		want Tuple
		err  bool
	}{
		{`["hue","abc"]`, Tuple{"hue", "abc"}, false},
		{`["tuya", 1234]`, Tuple{"tuya", "1234"}, false},
		{`["x", 1.5]`, Tuple{"x", "1.5"}, false},
		{`["x", true]`, Tuple{"x", "true"}, false},
		{`["x", null]`, Tuple{"x", ""}, false},
		{`["x", [1, "a"]]`, Tuple{"x", `[1,"a"]`}, false},
		{`["x", {"a": 1}]`, Tuple{"x", `{"a":1}`}, false},
		{`["only"]`, Tuple{"only"}, false},
		{`[]`, Tuple{}, false},
		{`"not an array"`, nil, true},
		{`{"a":1}`, nil, true},
	} {
		var got Tuple
		err := json.Unmarshal([]byte(c.in), &got)
		if c.err {
			if err == nil {
				t.Errorf("Unmarshal(%s) = %v, want error", c.in, got)
			}
			continue
		}
		if err != nil || !reflect.DeepEqual(got, c.want) {
			t.Errorf("Unmarshal(%s) = %v, %v; want %v", c.in, got, err, c.want)
		}
	}
	var d DeviceEntry
	if err := json.Unmarshal([]byte(`{"id":"d","identifiers":[["tuya",1234],["tuya","bf"]],"connections":[["mac","00:11"]]}`), &d); err != nil {
		t.Fatal(err)
	}
	if len(d.Identifiers) != 2 || d.Identifiers[0][1] != "1234" || d.Connections[0][1] != "00:11" {
		t.Fatalf("device %+v", d)
	}
}
