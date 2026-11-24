// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package bridge

// HeldStates is how many states the bridge holds beside its last
// snapshot (HR-P2-bound-38).
func HeldStates(b *Bridge) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.states)
}
