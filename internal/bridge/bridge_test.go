// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package bridge_test

import (
	"encoding/json"
	"strings"
	"testing"

	onoffdef "github.com/SukramJ/go-fabric/cluster/spec/onoff"
	"github.com/SukramJ/go-fabric/controller/client"
	"github.com/SukramJ/go-fabric/fabrictest"

	"github.com/SukramJ/go-ha2fabric/internal/bridge"
	"github.com/SukramJ/go-ha2fabric/internal/config"
	"github.com/SukramJ/go-ha2fabric/internal/expose"
	"github.com/SukramJ/go-ha2fabric/internal/ha"
	"github.com/SukramJ/go-ha2fabric/internal/ha/hatest"
)

// snapshot is the recorded fixtures as one sync reads them.
func snapshot(t *testing.T) *bridge.Snapshot {
	t.Helper()
	var s bridge.Snapshot
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

func byID(vs []bridge.Verdict) map[string]bridge.Verdict {
	out := map[string]bridge.Verdict{}
	for i := range vs {
		out[vs[i].EntityID] = vs[i]
	}
	return out
}

// TestDryPlan is --dump-plan on the fixtures: every candidate with the
// rule's word and go-hamapping's.
func TestDryPlan(t *testing.T) {
	v := byID(bridge.Dry(snapshot(t), expose.Rule{Assist: true, Include: []string{"sensor.outdoor"}, Exclude: []string{"fan.*"}}, nil))
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
	if got := byID(bridge.Dry(s, expose.Rule{Assist: true}, nil))["light.kitchen"]; got.Exposed || !strings.HasSuffix(got.ExposeReason, "no state yet") {
		t.Errorf("stateless light %+v", got)
	}
}

func TestNodeConfig(t *testing.T) {
	c, err := config.Load([]string{"--store-path", t.TempDir() + "/n.db"}, config.MapEnv(map[string]string{"HA_URL": "ws://a/api/websocket", "HA_TOKEN": "t"}), nil)
	if err != nil {
		t.Fatal(err)
	}
	nc := bridge.NodeConfig(c, nil)
	if err := nc.Validate(); err != nil {
		t.Fatalf("node.Config: %v", err)
	}
	if nc.BasicInformation.NodeLabel != config.DefaultLabel || nc.Network.Listen != config.DefaultListen || nc.Store.Path != c.StorePath || nc.BasicInformation.SoftwareVersion == 0 {
		t.Fatalf("node config %+v", nc)
	}
}

// TestResync: the registry and the states move the topology at run time.
func TestResync(t *testing.T) {
	e := newEnv(t, expose.Rule{Assist: true, Include: []string{"cover.garage", "sensor.legacy_yaml"}})
	e.commission()
	light, _ := e.endpoint("light.kitchen")

	// A capability attribute that changes re-plans the device; the
	// endpoint keeps its number.
	attrs := e.ha.State("light.kitchen")["attributes"].(map[string]any)
	attrs["supported_color_modes"] = []any{"color_temp"}
	e.ha.SetState("light.kitchen", "on", attrs)
	e.eventually("light.kitchen re-planned", func() bool {
		return byID(e.br.Plan())["light.kitchen"].Row == "light/color_temperature"
	})
	if again, _ := e.endpoint("light.kitchen"); again != light {
		t.Fatalf("the light moved from %d to %d", light, again)
	}

	// A new entity the include list names appears; a cover the kit
	// refuses says why.
	e.ha.SetState("cover.garage", "closed", map[string]any{"device_class": "garage", "supported_features": 3, "friendly_name": "Garage"})
	e.eventually("cover.garage assessed", func() bool { return byID(e.br.Plan())["cover.garage"].Exposed })
	if g := byID(e.br.Plan())["cover.garage"]; g.State != "unmappable" || !strings.Contains(g.Reason, "go-fabric refused the device") || g.Endpoint != 0 {
		t.Fatalf("garage %+v", g)
	}

	// A failed call_service is counted, and the entity's state goes back
	// onto its device.
	e.ha.Fail = func(sc ha.ServiceCall) *ha.Error { return &ha.Error{Code: "home_assistant_error", Message: "boom"} }
	if _, err := fabrictest.Invoke(e.ctx, e.pair, client.CommandAt(light, onoffdef.ClusterID, onoffdef.CmdOff), nil); err != nil {
		t.Fatal(err)
	}
	e.eventually("the failure is counted", func() bool { return e.br.Status().CommandErrs == 1 })
	if s := e.br.Status(); !strings.Contains(s.LastCommand, "boom") {
		t.Fatalf("status %+v", s)
	}
	e.eventually("the light is on again", func() bool {
		values, err := fabrictest.ReadCluster(e.ctx, e.pair, light, onoffdef.ClusterID)
		st, _ := onoffdef.StateFrom(values)
		return err == nil && st.OnOff
	})

	// An entity removed from the state machine leaves.
	e.ha.Fail = nil
	if _, ok := e.endpoint("binary_sensor.front_door"); !ok {
		t.Fatal("no front door")
	}
	e.ha.SetExposed("binary_sensor.front_door", false)
	e.eventually("the front door left", func() bool {
		_, ok := e.endpoint("binary_sensor.front_door")
		return !ok
	})
	if st := e.br.Status(); st.Endpoints == 0 || st.Candidates == 0 || st.LastSync.IsZero() {
		t.Fatalf("status %+v", st)
	}
}
