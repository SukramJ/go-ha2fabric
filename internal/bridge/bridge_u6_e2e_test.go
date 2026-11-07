// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package bridge_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/SukramJ/go-fabric/clock"
	onoffdef "github.com/SukramJ/go-fabric/cluster/spec/onoff"
	"github.com/SukramJ/go-fabric/fabrictest"

	"github.com/SukramJ/go-ha2fabric/internal/bridge"
	"github.com/SukramJ/go-ha2fabric/internal/config"
	"github.com/SukramJ/go-ha2fabric/internal/expose"
	"github.com/SukramJ/go-ha2fabric/internal/ha"
	"github.com/SukramJ/go-ha2fabric/internal/ha/hatest"
	"github.com/SukramJ/go-ha2fabric/internal/plan"
	"github.com/SukramJ/go-ha2fabric/internal/selection"
)

// TestSeveralNodesOneConnection is the daemon's node registry on a
// fabrictest site (issue #36, concept U6 C.3): two nodes, each with its
// bridge, on one Home Assistant connection through ha.Fanout. The second
// joins the running connection, as a node created in the web UI does,
// serves its own selection only — the registry's empty rule — and is
// commissioned with its own values; a state change reaches it through the
// fan-out, and removing its fabric is what Delete does to it.
func TestSeveralNodesOneConnection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	srv := hatest.New(t)
	site := fabrictest.NewSite(t, fabrictest.Options{Clock: clock.System{}, Logger: logger()})
	first := site.AddNode(bridge.NodeConfig(&config.Config{
		Passcode: 20202021, Discriminator: 3840, VendorID: 0xFFF1, ProductID: 0x8001, Label: "go-ha2fabric",
	}, logger()))
	second := site.AddNode(bridge.NodeConfig(&config.Config{
		Passcode: 34567890, Discriminator: 1234, VendorID: 0xFFF1, ProductID: 0x8001, Label: "Alexa",
	}, logger()))

	cl := ha.New(ha.Options{
		URL: srv.URL(), Token: hatest.Token, Logger: logger(),
		Events:     []string{ha.EventStateChanged, ha.EventEntityRegistryUpdated, ha.EventDeviceRegistryUpdated, ha.EventLabelRegistryUpdated},
		MinBackoff: 20 * time.Millisecond, MaxBackoff: 100 * time.Millisecond,
	})
	fan := &ha.Fanout{}
	rctx, stop := context.WithCancel(ctx)
	done := make(chan struct{}, 3)
	t.Cleanup(func() {
		stop()
		for range 3 {
			<-done
		}
	})
	br1 := bridge.New(first.Node(), cl, bridge.Options{Input: func() plan.Input { return plan.Input{Rule: expose.Rule{Assist: true}} }, Debounce: 20 * time.Millisecond, Logger: logger()})
	go func() { _ = br1.Run(rctx); done <- struct{}{} }()
	fan.Add(br1)
	go func() { _ = cl.Run(rctx, fan); done <- struct{}{} }()
	select {
	case <-br1.Synced():
	case <-ctx.Done():
		t.Fatal("no first sync of the configured node")
	}

	sel, err := selection.Open(filepath.Join(t.TempDir(), "selection-alexa.json"), logger())
	if err != nil {
		t.Fatal(err)
	}
	if err := sel.Set(map[string]selection.Entry{"light.kitchen": {Enabled: true}}, nil); err != nil {
		t.Fatal(err)
	}
	br2 := bridge.New(second.Node(), cl, bridge.Options{Input: func() plan.Input { return plan.Effective(expose.Rule{}, sel, false, nil) }, Debounce: 20 * time.Millisecond, Logger: logger()})
	go func() { _ = br2.Run(rctx); done <- struct{}{} }()
	fan.Add(br2)
	select {
	case <-br2.Synced():
	case <-ctx.Done():
		t.Fatal("the node added to the running connection never synced")
	}

	e1 := &env{t: t, ctx: ctx, ha: srv, site: site, dev: first, br: br1, cl: cl}
	e2 := &env{t: t, ctx: ctx, ha: srv, site: site, dev: second, br: br2, cl: cl}
	if _, ok := e1.endpoint("sensor.living_temperature"); !ok {
		t.Fatal("the configured node lost its rule")
	}
	if _, ok := e2.endpoint("sensor.living_temperature"); ok {
		t.Fatal("the registry node serves what its selection does not pick")
	}
	light, ok := e2.endpoint("light.kitchen")
	if !ok {
		t.Fatalf("the registry node does not serve its pick: %+v", br2.Plan())
	}

	e2.commission()
	onOff := func() bool {
		values, err := fabrictest.ReadCluster(ctx, e2.pair, light, onoffdef.ClusterID)
		if err != nil {
			t.Fatal(err)
		}
		st, _ := onoffdef.StateFrom(values)
		return st.OnOff
	}
	if !onOff() {
		t.Fatal("the kitchen light is off on the registry node")
	}
	srv.SetState("light.kitchen", "off", nil)
	e2.eventually("the state change reaches the registry node", func() bool { return !onOff() })

	// Delete's first step: the node leaves its fabrics.
	n := second.Node()
	infos, err := n.FabricInfos(ctx)
	if err != nil || len(infos) != 1 {
		t.Fatalf("fabrics %+v %v", infos, err)
	}
	if err := n.RemoveFabric(ctx, infos[0].FabricIndex); err != nil {
		t.Fatal(err)
	}
	if infos, _ := n.FabricInfos(ctx); len(infos) != 0 {
		t.Fatalf("fabrics after the removal %+v", infos)
	}
	// Off the fan-out, the node hears nothing more; the other one does.
	fan.Remove(br2)
	if fan.Len() != 1 {
		t.Fatalf("fan-out holds %d", fan.Len())
	}
}
