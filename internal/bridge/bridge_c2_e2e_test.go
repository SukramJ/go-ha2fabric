// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package bridge_test

import (
	"slices"
	"testing"
	"time"

	descriptordef "github.com/SukramJ/go-fabric/cluster/spec/descriptor"
	onoffdef "github.com/SukramJ/go-fabric/cluster/spec/onoff"
	"github.com/SukramJ/go-fabric/controller/client"
	"github.com/SukramJ/go-fabric/device"
	"github.com/SukramJ/go-fabric/fabrictest"

	"github.com/SukramJ/go-ha2fabric/internal/expose"
)

// onOff reads the OnOff attribute of ep.
func (e *env) onOff(ep uint16) bool {
	e.t.Helper()
	values, err := fabrictest.ReadCluster(e.ctx, e.pair, ep, onoffdef.ClusterID)
	if err != nil {
		e.t.Fatalf("endpoint %d OnOff: %v", ep, err)
	}
	st, _ := onoffdef.StateFrom(values)
	return st.OnOff
}

// staysOff holds that ep's OnOff reads false throughout a bounded wait.
func (e *env) staysOff(ep uint16, what string) {
	e.t.Helper()
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if e.onOff(ep) {
			e.t.Fatalf("%s: OnOff reads true", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestActionPlugs drives go-hamapping's rows script/plug, scene/plug and
// automation/plug: each action is an OnOffPlugInUnit whose OnOff reads
// false, whose On and Toggle call the action's service (adapt's services
// table), whose Off is discarded — answered with success, no call, counted
// and logged as bridge.command_discarded (#51) — and which reads false
// again after the call and while the entity's own state is on.
func TestActionPlugs(t *testing.T) {
	e := newEnv(t, expose.Rule{Assist: true})
	e.commission()

	for _, c := range []struct {
		entity, domain, row, service string
	}{
		{"script.markise_einfahren", "script", "script/plug", "turn_on"},
		{"scene.abend", "scene", "scene/plug", "turn_on"},
		{"automation.nachtlicht", "automation", "automation/plug", "trigger"},
	} {
		t.Run(c.row, func(t *testing.T) {
			if v := e.verdict(c.entity); v.Row != c.row || v.DeviceType != uint16(device.OnOffPlugInUnitID) {
				t.Fatalf("verdict %+v", v)
			}
			ep := e.mustEndpoint(c.entity)
			values, err := fabrictest.ReadCluster(e.ctx, e.pair, ep, descriptordef.ClusterID)
			if err != nil {
				t.Fatal(err)
			}
			// The bridged endpoint lists its device type beside
			// BridgedNode.
			st, _ := descriptordef.StateFrom(values)
			if !slices.ContainsFunc(st.DeviceTypeList, func(d descriptordef.DeviceTypeStruct) bool {
				return d.DeviceType == uint32(device.OnOffPlugInUnitID)
			}) {
				t.Fatalf("device types %+v", st.DeviceTypeList)
			}
			if e.onOff(ep) {
				t.Fatal("OnOff reads true before the call")
			}

			if _, err := fabrictest.Invoke(e.ctx, e.pair, client.CommandAt(ep, onoffdef.ClusterID, onoffdef.CmdOn), nil); err != nil {
				t.Fatal(err)
			}
			if call := e.waitCall(c.entity, c.service); call.Domain != c.domain || len(call.Data) != 0 {
				t.Fatalf("%s %+v", c.service, call)
			}
			e.staysOff(ep, "after the call")

			// Toggle from off activates as On does.
			mark := len(e.ha.Calls())
			if _, err := fabrictest.Invoke(e.ctx, e.pair, client.CommandAt(ep, onoffdef.ClusterID, onoffdef.CmdToggle), nil); err != nil {
				t.Fatal(err)
			}
			if call := e.nextCall(mark, c.entity); call.Service != c.service {
				t.Fatalf("Toggle: %+v", call)
			}
			e.staysOff(ep, "after Toggle")

			// Off is discarded: success, no call, counted.
			before := e.br.Status().Discards
			mark = len(e.ha.Calls())
			if _, err := fabrictest.Invoke(e.ctx, e.pair, client.CommandAt(ep, onoffdef.ClusterID, onoffdef.CmdOff), nil); err != nil {
				t.Fatal(err)
			}
			e.noCall(mark, c.entity)
			if got := e.br.Status().Discards; got != before+1 {
				t.Fatalf("discards %d, want %d", got, before+1)
			}
			e.staysOff(ep, "after Off")

			// A state of on (a script running, an automation enabled)
			// leaves the plug off.
			e.ha.SetState(c.entity, "on", nil)
			e.staysOff(ep, "after state_changed to on")
		})
	}
}
