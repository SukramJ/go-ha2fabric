// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package plan

import (
	"maps"
	"slices"

	"github.com/SukramJ/go-fabric/device"
)

// Built is one device group as the reconciler serves it: the signature
// its plan was built from and its endpoints' stable keys, in
// go-hamapping's endpoint order, each with the endpoint the batch adds
// or replaces.
type Built[E any] struct {
	Key, Sig string
	Keys     []device.SourceKey
	// Endpoints pair with Keys: len(Endpoints) == len(Keys), both empty
	// after a failed build.
	Endpoints []E
}

// OpKind is what one batch operation does (go-fabric's node.Batch).
type OpKind uint8

const (
	// Remove takes an endpoint off; the store keeps its number for the
	// key (node.Batch.Remove).
	Remove OpKind = iota + 1
	// Forget takes an endpoint off, if served, and frees its stored
	// number (node.Batch.Forget).
	Forget
	// Replace swaps the endpoint served under a key (node.Batch.Replace).
	Replace
	// Add serves a new endpoint (node.Batch.Add).
	Add
)

// Op is one batch operation.
type Op[E any] struct {
	Kind OpKind
	Key  device.SourceKey
	// Endpoint is the endpoint of a Replace or an Add.
	Endpoint E
	// Entity is a Forget's entity id, for the log.
	Entity string
}

// Changes is the batch that brings the served groups to the planned
// ones, and what it changes.
type Changes[E any] struct {
	// Ops are the operations in batch order: the removed groups', the
	// replaced groups', the forgets no group's drop made, the added
	// groups'.
	Ops []Op[E]
	// Added, Replaced and Removed are group keys, in Ops order.
	Added, Replaced, Removed []string
	// Forgotten is every forgotten key (key.String()) with its entity id.
	Forgotten map[string]string
}

// Diff is the batch that brings old to next. next is in the order
// [Assess] returns the groups; a group whose Sig is old's is kept, with
// no operation. forget is the stable key of every entity removed from
// Home Assistant's registry since the last sync, by entity id.
//
// A group only in old is removed and a group only in next added; a group
// in both with another Sig is replaced key by key: Replace for a key both
// hold, Add for a new key, a drop for a vanished one. A dropped key is
// forgotten when forget holds it, else removed. A forget whose key a
// group of next serves is dropped silently — the entity is back and
// keeps its endpoint; one whose key no group dropped is forgotten all
// the same, freeing the number of an entity that was not served. Removed
// groups and those trailing forgets are in sorted key order.
func Diff[E any](old map[string]*Built[E], next []*Built[E], forget map[string]device.SourceKey) Changes[E] {
	ch := Changes[E]{Forgotten: map[string]string{}}
	planned := make(map[string]bool, len(next))
	live := map[string]bool{}
	for _, g := range next {
		planned[g.Key] = true
		for _, k := range g.Keys {
			live[k.String()] = true
		}
	}
	keys := map[string]device.SourceKey{}
	for id, k := range forget {
		if !live[k.String()] {
			ch.Forgotten[k.String()] = id
			keys[k.String()] = k
		}
	}
	dropped := map[string]bool{}
	drop := func(k device.SourceKey) {
		if id, ok := ch.Forgotten[k.String()]; ok {
			dropped[k.String()] = true
			ch.Ops = append(ch.Ops, Op[E]{Kind: Forget, Key: k, Entity: id})
			return
		}
		ch.Ops = append(ch.Ops, Op[E]{Kind: Remove, Key: k})
	}
	for _, key := range slices.Sorted(maps.Keys(old)) {
		if planned[key] {
			continue
		}
		ch.Removed = append(ch.Removed, key)
		for _, k := range old[key].Keys {
			drop(k)
		}
	}
	for _, g := range next {
		was := old[g.Key]
		if was == nil || was.Sig == g.Sig {
			continue
		}
		ch.Replaced = append(ch.Replaced, g.Key)
		had := map[device.SourceKey]bool{}
		for _, k := range was.Keys {
			had[k] = true
		}
		for i, k := range g.Keys {
			kind := Add
			if had[k] {
				kind = Replace
				delete(had, k)
			}
			ch.Ops = append(ch.Ops, Op[E]{Kind: kind, Key: k, Endpoint: g.Endpoints[i]})
		}
		for _, k := range was.Keys {
			if had[k] {
				drop(k)
			}
		}
	}
	for _, k := range slices.Sorted(maps.Keys(ch.Forgotten)) {
		if !dropped[k] {
			ch.Ops = append(ch.Ops, Op[E]{Kind: Forget, Key: keys[k], Entity: ch.Forgotten[k]})
		}
	}
	for _, g := range next {
		if old[g.Key] != nil {
			continue
		}
		ch.Added = append(ch.Added, g.Key)
		for i, k := range g.Keys {
			ch.Ops = append(ch.Ops, Op[E]{Kind: Add, Key: k, Endpoint: g.Endpoints[i]})
		}
	}
	return ch
}
