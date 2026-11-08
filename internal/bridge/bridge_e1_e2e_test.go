// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package bridge_test

import (
	"context"
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
)

// TestEraseBringsAFreshNode is the daemon's Erase on a fabrictest site
// (issue #45, brief E1 §3): the bridge leaves the fan-out and its worker
// ends, the node is reset and a new one starts on the erased store
// (go-fabric fabrictest.Device.FactoryReset at v0.18.0, the pattern
// cmd/ha2fabric's served.erase follows), and a new bridge on it joins the
// running connection. The new node holds no fabric, its window is open,
// it serves the same entities, and a new controller commissions it and
// reads them. Whether the endpoint numbers coincide is measured and
// logged, not asserted: the reset erases the endpoint records, so the
// numbers come from a fresh allocation.
func TestEraseBringsAFreshNode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	srv := hatest.New(t)
	site := fabrictest.NewSite(t, fabrictest.Options{Clock: clock.System{}, Logger: logger()})
	dev := site.AddNode((&config.Config{
		Passcode: 20202021, Discriminator: 3840, VendorID: 0xFFF1, ProductID: 0x8000, Label: "go-ha2fabric",
	}).Node(logger()))
	cl := ha.New(ha.Options{
		URL: srv.URL(), Token: hatest.Token, Logger: logger(),
		Events:     []string{ha.EventStateChanged, ha.EventEntityRegistryUpdated, ha.EventDeviceRegistryUpdated, ha.EventLabelRegistryUpdated},
		MinBackoff: 20 * time.Millisecond, MaxBackoff: 100 * time.Millisecond,
	})
	fan := &ha.Fanout{}
	in := func() plan.Input { return plan.Input{Rule: expose.Rule{Assist: true}} }
	opts := bridge.Options{Input: in, Debounce: 20 * time.Millisecond, Logger: logger()}
	rctx, stop := context.WithCancel(ctx)
	clDone := make(chan struct{})
	t.Cleanup(func() {
		stop()
		<-clDone
	})

	run := func(br *bridge.Bridge) (context.CancelFunc, chan struct{}) {
		bctx, bcancel := context.WithCancel(rctx)
		done := make(chan struct{})
		go func() {
			defer close(done)
			_ = br.Run(bctx)
		}()
		t.Cleanup(func() {
			bcancel()
			<-done
		})
		return bcancel, done
	}
	br1 := bridge.New(dev.Node(), cl, opts)
	cancel1, done1 := run(br1)
	fan.Add(br1)
	go func() {
		defer close(clDone)
		_ = cl.Run(rctx, fan)
	}()
	select {
	case <-br1.Synced():
	case <-ctx.Done():
		t.Fatal("no first sync")
	}
	e := &env{t: t, ctx: ctx, ha: srv, site: site, dev: dev, br: br1, cl: cl}
	e.commission()
	before := map[string]uint16{}
	for _, id := range []string{"light.kitchen", "sensor.living_temperature"} {
		ep, ok := e.endpoint(id)
		if !ok {
			t.Fatalf("no endpoint for %s before the erase: %+v", id, br1.Plan())
		}
		before[id] = ep
	}
	high := func(br *bridge.Bridge) (lo, hi uint16) {
		vs := br.Plan()
		for i := range vs {
			if ep := vs[i].Endpoint; ep != 0 {
				if lo == 0 || ep < lo {
					lo = ep
				}
				hi = max(hi, ep)
			}
		}
		return lo, hi
	}
	lo1, hi1 := high(br1)
	if infos, err := dev.Node().FabricInfos(ctx); err != nil || len(infos) != 1 {
		t.Fatalf("fabrics before the erase %+v %v", infos, err)
	}
	old := dev.Node()

	// The erase, as served.erase: detach the bridge, end its worker,
	// reset; then a new bridge on the new node joins the connection.
	fan.Remove(br1)
	cancel1()
	<-done1
	if err := dev.FactoryReset(ctx); err != nil {
		t.Fatalf("factory reset: %v", err)
	}
	if dev.Node() == old {
		t.Fatal("the device kept its node across the reset")
	}
	br2 := bridge.New(dev.Node(), cl, opts)
	run(br2)
	fan.Add(br2)
	select {
	case <-br2.Synced():
	case <-ctx.Done():
		t.Fatal("the new node's bridge never synced")
	}

	if infos, err := dev.Node().FabricInfos(ctx); err != nil || len(infos) != 0 {
		t.Fatalf("fabrics after the erase %+v %v", infos, err)
	}
	if win := dev.Node().Window(); !win.Open {
		t.Fatalf("the erased node's window is not open: %+v", win)
	}
	e.br = br2
	after := map[string]uint16{}
	for id := range before {
		ep, ok := e.endpoint(id)
		if !ok {
			t.Fatalf("no endpoint for %s after the erase: %+v", id, br2.Plan())
		}
		after[id] = ep
	}
	// The measurement the brief asks for (E1 §4.1): reported, not a gate.
	same := 0
	for id := range before {
		if before[id] == after[id] {
			same++
		}
	}
	lo2, hi2 := high(br2)
	t.Logf("endpoint numbers across the erase: before %v (range %d-%d), after %v (range %d-%d), %d of %d the same",
		before, lo1, hi1, after, lo2, hi2, same, len(before))

	// A new controller commissions the new node and reads the light.
	e.commission()
	values, err := fabrictest.ReadCluster(ctx, e.pair, after["light.kitchen"], onoffdef.ClusterID)
	if err != nil {
		t.Fatalf("read OnOff on the new node: %v", err)
	}
	if st, _ := onoffdef.StateFrom(values); !st.OnOff {
		t.Fatal("the kitchen light is off on the new node")
	}
	if fan.Len() != 1 {
		t.Fatalf("fan-out holds %d", fan.Len())
	}
}
