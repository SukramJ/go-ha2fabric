// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package bridge_test

import (
	"testing"

	"github.com/SukramJ/go-fabric/cluster/spec"
	fancontroldef "github.com/SukramJ/go-fabric/cluster/spec/fancontrol"
	levelcontroldef "github.com/SukramJ/go-fabric/cluster/spec/levelcontrol"
	modeselectdef "github.com/SukramJ/go-fabric/cluster/spec/modeselect"
	onoffdef "github.com/SukramJ/go-fabric/cluster/spec/onoff"
	rvcopdef "github.com/SukramJ/go-fabric/cluster/spec/rvcoperationalstate"
	switchdef "github.com/SukramJ/go-fabric/cluster/spec/switchcluster"
	valvedef "github.com/SukramJ/go-fabric/cluster/spec/valveconfigurationandcontrol"
	"github.com/SukramJ/go-fabric/controller/client"
	"github.com/SukramJ/go-fabric/fabrictest"

	"github.com/SukramJ/go-ha2fabric/internal/expose"
	"github.com/SukramJ/go-ha2fabric/internal/ha"
)

// attrsWith is an entity's current attributes in the fake with changes:
// a state_changed carries the whole attribute set, as Home Assistant's
// does (core.py State._as_dict at 2026.10.3).
func (e *env) attrsWith(entityID string, changes map[string]any) map[string]any {
	e.t.Helper()
	st := e.ha.State(entityID)
	attrs, _ := st["attributes"].(map[string]any)
	if attrs == nil {
		e.t.Fatalf("no state for %s", entityID)
	}
	for k, v := range changes {
		attrs[k] = v
	}
	return attrs
}

func (e *env) mustEndpoint(entityID string) uint16 {
	e.t.Helper()
	ep, ok := e.endpoint(entityID)
	if !ok {
		e.t.Fatalf("no endpoint for %s: %+v", entityID, e.br.Plan())
	}
	return ep
}

func (e *env) waitCall(entityID, service string) ha.ServiceCall {
	e.t.Helper()
	call, err := e.ha.WaitCall(e.ctx, func(c ha.ServiceCall) bool { return c.EntityID == entityID && c.Service == service })
	if err != nil {
		e.t.Fatalf("%s %s: %v", entityID, service, err)
	}
	return call
}

