// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package plan_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"thingfabric.dev/go-ha2fabric/internal/expose"
	"thingfabric.dev/go-ha2fabric/internal/ha"
	"thingfabric.dev/go-ha2fabric/internal/plan"
)

// manySelects is the fixtures with n select entities of 256 options each
// (the cap go-hamapping maps) on the first device.
func manySelects(tb testing.TB, n int) (*ha.Snapshot, plan.Input) {
	tb.Helper()
	opts := make([]any, 256)
	for i := range opts {
		opts[i] = fmt.Sprintf("option number %03d of a long list", i)
	}
	s := snapshot(tb)
	dev := s.Devices[0].ID
	for i := range n {
		id := fmt.Sprintf("select.many_%04d", i)
		s.Entities = append(s.Entities, ha.EntityEntry{EntityID: id, ID: fmt.Sprintf("reg-%d", i), DeviceID: dev})
		s.States = append(s.States, ha.State{EntityID: id, State: "x", Attributes: map[string]any{"options": opts, "friendly_name": id}})
	}
	return s, plan.Input{Rule: expose.Rule{Include: []string{"select.many_*"}}}
}

// TestSignature: a group's signature is its device and each entity's key
// and description, in order — compared only for equality (HR-P4-bound-43
// keeps that meaning).
func TestSignature(t *testing.T) {
	s, in := manySelects(t, 3)
	_, groups := plan.Assess(s, in)
	if len(groups) != 1 {
		t.Fatalf("%d groups", len(groups))
	}
	g := groups[0]
	dev, _ := json.Marshal(g.Device)
	want := string(dev)
	for _, e := range g.Entities {
		want += "|" + e.Key() + "=" + g.Descs[e.Key()]
	}
	if g.Sig != want {
		t.Fatalf("signature\n got  %.200q\n want %.200q", g.Sig, want)
	}
}

// BenchmarkAssessManyEntities is HR-P4-bound-43's probe: Assess of one
// device with 1 000 selects of 256 options. Quadratic before (3.5 s on
// the review's machine), linear in the signature's size now.
func BenchmarkAssessManyEntities(b *testing.B) {
	s, in := manySelects(b, 1000)
	for b.Loop() {
		plan.Assess(s, in)
	}
}
