// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package bridge_test

import (
	"context"
	"strings"
	"testing"

	hamapping "github.com/SukramJ/go-hamapping"
	"github.com/SukramJ/go-hamqtt/model"
	onoffdef "thingfabric.dev/go-fabric/cluster/spec/onoff"
	"thingfabric.dev/go-fabric/controller/client"
	"thingfabric.dev/go-fabric/fabrictest"

	"github.com/SukramJ/go-ha2fabric/internal/adapt"
	"github.com/SukramJ/go-ha2fabric/internal/bridge"
	"github.com/SukramJ/go-ha2fabric/internal/expose"
	"github.com/SukramJ/go-ha2fabric/internal/ha"
)

// TestOversizedStateTakesTheEndpoint (HR-P2-bound-90, decision 52): a
// served entity whose state_changed comes over ha.MaxStateBytes loses
// its endpoint instead of being re-typed from its stripped state; a
// state under the cap again brings it back on its stored number.
func TestOversizedStateTakesTheEndpoint(t *testing.T) {
	e := newEnv(t, expose.Rule{Assist: true})
	ep := e.mustEndpoint("light.kitchen")
	attrs := e.attrsWith("light.kitchen", nil)
	big := e.attrsWith("light.kitchen", map[string]any{"blob": strings.Repeat("x", ha.MaxStateBytes)})
	e.ha.SetState("light.kitchen", "on", big)
	e.eventually("the oversized light's endpoint is gone", func() bool {
		_, ok := e.endpoint("light.kitchen")
		return !ok
	})
	if v := e.verdict("light.kitchen"); v.State != "unmappable" || !strings.HasPrefix(v.Reason, "state over 256 KiB") {
		t.Fatalf("light.kitchen: %s: %q", v.State, v.Reason)
	}
	delete(attrs, "blob")
	e.ha.SetState("light.kitchen", "on", attrs)
	e.eventually("the light back under the cap", func() bool {
		got, ok := e.endpoint("light.kitchen")
		return ok && got == ep
	})
}

// TestRefusedEntityIsNotServed (HR-P2-data-91): an entity go-fabric
// refused while its device's siblings were built (data-35) has no
// endpoint, so it is not served: a command for it queued before the
// rebuild is dropped by the HR-P1-conc-33 guard and makes no call.
func TestRefusedEntityIsNotServed(t *testing.T) {
	e := newEnv(t, expose.Rule{Assist: true})
	e.ha.UpdateEntity("climate.hall", map[string]any{"device_id": "dev-shelly-1"})
	e.ha.SetState("climate.hall", "heat", map[string]any{"hvac_modes": []any{"off", "heat"}, "min_temp": 50, "temperature": 55, "supported_features": 1, "friendly_name": "Hall"})
	e.eventually("the thermostat refused", func() bool {
		v := e.verdict("climate.hall")
		return strings.HasPrefix(v.Reason, "go-fabric refused")
	})
	if !bridge.Serving(e.br, "switch.coffee") {
		t.Fatal("switch.coffee is not served")
	}
	if bridge.Serving(e.br, "climate.hall") {
		t.Fatal("the refused climate.hall counts as served")
	}
	st := &ha.State{EntityID: "climate.hall", State: "heat", Attributes: map[string]any{}}
	ent := adapt.Entity(adapt.Input{EntityID: "climate.hall", State: st, Device: adapt.OwnDevice("climate.hall", "Hall"), TemperatureUnit: "°C"})
	errs, mark := e.br.Status().CommandErrs, len(e.ha.Calls())
	bridge.RunCommand(e.ctx, e.br, model.Command{Entity: ent, Method: hamapping.MethodTemperature, Payload: []byte("21")})
	e.noCall(mark, "climate.hall")
	if got := e.br.Status().CommandErrs; got != errs+1 {
		t.Fatalf("command errors %d, want %d", got, errs+1)
	}
}

// holdFirst holds the first call_service for entity until release is
// closed, whatever its context: the command loop waits on it, and no
// failed call restores the state meanwhile.
type holdFirst struct {
	bridge.Upstream
	entity  string
	held    chan struct{}
	release chan struct{}
}

func (h *holdFirst) CallService(ctx context.Context, sc ha.ServiceCall) error {
	if sc.EntityID == h.entity {
		select {
		case h.held <- struct{}{}:
			<-h.release
		default:
		}
	}
	return h.Upstream.CallService(ctx, sc)
}

// TestDroppedCommandAttributeBack (HR-P2-test-95, HR-P2-err-63): a
// controller's On that the full command queue drops does not leave the
// plug's OnOff at the controller's on: it reads Home Assistant's off
// again.
func TestDroppedCommandAttributeBack(t *testing.T) {
	h := &holdFirst{entity: "switch.coffee", held: make(chan struct{}, 1), release: make(chan struct{})}
	e := newEnv(t, expose.Rule{Assist: true}, func(s *setup) {
		s.wrap = func(up bridge.Upstream) bridge.Upstream { h.Upstream = up; return h }
	})
	t.Cleanup(func() { close(h.release) })
	e.commission()
	ep := e.mustEndpoint("switch.coffee")
	if e.onOff(ep) {
		t.Fatal("switch.coffee reads on before the commands")
	}
	invoke := func(cmd uint32) error {
		_, err := fabrictest.Invoke(e.ctx, e.pair, client.CommandAt(ep, onoffdef.ClusterID, cmd), nil)
		return err
	}
	if err := invoke(onoffdef.CmdOn); err != nil {
		t.Fatal(err)
	}
	<-h.held // the command loop waits on the first call
	// The queue full behind it: commands of an entity no longer served,
	// dropped once the loop runs again.
	gone := adapt.Entity(adapt.Input{EntityID: "switch.gone", Device: adapt.OwnDevice("switch.gone", "Gone")})
	bridge.FillCommands(e.br, model.Command{Entity: gone, Method: hamapping.MethodTurnOn})
	// Off, then On (an On on an on plug is no command): both dropped.
	errs := e.br.Status().CommandErrs
	for _, cmd := range []uint32{onoffdef.CmdOff, onoffdef.CmdOn} {
		if err := invoke(cmd); err != nil {
			t.Fatal(err)
		}
	}
	if st := e.br.Status(); st.CommandErrs != errs+2 || st.LastCommand != "queue full" {
		t.Fatalf("the commands were not dropped: %+v", st)
	}
	e.eventually("OnOff back at Home Assistant's off", func() bool { return !e.onOff(ep) })
}
