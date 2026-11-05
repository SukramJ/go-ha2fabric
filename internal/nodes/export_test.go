// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package nodes

import (
	"io"
	"time"
)

// SetSources replaces the allocation's randomness, port probe and clock.
func (r *Registry) SetSources(rnd io.Reader, free func(network, addr string) bool, now func() time.Time) {
	if rnd != nil {
		r.rand = rnd
	}
	if free != nil {
		r.free = free
	}
	if now != nil {
		r.now = now
	}
}

// ValidKey is validKey.
var ValidKey = validKey
