// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package bridge

import (
	"context"
	"maps"
	"slices"
	"sync"
	"testing"
	"time"

	hamapping "sukramj.dev/go-hamapping"
	"sukramj.dev/go-hamqtt/model"
)

// SetCameraEvictIdle sets the idle time after which go-fabric's kit may
// evict a camera's session at its bounds (camera.Options.EvictIdle).
func SetCameraEvictIdle(o *Options, d time.Duration) { o.cameraEvictIdle = d }

// HeldStates is how many states the bridge holds beside its last
// snapshot (HR-P2-bound-38).
func HeldStates(b *Bridge) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.states)
}

// Serving reports whether the last sync serves entity id on an endpoint:
// the HR-P1-conc-33 guard's set.
func Serving(b *Bridge, id string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.serving[id]
}

// RunCommand runs one queued command as the command loop does.
func RunCommand(ctx context.Context, b *Bridge, c model.Command) { b.command(ctx, c) }

// FillCommands queues c until the command queue is full.
func FillCommands(b *Bridge, c model.Command) {
	for {
		select {
		case b.cmds <- c:
		default:
			return
		}
	}
}

// Forgets are the entity ids whose Forget the next sync makes.
func Forgets(b *Bridge) []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return slices.Sorted(maps.Keys(b.forget))
}

// Plans are the plan of entity id's group and the plan that serves its
// endpoint (group.live), read on the worker; nil when the bridge has no
// group of id.
func Plans(b *Bridge, id string) (own, live *hamapping.Plan) {
	type pair struct{ own, live *hamapping.Plan }
	ch := make(chan pair, 1)
	b.enqueue(func(context.Context) {
		if g := b.owner[id]; g != nil {
			ch <- pair{g.plan, g.live(id).plan}
			return
		}
		ch <- pair{}
	})
	p := <-ch
	return p.own, p.live
}

// PlanCloses counts the plans the bridge closes until the test ends.
func PlanCloses(t *testing.T) func(*hamapping.Plan) int {
	t.Helper()
	var mu sync.Mutex
	n := map[*hamapping.Plan]int{}
	saved := closePlan
	closePlan = func(p *hamapping.Plan) {
		mu.Lock()
		n[p]++
		mu.Unlock()
		saved(p)
	}
	t.Cleanup(func() { closePlan = saved })
	return func(p *hamapping.Plan) int {
		mu.Lock()
		defer mu.Unlock()
		return n[p]
	}
}
