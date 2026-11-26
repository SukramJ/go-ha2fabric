// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package bridge_test

import (
	"testing"

	"thingfabric.dev/go-fabric/device"
	"thingfabric.dev/go-fabric/node"

	"thingfabric.dev/go-ha2fabric/internal/bridge"
	"thingfabric.dev/go-ha2fabric/internal/expose"
)

// TestRefusedReplaceKeepsServedEndpointLive (decision 73, HR-P4-err-40):
// while go-fabric refuses a key's Replace the node still serves the old
// device under it, so the old plan stays live for exactly that key: the
// served endpoint follows Home Assistant's state and reachability, the
// entity counts as served (a controller's command is not refused as
// unserved), and the read model names the served endpoint. A later batch
// that applies the Replace hands the key to the new plan. The plan kept
// stale stays open — Close would drop its command hooks (HR-P4-test-91)
// — until then, and the refused build's own plan is closed.
func TestRefusedReplaceKeepsServedEndpointLive(t *testing.T) {
	closes := bridge.PlanCloses(t)
	e := newEnv(t, expose.Rule{Assist: true})
	const id = "binary_sensor.front_door"
	key := "ha:dev-zha-2/" + e.regID(id)
	e.eventually("served", func() bool { return e.served(key) != nil })
	p0, _ := bridge.Plans(e.br, id)
	host, err := device.NewBridgedNode(device.Options{StableKey: device.StringKey("foreign"), Name: "foreign"})
	if err != nil {
		t.Fatal(err)
	}
	part, err := device.NewOnOffPlugInUnit(device.Options{StableKey: device.StringKey(key + "#1"), Name: "foreign part"})
	if err != nil {
		t.Fatal(err)
	}
	if err := host.AddPart(part.Device); err != nil {
		t.Fatal(err)
	}
	if err := e.dev.Act(func(n *node.Node) error { return n.Aggregator().Add(e.ctx, host) }); err != nil {
		t.Fatal(err)
	}
	smoke := map[string]any{"device_class": "smoke", "friendly_name": "Front door contact Door"}
	e.ha.SetState(id, "off", smoke)
	if err := e.br.Resync(e.ctx); err == nil {
		t.Fatal("Replace not refused")
	}
	old := e.served(key).(*device.Device)
	if !old.Reachable() {
		t.Fatal("precondition: old endpoint unreachable")
	}
	p1, live := bridge.Plans(e.br, id)
	if p0 == nil || p1 == p0 || live != p0 || closes(p0) != 0 {
		t.Fatalf("after the refusal: own %p, live %p, served %p closed %d times", p1, live, p0, closes(p0))
	}
	n, _ := old.EndpointNumber()
	if ep, ok := e.endpoint(id); !ok || ep != n {
		t.Fatalf("the read model names endpoint %d (%v), the node serves %d", ep, ok, n)
	}
	if !bridge.Serving(e.br, id) {
		t.Fatal("the served entity counts as unserved: its commands would be refused")
	}
	e.ha.SetState(id, "unavailable", smoke)
	e.eventually("the served endpoint follows the state", func() bool { return !old.Reachable() })
	e.ha.SetState(id, "off", smoke)
	e.eventually("the served endpoint follows the state back", func() bool { return old.Reachable() })
	// Home Assistant gone: the served endpoint is unreachable too.
	e.ha.Down()
	e.eventually("the served endpoint is unreachable while Home Assistant is gone", func() bool { return !old.Reachable() })
	e.ha.Up()
	e.eventually("the served endpoint is reachable again", func() bool { return old.Reachable() })

	if _, live := bridge.Plans(e.br, id); live != p0 || closes(p0) != 0 {
		t.Fatalf("after the retries: live %p, served %p closed %d times", live, p0, closes(p0))
	}
	// The part key free again: the retry replaces, and the new device
	// follows the state.
	if err := e.dev.Act(func(n *node.Node) error { return n.Aggregator().Remove(e.ctx, device.StringKey("foreign")) }); err != nil {
		t.Fatal(err)
	}
	if err := e.br.Resync(e.ctx); err != nil {
		t.Fatalf("Resync after the part key is free: %v", err)
	}
	if own, live := bridge.Plans(e.br, id); live != own || closes(p0) != 1 || closes(p1) != 1 {
		t.Fatalf("after the replace: own %p, live %p; the stale plan closed %d times, the refused build's %d", own, live, closes(p0), closes(p1))
	}
	cur := e.served(key).(*device.Device)
	if cur == old {
		t.Fatal("the old device is still served")
	}
	e.ha.SetState(id, "unavailable", smoke)
	e.eventually("the replacement follows the state", func() bool { return !cur.Reachable() })
	if !old.Reachable() {
		t.Fatal("a state reached the replaced device: the stale plan is still applied")
	}
}
