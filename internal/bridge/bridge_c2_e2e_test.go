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

// TestActionPlugs drives go-hamapping v0.4.5's rows script/plug,
// scene/plug and automation/plug: each action is an OnOffPlugInUnit
// whose OnOff reads false, whose On calls the action's service (adapt's
// services table) and which reads false again after the call and while
// the entity's own state is on.
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

			// A state of on (a script running, an automation enabled)
			// leaves the plug off.
			e.ha.SetState(c.entity, "on", nil)
			e.staysOff(ep, "after state_changed to on")
		})
	}
}
