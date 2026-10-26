// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package bridge_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/SukramJ/go-fabric/clock"
	bridgedbidef "github.com/SukramJ/go-fabric/cluster/spec/bridgeddevicebasicinformation"
	onoffdef "github.com/SukramJ/go-fabric/cluster/spec/onoff"
	temperaturedef "github.com/SukramJ/go-fabric/cluster/spec/temperaturemeasurement"
	"github.com/SukramJ/go-fabric/controller/client"
	"github.com/SukramJ/go-fabric/fabrictest"

	"github.com/SukramJ/go-ha2fabric/internal/bridge"
	"github.com/SukramJ/go-ha2fabric/internal/config"
	"github.com/SukramJ/go-ha2fabric/internal/expose"
	"github.com/SukramJ/go-ha2fabric/internal/ha"
	"github.com/SukramJ/go-ha2fabric/internal/ha/hatest"
)

func logger() *slog.Logger {
	if os.Getenv("HA2FABRIC_TEST_LOG") != "" {
		return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}
	return slog.New(slog.DiscardHandler)
}

// env is the daemon's composition in one process: the fake Home
// Assistant, the HA client, the bridge, and the bridge's node on a
// fabrictest site with a controller. The node is the site's
// (fabrictest.Device.Node), as go-fabric-server's S2 runs its bridge half
// (bridgeapi/harness_test.go): a node.New of the daemon's own cannot join
// a site, whose in-memory network and clock are wired below the facade.
// The site runs on the wall clock, as S2's does: the fake answers over a
// real WebSocket no site counter sees.
type env struct {
	t    *testing.T
	ctx  context.Context
	ha   *hatest.Server
	site *fabrictest.Site
	dev  *fabrictest.Device
	br   *bridge.Bridge
	cl   *ha.Client
	pair *fabrictest.Pair
}

func newEnv(t *testing.T, rule expose.Rule, edit ...func(*bridge.Options)) *env {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	srv := hatest.New(t)
	site := fabrictest.NewSite(t, fabrictest.Options{Clock: clock.System{}, Logger: logger()})
	// The daemon's own node configuration on the site (go-fabric
	// fabrictest.Site.AddNode, v0.12.1): the identity, the commissioning
	// values and the health probe main.go hands node.New; the site puts in
	// its network, clock, advertiser and a store (StorePath is empty).
	dev := site.AddNode(bridge.NodeConfig(&config.Config{
		Passcode: 20202021, Discriminator: 3840, VendorID: 0xFFF1, ProductID: 0x8000, Label: "go-ha2fabric",
	}, logger()))
	cl := ha.New(ha.Options{
		URL: srv.URL(), Token: hatest.Token, Logger: logger(),
		Events:     []string{ha.EventStateChanged, ha.EventEntityRegistryUpdated, ha.EventDeviceRegistryUpdated, ha.EventLabelRegistryUpdated},
		MinBackoff: 20 * time.Millisecond, MaxBackoff: 100 * time.Millisecond,
	})
	opts := bridge.Options{Rule: rule, Debounce: 20 * time.Millisecond, Logger: logger()}
	for _, fn := range edit {
		fn(&opts)
	}
	br := bridge.New(dev.Node(), cl, opts)
	rctx, stop := context.WithCancel(ctx)
	done := make(chan struct{}, 2)
	go func() { _ = br.Run(rctx); done <- struct{}{} }()
	go func() { _ = cl.Run(rctx, br); done <- struct{}{} }()
	t.Cleanup(func() {
		stop()
		<-done
		<-done
	})
	select {
	case <-br.Synced():
	case <-ctx.Done():
		t.Fatal("no first sync")
	}
	return &env{t: t, ctx: ctx, ha: srv, site: site, dev: dev, br: br, cl: cl}
}

