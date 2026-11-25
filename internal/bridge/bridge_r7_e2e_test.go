// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package bridge_test

import (
	"strings"
	"testing"

	"thingfabric.dev/go-fabric/device"
	"thingfabric.dev/go-fabric/node"

	"thingfabric.dev/go-ha2fabric/internal/expose"
)

// TestRefusedForgetIsRetried (R7): since go-fabric v0.26.0 a batch holds
// a Forget of a key another endpoint serves and refuses it at its commit,
// not at the call. The bridge takes what the batch applied from the
// node's set, so the refused Forget is recorded again and applied by a
// later sync once the key is free: the entity, created again with its
// old registry id, gets a fresh number, not its reserved one.
func TestRefusedForgetIsRetried(t *testing.T) {
	e := newEnv(t, expose.Rule{Assist: true})
	key := "ha:dev-hue-1/" + e.regID("light.kitchen")
	light, ok := e.endpoint("light.kitchen")
	if !ok {
		t.Fatal("no light")
	}
	e.ha.UpdateEntity("light.kitchen", map[string]any{"disabled_by": "user"})
	e.eventually("the disabled light is gone", func() bool { return e.served(key) == nil })

	// A foreign device serves the light's key as a part.
	host, err := device.NewBridgedNode(device.Options{StableKey: device.StringKey("foreign"), Name: "foreign"})
	if err != nil {
		t.Fatal(err)
	}
	part, err := device.NewOnOffPlugInUnit(device.Options{StableKey: device.StringKey(key), Name: "foreign part"})
	if err != nil {
		t.Fatal(err)
	}
	if err := host.AddPart(part.Device); err != nil {
		t.Fatal(err)
	}
	if err := e.dev.Act(func(n *node.Node) error { return n.Aggregator().Add(e.ctx, host) }); err != nil {
		t.Fatal(err)
	}

	// Removed from the registry: the Forget is held and refused.
	synced := e.br.Status().LastSync
	e.ha.RemoveEntity("light.kitchen")
	e.eventually("the removal is synced", func() bool { return e.br.Status().LastSync.After(synced) })
	if s := e.br.Status(); s.SyncError == "" {
		t.Fatalf("the refused Forget is not reported: %+v", s)
	}

	// The key is free again: the next sync forgets it.
	if err := e.dev.Act(func(n *node.Node) error { return n.Aggregator().Remove(e.ctx, device.StringKey("foreign")) }); err != nil {
		t.Fatal(err)
	}
	if err := e.br.Resync(e.ctx); err != nil {
		t.Fatalf("Resync after the key is free: %v", err)
	}

	e.ha.RestoreEntity("light.kitchen")
	e.ha.UpdateEntity("light.kitchen", map[string]any{"disabled_by": nil})
	e.eventually("the re-added light is back", func() bool {
		_, ok := e.endpoint("light.kitchen")
		return ok
	})
	if got, _ := e.endpoint("light.kitchen"); got == light {
		t.Fatalf("the re-added light got its old number %d: the refused Forget was lost", light)
	}
}

// TestRefusedReplaceIsRetried (R7 review L4): since go-fabric v0.26.0 a
// batch holds a Replace whose device brings a part key another endpoint
// serves and refuses it at its commit. The group keeps no signature, so
// a later sync replaces it once the part key is free.
func TestRefusedReplaceIsRetried(t *testing.T) {
	e := newEnv(t, expose.Rule{Assist: true})
	key := "ha:dev-zha-2/" + e.regID("binary_sensor.front_door")
	e.eventually("the contact sensor is served", func() bool { return e.served(key) != nil })
	if n := len(e.served(key).Spec().Parts); n != 0 {
		t.Fatalf("the contact sensor has %d parts", n)
	}

	// A foreign device serves the part key the rebuilt device brings.
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

	// A smoke sensor is a SmokeCoAlarm with a PowerSource part (<key>#1):
	// the Replace is held and refused.
	e.ha.SetState("binary_sensor.front_door", "off", map[string]any{"device_class": "smoke", "friendly_name": "Front door contact Door"})
	if err := e.br.Resync(e.ctx); err == nil || !strings.Contains(err.Error(), key+"#1") {
		t.Fatalf("Resync: %v", err)
	}
	if n := len(e.served(key).Spec().Parts); n != 0 {
		t.Fatalf("the refused replacement is served (%d parts)", n)
	}

	if err := e.dev.Act(func(n *node.Node) error { return n.Aggregator().Remove(e.ctx, device.StringKey("foreign")) }); err != nil {
		t.Fatal(err)
	}
	if err := e.br.Resync(e.ctx); err != nil {
		t.Fatalf("Resync after the part key is free: %v", err)
	}
	if ps := e.served(key).Spec().Parts; len(ps) != 1 || ps[0].StableKey.String() != key+"#1" {
		t.Fatalf("the smoke alarm was not replaced again: %d parts", len(ps))
	}
}
