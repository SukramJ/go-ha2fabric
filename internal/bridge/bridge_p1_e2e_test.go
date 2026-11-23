// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package bridge_test

import (
	"context"
	"strings"
	"testing"

	onoffdef "thingfabric.dev/go-fabric/cluster/spec/onoff"
	"thingfabric.dev/go-fabric/controller/client"
	"thingfabric.dev/go-fabric/device"
	"thingfabric.dev/go-fabric/fabrictest"
	"thingfabric.dev/go-fabric/node"

	"github.com/SukramJ/go-ha2fabric/internal/bridge"
	"github.com/SukramJ/go-ha2fabric/internal/expose"
	"github.com/SukramJ/go-ha2fabric/internal/ha"
)

// served is the endpoint the node serves under key, or nil.
func (e *env) served(key string) node.Endpoint {
	for _, ep := range e.dev.Node().Aggregator().Endpoints() {
		if ep.Spec().StableKey.String() == key {
			return ep
		}
	}
	return nil
}

// TestFailedBatchOperationIsRetried (HR-P1-err-31): an Add the node
// refuses is not taken as served. Resync reports it, and the next sync
// adds the endpoint once the node takes it.
func TestFailedBatchOperationIsRetried(t *testing.T) {
	e := newEnv(t, expose.Rule{Assist: true})
	key := "ha:dev-hue-1/" + e.regID("light.kitchen")
	e.ha.SetExposed("light.kitchen", false)
	e.eventually("the light left", func() bool { return e.served(key) == nil })

	// Something else holds the light's key when it comes back.
	plug, err := device.NewOnOffPlugInUnit(device.Options{StableKey: device.StringKey(key), Name: "foreign"})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.dev.Act(func(n *node.Node) error { return n.Aggregator().Add(e.ctx, plug) }); err != nil {
		t.Fatal(err)
	}
	e.ha.SetExposed("light.kitchen", true)
	err = e.br.Resync(e.ctx)
	if err == nil || !strings.Contains(err.Error(), key) {
		t.Fatalf("Resync: %v", err)
	}
	if s := e.br.Status(); s.SyncError == "" {
		t.Fatalf("status %+v", s)
	}

	if err := e.dev.Act(func(n *node.Node) error { return n.Aggregator().Remove(e.ctx, device.StringKey(key)) }); err != nil {
		t.Fatal(err)
	}
	if err := e.br.Resync(e.ctx); err != nil {
		t.Fatalf("Resync after the key is free: %v", err)
	}
	ep := e.served(key)
	if ep == nil {
		t.Fatal("the light was not added again")
	}
	if _, foreign := ep.(*device.OnOffPlugInUnit); foreign {
		t.Fatal("the foreign endpoint is still served")
	}
	if s := e.br.Status(); s.SyncError != "" {
		t.Fatalf("status %+v", s)
	}
}

// gated holds every call_service on one entity until hold is closed.
type gated struct {
	bridge.Upstream
	entity string
	held   chan struct{}
	hold   chan struct{}
}

func (g *gated) CallService(ctx context.Context, sc ha.ServiceCall) error {
	if sc.EntityID == g.entity {
		g.held <- struct{}{}
		<-g.hold
	}
	return g.Upstream.CallService(ctx, sc)
}

