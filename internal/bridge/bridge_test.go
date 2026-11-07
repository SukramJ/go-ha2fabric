// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package bridge_test

import (
	"strings"
	"testing"

	onoffdef "github.com/SukramJ/go-fabric/cluster/spec/onoff"
	"github.com/SukramJ/go-fabric/controller/client"
	"github.com/SukramJ/go-fabric/fabrictest"

	"github.com/SukramJ/go-ha2fabric/internal/expose"
	"github.com/SukramJ/go-ha2fabric/internal/ha"
	"github.com/SukramJ/go-ha2fabric/internal/plan"
)

func byID(vs []plan.Verdict) map[string]plan.Verdict {
	out := map[string]plan.Verdict{}
	for i := range vs {
		out[vs[i].EntityID] = vs[i]
	}
	return out
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

	// A new entity the include list names appears and gets an endpoint;
	// a cover without SET_POSITION is a Lift-only WindowCovering, which
	// go-fabric's kit builds at v0.17.0 (report H2, Q1).
	e.ha.SetState("cover.garage", "closed", map[string]any{"device_class": "garage", "supported_features": 3, "friendly_name": "Garage"})
	e.eventually("cover.garage assessed", func() bool { return byID(e.br.Plan())["cover.garage"].Exposed })
	if g := byID(e.br.Plan())["cover.garage"]; g.State != "partially_mappable" || g.Endpoint == 0 {
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
