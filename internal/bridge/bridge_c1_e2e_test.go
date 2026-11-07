// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package bridge_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/SukramJ/go-fabric/cluster/spec"
	wcdef "github.com/SukramJ/go-fabric/cluster/spec/windowcovering"
	"github.com/SukramJ/go-fabric/controller/client"
	"github.com/SukramJ/go-fabric/fabrictest"

	"github.com/SukramJ/go-ha2fabric/internal/expose"
	"github.com/SukramJ/go-ha2fabric/internal/ha"
)

// coverInvoke sends a WindowCovering command to ep and returns the mark
// of the calls recorded before it; the invoke must succeed.
func (e *env) coverInvoke(ep uint16, cmd uint32, fields spec.Encodable) int {
	e.t.Helper()
	mark := len(e.ha.Calls())
	if _, err := fabrictest.Invoke(e.ctx, e.pair, client.CommandAt(ep, wcdef.ClusterID, cmd), fields); err != nil {
		e.t.Fatalf("endpoint %d command %#x: %v", ep, cmd, err)
	}
	return mark
}

// callsOn is the calls on entityID recorded after mark.
func (e *env) callsOn(mark int, entityID string) []ha.ServiceCall {
	var out []ha.ServiceCall
	for _, c := range e.ha.Calls()[mark:] {
		if c.EntityID == entityID {
			out = append(out, c)
		}
	}
	return out
}

// nextCall waits for the first call on entityID after mark.
func (e *env) nextCall(mark int, entityID string) ha.ServiceCall {
	e.t.Helper()
	var calls []ha.ServiceCall
	e.eventually("a call on "+entityID, func() bool {
		calls = e.callsOn(mark, entityID)
		return len(calls) > 0
	})
	return calls[0]
}

// noCall holds that no call on entityID arrives after mark within a
// bounded wait.
func (e *env) noCall(mark int, entityID string) {
	e.t.Helper()
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if calls := e.callsOn(mark, entityID); len(calls) != 0 {
			e.t.Fatalf("%s: unexpected %v", entityID, calls)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestCoverCommandGates drives the cover rows of go-hamapping v0.4.4
// (rows/cover.yaml, the commands of window_covering, #42): StopMotion
// calls stop_cover only with STOP, GoToLiftPercentage calls
// set_cover_position only with SET_POSITION, and without it
// LiftPercent100thsValue 0 opens and any other value closes. The
// feature flags are Home Assistant's (cover/const.py:44-54 @2026.10.3).
func TestCoverCommandGates(t *testing.T) {
	e := newEnv(t, expose.Rule{Assist: true})
	e.commission()

	// OPEN|CLOSE (3), no position: Lift without PositionAwareLift.
	scene := e.mustEndpoint("cover.scene_blind")
	values, err := fabrictest.ReadCluster(e.ctx, e.pair, scene, wcdef.ClusterID)
	if err != nil {
		t.Fatal(err)
	}
	node, ok := values.Value(spec.AttrFeatureMap)
	if !ok {
		t.Fatal("no FeatureMap on cover.scene_blind")
	}
	fm, err := spec.DecodeUint[uint32](32)(node)
	if err != nil {
		t.Fatal(err)
	}
	if f := wcdef.Feature(fm); f&wcdef.FeatureLift == 0 || f&wcdef.FeaturePositionAwareLift != 0 {
		t.Fatalf("cover.scene_blind FeatureMap %#x", fm)
	}

	mark := e.coverInvoke(scene, wcdef.CmdGoToLiftPercentage, wcdef.GoToLiftPercentageRequest{LiftPercent100thsValue: 0})
	if c := e.nextCall(mark, "cover.scene_blind"); c.Domain != "cover" || c.Service != "open_cover" {
		t.Fatalf("GoToLiftPercentage 0: %v", c)
	}
	mark = e.coverInvoke(scene, wcdef.CmdGoToLiftPercentage, wcdef.GoToLiftPercentageRequest{LiftPercent100thsValue: 10000})
	if c := e.nextCall(mark, "cover.scene_blind"); c.Domain != "cover" || c.Service != "close_cover" {
		t.Fatalf("GoToLiftPercentage 10000: %v", c)
	}
	// StopMotion without STOP answers success and calls nothing; the
	// UpOrOpen after it is the next call on the entity.
	mark = e.coverInvoke(scene, wcdef.CmdStopMotion, nil)
	e.noCall(mark, "cover.scene_blind")
	if c := e.nextCall(e.coverInvoke(scene, wcdef.CmdUpOrOpen, nil), "cover.scene_blind"); c.Service != "open_cover" {
		t.Fatalf("UpOrOpen: %v", c)
	}
	if calls := e.callsOn(mark, "cover.scene_blind"); len(calls) != 1 {
		t.Fatalf("after StopMotion: %v", calls)
	}

	// OPEN|CLOSE|SET_POSITION (7): a position, no stop. Lift 2500
	// (25 % closed) is position 75.
	pos := e.mustEndpoint("cover.position_blind")
	mark = e.coverInvoke(pos, wcdef.CmdGoToLiftPercentage, wcdef.GoToLiftPercentageRequest{LiftPercent100thsValue: 2500})
	if c := e.nextCall(mark, "cover.position_blind"); c.Service != "set_cover_position" || fmt.Sprint(c.Data["position"]) != "75" {
		t.Fatalf("GoToLiftPercentage 2500: %v %v", c, c.Data)
	}
	mark = e.coverInvoke(pos, wcdef.CmdStopMotion, nil)
	e.noCall(mark, "cover.position_blind")

	// All four (15): StopMotion stays stop_cover.
	living := e.mustEndpoint("cover.living_blind")
	if c := e.nextCall(e.coverInvoke(living, wcdef.CmdStopMotion, nil), "cover.living_blind"); c.Service != "stop_cover" {
		t.Fatalf("StopMotion on cover.living_blind: %v", c)
	}
	if s := e.br.Status(); s.CommandErrs != 0 {
		t.Fatalf("command errors %+v", s)
	}
}