func (e *env) commission() {
	e.t.Helper()
	ctrl := e.site.AddController(fabrictest.ControllerOptions{})
	pair, err := ctrl.Commission(e.ctx, e.dev, fabrictest.CommissionOptions{})
	if err != nil {
		e.t.Fatalf("commission: %v", err)
	}
	e.pair = pair
}

func (e *env) endpoint(entityID string) (uint16, bool) {
	plan := e.br.Plan()
	for i := range plan {
		if plan[i].EntityID == entityID && plan[i].Endpoint != 0 {
			return plan[i].Endpoint, true
		}
	}
	return 0, false
}

// regID is an entity's registry entry id in the fixtures.
func (e *env) regID(entityID string) string {
	e.t.Helper()
	var entries []ha.EntityEntry
	if err := json.Unmarshal(hatest.Fixture(e.t, "entity_registry_list.json"), &entries); err != nil {
		e.t.Fatal(err)
	}
	for i := range entries {
		if entries[i].EntityID == entityID {
			return entries[i].ID
		}
	}
	e.t.Fatalf("no registry entry %s", entityID)
	return ""
}

func (e *env) eventually(what string, cond func() bool) {
	e.t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			e.t.Fatalf("never: %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func (e *env) reachable(ep uint16) bool {
	e.t.Helper()
	values, err := fabrictest.ReadCluster(e.ctx, e.pair, ep, bridgedbidef.ClusterID)
	if err != nil {
		e.t.Fatalf("read BridgedDeviceBasicInformation: %v", err)
	}
	st, _ := bridgedbidef.StateFrom(values)
	return st.Reachable
}

func TestEndToEnd(t *testing.T) {
	// The battery is a diagnostic entity Assist does not get by default
	// (exposed_entities.py:304-312 at 2026.10.3); the include list adds it.
	e := newEnv(t, expose.Rule{Assist: true, Include: []string{"sensor.*_battery"}})
	e.commission()

	light, ok := e.endpoint("light.kitchen")
	if !ok {
		t.Fatalf("no endpoint for light.kitchen: %+v", e.br.Plan())
	}
	temp, ok := e.endpoint("sensor.living_temperature")
	if !ok {
		t.Fatal("no endpoint for sensor.living_temperature")
	}
	// The battery rides on the sensor's endpoint (go-hamapping
	// sensor/battery).
	if bat, _ := e.endpoint("sensor.living_battery"); bat != temp {
		t.Fatalf("the battery is on endpoint %d, the sensor on %d", bat, temp)
	}
	// What the rule leaves out has no endpoint.
	for _, id := range []string{"light.cellar", "light.attic", "sensor.processor_use"} {
		if _, ok := e.endpoint(id); ok {
			t.Errorf("%s has an endpoint", id)
		}
	}

	values, err := fabrictest.ReadCluster(e.ctx, e.pair, light, onoffdef.ClusterID)
	if err != nil {
		t.Fatal(err)
	}
	if st, _ := onoffdef.StateFrom(values); !st.OnOff {
		t.Fatalf("the kitchen light is off: %+v", st)
	}
	values, err = fabrictest.ReadCluster(e.ctx, e.pair, temp, temperaturedef.ClusterID)
	if err != nil {
		t.Fatal(err)
	}
	if st, _ := temperaturedef.StateFrom(values); st.MeasuredValue == nil || *st.MeasuredValue != 2150 {
		t.Fatalf("MeasuredValue %+v", st.MeasuredValue)
	}

	// A state_changed reaches the controller's subscription.
	reports := make(chan *fabrictest.Report, 16)
	sub, err := fabrictest.Subscribe(e.ctx, e.pair, fabrictest.SubscribeRequest{
		Attributes: []fabrictest.AttributePath{
			client.AttributeAt(temp, temperaturedef.ClusterID, temperaturedef.AttrMeasuredValue),
			client.AttributeAt(light, bridgedbidef.ClusterID, bridgedbidef.AttrReachable),
		},
		Updated: func(r *fabrictest.Report) { reports <- r },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()
	e.ha.SetState("sensor.living_temperature", "22.25", nil)
	e.waitReport(reports, "MeasuredValue 2225", func(r *fabrictest.Report) bool {
		st, _ := temperaturedef.StateFrom(r.Values(temp, temperaturedef.ClusterID))
		_, ok := r.Value(temp, temperaturedef.ClusterID, temperaturedef.AttrMeasuredValue)
		return ok && st.MeasuredValue != nil && *st.MeasuredValue == 2225
	})

	// A controller's Off and On become light.turn_off and light.turn_on.
	if _, err := fabrictest.Invoke(e.ctx, e.pair, client.CommandAt(light, onoffdef.ClusterID, onoffdef.CmdOff), nil); err != nil {
		t.Fatal(err)
	}
	call, err := e.ha.WaitCall(e.ctx, func(c ha.ServiceCall) bool { return c.EntityID == "light.kitchen" && c.Service == "turn_off" })
	if err != nil || call.Domain != "light" {
		t.Fatalf("turn_off %+v %v", call, err)
	}
	if _, err := fabrictest.Invoke(e.ctx, e.pair, client.CommandAt(light, onoffdef.ClusterID, onoffdef.CmdOn), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.ha.WaitCall(e.ctx, func(c ha.ServiceCall) bool { return c.EntityID == "light.kitchen" && c.Service == "turn_on" }); err != nil {
		t.Fatal(err)
	}

	// A registry disable removes the endpoint.
	plug, ok := e.endpoint("switch.coffee")
	if !ok {
		t.Fatal("no endpoint for switch.coffee")
	}
	// The stable key is the device's registry id and the entity's
	// registry entry id (go-hamapping Options.StableKey).
	if got, ok := e.dev.EndpointOf("ha:dev-shelly-1/" + e.regID("switch.coffee")); !ok || got != plug {
		t.Fatalf("the node serves switch.coffee on %d %v, the plan says %d", got, ok, plug)
	}
	e.ha.UpdateEntity("switch.coffee", map[string]any{"disabled_by": "user"})
	e.eventually("switch.coffee's endpoint is gone", func() bool {
		_, ok := e.endpoint("switch.coffee")
		return !ok
	})
	// A wildcard read of a removed endpoint expands to nothing.
	if values, err := fabrictest.ReadCluster(e.ctx, e.pair, plug, onoffdef.ClusterID); err == nil && len(values) != 0 {
		t.Fatalf("endpoint %d still answers: %v", plug, values)
	}
	if _, ok := e.dev.EndpointOf("ha:dev-shelly-1/" + e.regID("switch.coffee")); ok {
		t.Fatal("the node still serves switch.coffee")
	}
	// The other endpoints keep their numbers.
	if again, _ := e.endpoint("light.kitchen"); again != light {
		t.Fatalf("the light moved from %d to %d", light, again)
	}

	// Home Assistant going away makes every endpoint unreachable; its
	// return makes them reachable again.
	e.ha.Down()
	e.waitReport(reports, "Reachable false", func(r *fabrictest.Report) bool {
		st, _ := bridgedbidef.StateFrom(r.Values(light, bridgedbidef.ClusterID))
		_, ok := r.Value(light, bridgedbidef.ClusterID, bridgedbidef.AttrReachable)
		return ok && !st.Reachable
	})
	e.ha.Up()
	e.eventually("reachable again", func() bool { return e.reachable(light) })
	if s := e.br.Status(); !s.HAConnected || s.Commands < 2 || s.CommandErrs != 0 {
		t.Fatalf("status %+v", s)
	}
}

func (e *env) waitReport(reports <-chan *fabrictest.Report, what string, match func(*fabrictest.Report) bool) {
	e.t.Helper()
	timeout := time.After(20 * time.Second)
	for {
		select {
		case r := <-reports:
			if match(r) {
				return
			}
		case <-timeout:
			e.t.Fatalf("no report: %s", what)
		}
	}
}
