// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package ha

import (
	"slices"
	"sync"
)

// Fanout is a [Handler] that hands the connection's life and every event
// to each handler it holds: one client, several nodes (go-fabric-ui
// concept U6, C.3 — one bridge per node on the daemon's one Home Assistant
// connection). Each handler's own work — its re-sync, its selection —
// stays its own.
//
// A handler added while the connection is up gets [Handler.Connected] at
// once, with the version of the connection, so a node created at run time
// reads Home Assistant as the ones present at the connection did. The
// methods run on the client's goroutines and the caller's; the handlers
// must not block, as [Handler] says, since Fanout calls them under its
// lock to keep each handler's order of Connected, Event and Disconnected.
type Fanout struct {
	mu        sync.Mutex
	hs        []Handler
	connected bool
	version   string
}

// Add hands the connection to h from now on; h gets Connected at once
// while the connection is up. Adding h twice is one h.
func (f *Fanout) Add(h Handler) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if slices.Contains(f.hs, h) {
		return
	}
	f.hs = append(f.hs, h)
	if f.connected {
		h.Connected(f.version)
	}
}

// Remove stops handing the connection to h; it gets no Disconnected.
func (f *Fanout) Remove(h Handler) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hs = slices.DeleteFunc(f.hs, func(x Handler) bool { return x == h })
}

// Len is the number of handlers.
func (f *Fanout) Len() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.hs)
}

// Connected implements [Handler].
func (f *Fanout) Connected(haVersion string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.connected, f.version = true, haVersion
	for _, h := range f.hs {
		h.Connected(haVersion)
	}
}

// Event implements [Handler].
func (f *Fanout) Event(ev Event) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, h := range f.hs {
		h.Event(ev)
	}
}

// Disconnected implements [Handler].
func (f *Fanout) Disconnected(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.connected = false
	for _, h := range f.hs {
		h.Disconnected(err)
	}
}
