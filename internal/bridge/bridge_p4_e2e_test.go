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
// that applies the Replace hands the key to the new plan.
func TestRefusedReplaceKeepsServedEndpointLive(t *testing.T) {
	e := newEnv(t, expose.Rule{Assist: true})
	const id = "binary_sensor.front_door"
	key := "ha:dev-zha-2/" + e.regID(id)
	e.eventually("served", func() bool { return e.served(key) != nil })
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

	// The part key free again: the retry replaces, and the new device
	// follows the state.
	if err := e.dev.Act(func(n *node.Node) error { return n.Aggregator().Remove(e.ctx, device.StringKey("foreign")) }); err != nil {
		t.Fatal(err)
	}
	if err := e.br.Resync(e.ctx); err != nil {
		t.Fatalf("Resync after the part key is free: %v", err)
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
