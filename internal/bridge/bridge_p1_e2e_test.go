// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package bridge_test

import (
	"strings"
	"testing"

	"thingfabric.dev/go-fabric/device"
	"thingfabric.dev/go-fabric/node"

	"github.com/SukramJ/go-ha2fabric/internal/expose"
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
