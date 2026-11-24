// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package bridge_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	hamapping "github.com/SukramJ/go-hamapping"
	"thingfabric.dev/go-fabric/cluster/spec"
	wcdef "thingfabric.dev/go-fabric/cluster/spec/windowcovering"
	"thingfabric.dev/go-fabric/eligibility"
	"thingfabric.dev/go-fabric/fabrictest"

	"thingfabric.dev/go-ha2fabric/internal/camera"
	"thingfabric.dev/go-ha2fabric/internal/expose"
	"thingfabric.dev/go-ha2fabric/internal/ha"
	"thingfabric.dev/go-ha2fabric/internal/plan"
)

// coverAttr reads one uint attribute of ep's WindowCovering.
func (e *env) coverAttr(ep uint16, attr uint32, bits int) uint64 {
	e.t.Helper()
	values, err := fabrictest.ReadCluster(e.ctx, e.pair, ep, wcdef.ClusterID)
	if err != nil {
		e.t.Fatal(err)
	}
	node, ok := values.Value(attr)
	if !ok {
		e.t.Fatalf("endpoint %d: no attribute %#x", ep, attr)
	}
	v, err := spec.DecodeUint[uint64](bits)(node)
	if err != nil {
		e.t.Fatalf("endpoint %d attribute %#x: %v", ep, attr, err)
	}
	return v
}

// TestCoverTilt (HR-P2-adpt-33, decision 47): the fixtures' covers with
// tilt are served — a venetian blind with every feature (255) and a
// tilt-only one (240: OPEN_TILT, CLOSE_TILT, STOP_TILT, SET_TILT_POSITION;
// cover/const.py:44-54 @2026.10.3) — and their commands reach Home
// Assistant: the tilt-only cover's open and close move the tilt, and
// without SET_TILT_POSITION they call open_cover_tilt and
// close_cover_tilt.
func TestCoverTilt(t *testing.T) {
	e := newEnv(t, expose.Rule{Assist: true})
	e.commission()

	blind := e.mustEndpoint("cover.tilt_blind")
	if f := wcdef.Feature(e.coverAttr(blind, spec.AttrFeatureMap, 32)); f&wcdef.FeatureLift == 0 || f&wcdef.FeatureTilt == 0 {
		t.Fatalf("cover.tilt_blind FeatureMap %#x", f)
	}
	if ty := e.coverAttr(blind, wcdef.AttrType, 8); ty != uint64(wcdef.TypeTiltBlindLift) {
		t.Fatalf("cover.tilt_blind Type %d", ty)
	}
	if ep := e.coverAttr(blind, wcdef.AttrEndProductType, 8); ep != uint64(wcdef.EndProductTypeInteriorBlind) {
		t.Fatalf("cover.tilt_blind EndProductType %d", ep)
	}
	mark := e.coverInvoke(blind, wcdef.CmdGoToTiltPercentage, wcdef.GoToTiltPercentageRequest{TiltPercent100thsValue: 2500})
	if c := e.nextCall(mark, "cover.tilt_blind"); c.Service != "set_cover_tilt_position" || fmt.Sprint(c.Data["tilt_position"]) != "75" {
		t.Fatalf("GoToTiltPercentage 2500: %v %v", c, c.Data)
	}

	slats := e.mustEndpoint("cover.tilt_slats")
	mark = e.coverInvoke(slats, wcdef.CmdUpOrOpen, nil)
	if c := e.nextCall(mark, "cover.tilt_slats"); c.Service != "set_cover_tilt_position" || fmt.Sprint(c.Data["tilt_position"]) != "100" {
		t.Fatalf("UpOrOpen on the tilt-only cover: %v %v", c, c.Data)
	}
	mark = e.coverInvoke(slats, wcdef.CmdDownOrClose, nil)
	if c := e.nextCall(mark, "cover.tilt_slats"); c.Service != "set_cover_tilt_position" || fmt.Sprint(c.Data["tilt_position"]) != "0" {
		t.Fatalf("DownOrClose on the tilt-only cover: %v %v", c, c.Data)
	}

	// OPEN_TILT, CLOSE_TILT, STOP_TILT (112): no SET_TILT_POSITION.
	attrs := e.ha.State("cover.tilt_slats")["attributes"].(map[string]any)
	attrs["supported_features"] = 112
	e.ha.SetState("cover.tilt_slats", "open", attrs)
	e.eventually("the tilt-only cover without a tilt position re-planned", func() bool {
		ep, ok := e.endpoint("cover.tilt_slats")
		return ok && wcdef.Feature(e.coverAttr(ep, spec.AttrFeatureMap, 32))&wcdef.FeaturePositionAwareTilt == 0
	})
	slats = e.mustEndpoint("cover.tilt_slats")
	if c := e.nextCall(e.coverInvoke(slats, wcdef.CmdUpOrOpen, nil), "cover.tilt_slats"); c.Service != "open_cover_tilt" || len(c.Data) != 0 {
		t.Fatalf("UpOrOpen without SET_TILT_POSITION: %v %v", c, c.Data)
	}
	if c := e.nextCall(e.coverInvoke(slats, wcdef.CmdDownOrClose, nil), "cover.tilt_slats"); c.Service != "close_cover_tilt" || len(c.Data) != 0 {
		t.Fatalf("DownOrClose without SET_TILT_POSITION: %v %v", c, c.Data)
	}
	if s := e.br.Status(); s.CommandErrs != 0 {
		t.Fatalf("command errors %+v", s)
	}
}

