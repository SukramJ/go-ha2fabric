// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package ha

import (
	"errors"
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