// TestNewDomainsEndToEnd drives the domains of go-hamapping v0.3.0's rows
// through a commissioned controller: per domain the mapped attribute is
// read, a state change reaches a subscription, and a controller's
// command becomes the service call of internal/adapt.
func TestNewDomainsEndToEnd(t *testing.T) {
	e := newEnv(t, expose.Rule{Assist: true})
	e.commission()

	sel := e.mustEndpoint("select.hall_scene")
	button := e.mustEndpoint("button.router_restart")
	vacuum := e.mustEndpoint("vacuum.robo")
	valve := e.mustEndpoint("valve.garden")
	speaker := e.mustEndpoint("media_player.kitchen_speaker")
	ac := e.mustEndpoint("climate.bedroom_ac")
	remote := e.mustEndpoint("event.hall_remote")

	// The fixtures' values, read once.
	values, err := fabrictest.ReadCluster(e.ctx, e.pair, sel, modeselectdef.ClusterID)
	if err != nil {
		t.Fatal(err)
	}
	if st, _ := modeselectdef.StateFrom(values); len(st.SupportedModes) != 3 || st.CurrentMode != 1 || st.SupportedModes[1].Label != "Evening" {
		t.Fatalf("select %+v", st)
	}
	values, err = fabrictest.ReadCluster(e.ctx, e.pair, button, onoffdef.ClusterID)
	if err != nil {
		t.Fatal(err)
	}
	if st, _ := onoffdef.StateFrom(values); st.OnOff {
		t.Fatalf("button %+v", st)
	}
	values, err = fabrictest.ReadCluster(e.ctx, e.pair, vacuum, rvcopdef.ClusterID)
	if err != nil {
		t.Fatal(err)
	}
	if st, _ := rvcopdef.StateFrom(values); st.OperationalState != rvcopdef.OperationalStateDocked {
		t.Fatalf("vacuum %+v", st)
	}
	values, err = fabrictest.ReadCluster(e.ctx, e.pair, valve, valvedef.ClusterID)
	if err != nil {
		t.Fatal(err)
	}
	if st, _ := valvedef.StateFrom(values); st.CurrentState == nil || *st.CurrentState != valvedef.ValveStateClosed {
		t.Fatalf("valve %+v", st)
	}
	// volume_level 0.5 is CurrentLevel 127 (go-fabric
	// cluster/spec/levelcontrol/values.go:34-39, LevelFromUnit).
	values, err = fabrictest.ReadCluster(e.ctx, e.pair, speaker, levelcontroldef.ClusterID)
	if err != nil {
		t.Fatal(err)
	}
	if st, _ := levelcontroldef.StateFrom(values); st.CurrentLevel == nil || *st.CurrentLevel != 127 {
		t.Fatalf("speaker %+v", st.CurrentLevel)
	}
	values, err = fabrictest.ReadCluster(e.ctx, e.pair, ac, fancontroldef.ClusterID)
	if err != nil {
		t.Fatal(err)
	}
	if st, _ := fancontroldef.StateFrom(values); st.FanMode != fancontroldef.FanModeAuto {
		t.Fatalf("air conditioner %+v", st)
	}

	// One subscription for every state change below.
	reports := make(chan *fabrictest.Report, 64)
	sub, err := fabrictest.Subscribe(e.ctx, e.pair, fabrictest.SubscribeRequest{
		Attributes: []fabrictest.AttributePath{
			client.AttributeAt(sel, modeselectdef.ClusterID, modeselectdef.AttrCurrentMode),
			client.AttributeAt(vacuum, rvcopdef.ClusterID, rvcopdef.AttrOperationalState),
			client.AttributeAt(valve, valvedef.ClusterID, valvedef.AttrCurrentState),
			client.AttributeAt(speaker, levelcontroldef.ClusterID, levelcontroldef.AttrCurrentLevel),
			client.AttributeAt(ac, fancontroldef.ClusterID, fancontroldef.AttrFanMode),
		},
		Events: []fabrictest.EventPath{
			client.EventAt(remote, switchdef.ClusterID, switchdef.EventInitialPress),
			client.EventAt(remote, switchdef.ClusterID, switchdef.EventShortRelease),
		},
		Updated: func(r *fabrictest.Report) { reports <- r },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	t.Run("select", func(t *testing.T) {
		e.ha.SetState("select.hall_scene", "Movie night", nil)
		e.waitReport(reports, "CurrentMode 2", func(r *fabrictest.Report) bool {
			_, ok := r.Value(sel, modeselectdef.ClusterID, modeselectdef.AttrCurrentMode)
			st, _ := modeselectdef.StateFrom(r.Values(sel, modeselectdef.ClusterID))
			return ok && st.CurrentMode == 2
		})
		if _, err := fabrictest.Invoke(e.ctx, e.pair, client.CommandAt(sel, modeselectdef.ClusterID, modeselectdef.CmdChangeToMode), modeselectdef.ChangeToModeRequest{NewMode: 0}); err != nil {
			t.Fatal(err)
		}
		if call := e.waitCall("select.hall_scene", "select_option"); call.Data["option"] != "Day" {
			t.Fatalf("select_option %+v", call)
		}
	})

	t.Run("button", func(t *testing.T) {
		if _, err := fabrictest.Invoke(e.ctx, e.pair, client.CommandAt(button, onoffdef.ClusterID, onoffdef.CmdOn), nil); err != nil {
			t.Fatal(err)
		}
		if call := e.waitCall("button.router_restart", "press"); call.Domain != "button" || len(call.Data) != 0 {
			t.Fatalf("press %+v", call)
		}
	})

	t.Run("vacuum", func(t *testing.T) {
		e.ha.SetState("vacuum.robo", "cleaning", nil)
		e.waitReport(reports, "OperationalState Running", func(r *fabrictest.Report) bool {
			_, ok := r.Value(vacuum, rvcopdef.ClusterID, rvcopdef.AttrOperationalState)
			st, _ := rvcopdef.StateFrom(r.Values(vacuum, rvcopdef.ClusterID))
			return ok && st.OperationalState == rvcopdef.OperationalStateRunning
		})
		if _, err := fabrictest.Invoke(e.ctx, e.pair, client.CommandAt(vacuum, rvcopdef.ClusterID, rvcopdef.CmdPause), nil); err != nil {
			t.Fatal(err)
		}
		if call := e.waitCall("vacuum.robo", "pause"); call.Domain != "vacuum" {
			t.Fatalf("pause %+v", call)
		}
	})

	t.Run("valve", func(t *testing.T) {
		e.ha.SetState("valve.garden", "open", e.attrsWith("valve.garden", map[string]any{"is_closed": false}))
		e.waitReport(reports, "CurrentState Open", func(r *fabrictest.Report) bool {
			_, ok := r.Value(valve, valvedef.ClusterID, valvedef.AttrCurrentState)
			st, _ := valvedef.StateFrom(r.Values(valve, valvedef.ClusterID))
			return ok && st.CurrentState != nil && *st.CurrentState == valvedef.ValveStateOpen
		})
		if _, err := fabrictest.Invoke(e.ctx, e.pair, client.CommandAt(valve, valvedef.ClusterID, valvedef.CmdClose), nil); err != nil {
			t.Fatal(err)
		}
		if call := e.waitCall("valve.garden", "close_valve"); call.Domain != "valve" {
			t.Fatalf("close_valve %+v", call)
		}
	})

	t.Run("speaker", func(t *testing.T) {
		// volume_level 0.2 is CurrentLevel round(50.8) = 51.
		e.ha.SetState("media_player.kitchen_speaker", "playing", e.attrsWith("media_player.kitchen_speaker", map[string]any{"volume_level": 0.2}))
		e.waitReport(reports, "CurrentLevel 51", func(r *fabrictest.Report) bool {
			_, ok := r.Value(speaker, levelcontroldef.ClusterID, levelcontroldef.AttrCurrentLevel)
			st, _ := levelcontroldef.StateFrom(r.Values(speaker, levelcontroldef.ClusterID))
			return ok && st.CurrentLevel != nil && *st.CurrentLevel == 51
		})
		if _, err := fabrictest.Invoke(e.ctx, e.pair, client.CommandAt(speaker, levelcontroldef.ClusterID, levelcontroldef.CmdMoveToLevel), levelcontroldef.MoveToLevelRequest{Level: 127}); err != nil {
			t.Fatal(err)
		}
		call, err := e.ha.WaitCall(e.ctx, func(c ha.ServiceCall) bool {
			return c.EntityID == "media_player.kitchen_speaker" && c.Service == "volume_set" && c.Data["volume_level"] == 0.5
		})
		if err != nil {
			t.Fatalf("volume_set %+v %v", call, err)
		}
	})

	t.Run("air conditioner", func(t *testing.T) {
		e.ha.SetState("climate.bedroom_ac", "cool", e.attrsWith("climate.bedroom_ac", map[string]any{"fan_mode": "low"}))
		e.waitReport(reports, "FanMode Low", func(r *fabrictest.Report) bool {
			_, ok := r.Value(ac, fancontroldef.ClusterID, fancontroldef.AttrFanMode)
			st, _ := fancontroldef.StateFrom(r.Values(ac, fancontroldef.ClusterID))
			return ok && st.FanMode == fancontroldef.FanModeLow
		})
		if err := fabrictest.Write(e.ctx, e.pair, client.AttributeAt(ac, fancontroldef.ClusterID, fancontroldef.AttrFanMode), spec.Uint8(uint8(fancontroldef.FanModeHigh))); err != nil {
			t.Fatal(err)
		}
		if call := e.waitCall("climate.bedroom_ac", "set_fan_mode"); call.Data["fan_mode"] != "high" {
			t.Fatalf("set_fan_mode %+v", call)
		}
	})

	t.Run("event", func(t *testing.T) {
		// A new timestamp is a press (event/__init__.py:153-168,187-194):
		// go-hamapping's event/generic_switch emits InitialPress, then
		// ShortRelease.
		e.ha.SetState("event.hall_remote", "2026-10-31T12:00:00.000+00:00", nil)
		var initial, release bool
		e.waitReport(reports, "InitialPress and ShortRelease", func(r *fabrictest.Report) bool {
			for _, ev := range r.Events {
				if ev.Path.Endpoint != remote || ev.Path.Cluster != switchdef.ClusterID {
					continue
				}
				switch ev.Path.Event {
				case switchdef.EventInitialPress:
					initial = true
				case switchdef.EventShortRelease:
					release = initial
				}
			}
			return initial && release
		})
	})

	if s := e.br.Status(); s.CommandErrs != 0 {
		t.Fatalf("status %+v", s)
	}
}
