// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package bridge

import (
	"errors"
	"log/slog"
	"testing"

	"thingfabric.dev/go-fabric/clock"
	"thingfabric.dev/go-fabric/device"
	"thingfabric.dev/go-fabric/fabrictest"
	"thingfabric.dev/go-fabric/node"

	"thingfabric.dev/go-ha2fabric/internal/plan"
)

// batchOps is a diff's operations for the merge: a Forget of a, an Add
// and a Replace of b, a Remove of c.
func batchOps() []plan.Op[*device.Device] {
	return []plan.Op[*device.Device]{
		{Kind: plan.Forget, Key: device.StringKey("a"), Entity: "light.a"},
		{Kind: plan.Add, Key: device.StringKey("b")},
		{Kind: plan.Replace, Key: device.StringKey("b")},
		{Kind: plan.Remove, Key: device.StringKey("c")},
		{Kind: plan.Add, Key: device.StringKey("b")},
	}
}

// TestBatchCallErrors: an entry of a *node.BatchError's Calls marks
// every operation of its kind and key, joined to what the call answered;
// an entry no operation matches, Fn alone and a plain error change
// nothing.
func TestBatchCallErrors(t *testing.T) {
	answered := errors.New("answered at the call")
	skipped := errors.New(`node: the erasure of "a" is skipped`)
	refused := errors.New("held key refused")
	ops := batchOps()
	errs := make([]error, len(ops))
	errs[0] = answered
	batchCallErrors(ops, errs, &node.BatchError{Calls: []*node.BatchCallError{
		{Op: node.BatchForget, Key: device.StringKey("a"), Applied: true, Err: skipped},
		{Op: node.BatchAdd, Key: device.StringKey("b"), Err: refused},
		{Op: node.BatchRemove, Key: device.StringKey("z"), Err: refused},
		nil,
	}})
	if !errors.Is(errs[0], answered) || !errors.Is(errs[0], skipped) {
		t.Fatalf("the Forget's error %v, want the call's and the entry's", errs[0])
	}
	if !errors.Is(errs[1], refused) || !errors.Is(errs[4], refused) {
		t.Fatalf("the Adds of b %v, %v: an entry marks every operation of its kind and key", errs[1], errs[4])
	}
	if errs[2] != nil || errs[3] != nil {
		t.Fatalf("the Replace of b %v, the Remove of c %v: no entry of theirs", errs[2], errs[3])
	}

	for _, err := range []error{
		&node.BatchError{Fn: refused},
		errors.Join(refused, skipped),
		nil,
	} {
		errs := make([]error, len(ops))
		batchCallErrors(ops, errs, err)
		for i, e := range errs {
			if e != nil {
				t.Fatalf("%v marked operation %d: %v", err, i, e)
			}
		}
	}
}

// TestSkippedErasureForgetIsRecordedAgain: a Forget that answered nil and
// whose erasure go-fabric skipped after fn is recorded for the next sync,
// once the batch's error is merged in; without the entry it is taken as
// applied.
func TestSkippedErasureForgetIsRecordedAgain(t *testing.T) {
	site := fabrictest.NewSite(t, fabrictest.Options{Clock: clock.System{}, Logger: slog.New(slog.DiscardHandler)})
	dev := site.AddDevice(fabrictest.Topology{})
	b := New(dev.Node(), &fakeUp{}, Options{Logger: slog.New(slog.DiscardHandler), Input: func() plan.Input { return plan.Input{} }})
	ops := batchOps()[:1]

	errs := make([]error, 1)
	b.settle(ops, errs, nil)
	if _, ok := b.forget["light.a"]; ok {
		t.Fatal("a Forget without an error was recorded again")
	}

	batchCallErrors(ops, errs, &node.BatchError{Calls: []*node.BatchCallError{
		{Op: node.BatchForget, Key: device.StringKey("a"), Applied: true, Err: errors.New(`node: the erasure of "a" is skipped`)},
	}})
	b.settle(ops, errs, nil)
	if k, ok := b.forget["light.a"]; !ok || k.String() != "a" {
		t.Fatalf("the skipped erasure's Forget not recorded again: %v", b.forget)
	}
}
