// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package bridge

import (
	"context"

	"github.com/SukramJ/go-hamqtt/model"
)

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