// TestCoverShapesBuild (HR-P2-adpt-33, from X2's probe): every cover
// shape go-hamapping calls mappable builds, by device class and
// supported features.
func TestCoverShapesBuild(t *testing.T) {
	for _, dc := range []any{nil, "blind", "shutter", "curtain", "awning", "garage", "shade", "window", "door", "damper", "gate"} {
		for _, feat := range []int{3, 11, 15, 112, 128, 139, 143, 191, 240, 255} {
			attrs := map[string]any{"supported_features": float64(feat), "current_position": 50.0, "current_tilt_position": 50.0}
			if dc != nil {
				attrs["device_class"] = dc
			}
			snap := &ha.Snapshot{States: []ha.State{{EntityID: "cover.a", State: "open", Attributes: attrs}}}
			in := plan.Input{Rule: expose.Rule{Include: []string{"*"}}, Ports: camera.DryPorts}
			_, groups := plan.Assess(snap, in)
			for _, g := range groups {
				hp, mv := hamapping.Map(g.Device, g.Entities, plan.MapOptions(g.Regs, g.Names, in.Ports))
				_, err := hp.Build(context.Background())
				if err != nil && mv[0].State != eligibility.StateUnmappable {
					t.Errorf("device_class %v features %d: %s and refused: %v", dc, feat, mv[0].State, err)
				}
				hp.Close()
			}
		}
	}
}

// TestSelect256Options (HR-P2-adpt-34, from X2's probe): a select of 256
// options is unmappable in the dry plan (the web UI's preview,
// --dump-plan) as on the node, for the same reason.
func TestSelect256Options(t *testing.T) {
	e := newEnv(t, expose.Rule{Assist: true})
	opts := make([]any, 256)
	for i := range opts {
		opts[i] = fmt.Sprintf("o%d", i)
	}
	e.ha.SetState("select.hall_scene", "o1", map[string]any{"options": opts, "friendly_name": "x"})
	e.eventually("the sync after the options", func() bool {
		for _, v := range e.br.Plan() {
			if v.EntityID == "select.hall_scene" && v.State != "mappable" {
				return true
			}
		}
		return false
	})
	var live, dry string
	for _, v := range e.br.Plan() {
		if v.EntityID == "select.hall_scene" {
			live = v.State + ": " + v.Reason
		}
	}
	if c := e.br.View().All["select.hall_scene"]; c != nil {
		dry = c.State + ": " + c.Reason
	}
	if !strings.HasPrefix(dry, "unmappable") || dry != live {
		t.Errorf("a select with 256 options: dry %q, bridge %q", dry, live)
	}
	if strings.Contains(live, "go-fabric refused") {
		t.Errorf("the node refused what Map called mappable: %s", live)
	}
}

// TestRefusedEntityKeepsItsSiblings (HR-P2-data-35, decision 40, from
// X2's probe): an entity go-fabric refuses no longer takes its device's
// other entities off the bridge. The refused one is a thermostat stating
// only a minimum above go-fabric's default maximum, a shape go-hamapping
// v0.7.0 still calls mappable (go-hamapping #45); it joins the plug's
// device.
func TestRefusedEntityKeepsItsSiblings(t *testing.T) {
	e := newEnv(t, expose.Rule{Assist: true})
	if _, ok := e.endpoint("switch.coffee"); !ok {
		t.Fatal("no endpoint")
	}
	e.ha.UpdateEntity("climate.hall", map[string]any{"device_id": "dev-shelly-1"})
	e.ha.SetState("climate.hall", "heat", map[string]any{"hvac_modes": []any{"off", "heat"}, "min_temp": 50, "temperature": 55, "supported_features": 1, "friendly_name": "Hall"})
	var refused *plan.Verdict
	e.eventually("the thermostat's verdict", func() bool {
		for _, v := range e.br.Plan() {
			if v.EntityID == "climate.hall" && strings.Contains(v.Reason, "refused") {
				refused = &v
				return true
			}
		}
		return false
	})
	if refused.State != "unmappable" || !strings.HasPrefix(refused.Reason, "go-fabric refused") {
		t.Errorf("climate.hall: %s: %s", refused.State, refused.Reason)
	}
	for _, v := range e.br.Plan() {
		if v.EntityID == "switch.coffee" && v.State == "unmappable" {
			t.Errorf("switch.coffee: %s: %s", v.State, v.Reason)
		}
	}
	if _, ok := e.endpoint("switch.coffee"); !ok {
		t.Errorf("switch.coffee lost its endpoint to its device's refused thermostat")
	}
	if _, ok := e.endpoint("climate.hall"); ok {
		t.Errorf("climate.hall has an endpoint")
	}
}