// TestQueuedCommandOfUnexposedEntity (HR-P1-conc-33): a command queued
// while its entity was exposed is not called once a sync took the
// entity's endpoint away.
func TestQueuedCommandOfUnexposedEntity(t *testing.T) {
	g := &gated{entity: "light.kitchen", held: make(chan struct{}, 1), hold: make(chan struct{})}
	e := newEnv(t, expose.Rule{Assist: true}, func(s *setup) {
		s.wrap = func(up bridge.Upstream) bridge.Upstream { g.Upstream = up; return g }
	})
	e.commission()
	light, coffee := e.mustEndpoint("light.kitchen"), e.mustEndpoint("switch.coffee")
	mark := len(e.ha.Calls())
	if _, err := fabrictest.Invoke(e.ctx, e.pair, client.CommandAt(light, onoffdef.ClusterID, onoffdef.CmdOff), nil); err != nil {
		t.Fatal(err)
	}
	<-g.held // the light's call is running
	if _, err := fabrictest.Invoke(e.ctx, e.pair, client.CommandAt(coffee, onoffdef.ClusterID, onoffdef.CmdOn), nil); err != nil {
		t.Fatal(err)
	}
	e.ha.SetExposed("switch.coffee", false)
	if err := e.br.Resync(e.ctx); err != nil {
		t.Fatal(err)
	}
	if _, ok := e.endpoint("switch.coffee"); ok {
		t.Fatal("the switch is still served")
	}
	close(g.hold)
	e.nextCall(mark, "light.kitchen")
	e.noCall(mark, "switch.coffee")
	e.eventually("the dropped command is counted", func() bool { return e.br.Status().CommandErrs == 1 })
}

// TestExcludeLabelRenamed (HR-P1-cfg-34): an exclude label configured by
// its name keeps excluding after Home Assistant renames the label (the
// rename keeps its label_id); the warning says the name is followed by
// its id. Once the label is gone, the warning says the exclusion lapsed.
func TestExcludeLabelRenamed(t *testing.T) {
	e := newEnv(t, expose.Rule{Include: []string{"sensor.outdoor"}, ExcludeLabels: []string{"Matter"}})
	if v := byID(e.br.Plan())["sensor.outdoor"]; v.Exposed {
		t.Fatalf("not excluded: %+v", v)
	}
	synced := e.br.Status().LastSync
	e.ha.UpdateLabel("matter", "Bridge")
	e.eventually("the rename is synced", func() bool { return e.br.Status().LastSync.After(synced) })
	if v := byID(e.br.Plan())["sensor.outdoor"]; v.Exposed || v.Source != expose.SourceExclude {
		t.Fatalf("the renamed label no longer excludes: %+v", v)
	}
	if w := e.br.Status().Warnings; len(w) != 1 || !strings.Contains(w[0], `"Matter"`) || !strings.Contains(w[0], `"matter"`) {
		t.Fatalf("warnings %q", w)
	}
	synced = e.br.Status().LastSync
	e.ha.RemoveLabel("matter")
	e.eventually("the removal is synced", func() bool { return e.br.Status().LastSync.After(synced) })
	if w := e.br.Status().Warnings; len(w) != 1 || !strings.Contains(w[0], "excludes nothing") {
		t.Fatalf("warnings after the removal %q", w)
	}
}

// TestEntityMovedToAnotherDevice (HR-P1-test-38): an entity whose
// registry device changes gets a new stable key, so a new endpoint; its
// old key is no longer served.
func TestEntityMovedToAnotherDevice(t *testing.T) {
	e := newEnv(t, expose.Rule{Assist: true})
	reg := e.regID("switch.coffee")
	old := e.mustEndpoint("switch.coffee")
	var oldKey string
	for _, ep := range e.dev.Node().Aggregator().Endpoints() {
		if k := ep.Spec().StableKey.String(); strings.HasSuffix(k, "/"+reg) {
			oldKey = k
		}
	}
	newKey := "ha:dev-hue-1/" + reg
	if oldKey == "" || oldKey == newKey {
		t.Fatalf("old key %q", oldKey)
	}
	e.ha.UpdateEntity("switch.coffee", map[string]any{"device_id": "dev-hue-1"})
	e.eventually("the switch moved", func() bool { _, ok := e.dev.EndpointOf(newKey); return ok })
	if e.served(oldKey) != nil {
		t.Fatalf("the old key %s is still served", oldKey)
	}
	if again, ok := e.dev.EndpointOf(newKey); !ok || again == old {
		t.Fatalf("the moved switch is on %d (was %d)", again, old)
	}
}
