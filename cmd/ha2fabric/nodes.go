// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"unicode/utf8"

	fabricui "github.com/SukramJ/go-fabric-ui/fabricui"
	"github.com/SukramJ/go-fabric/node"

	"github.com/SukramJ/go-ha2fabric/internal/bridge"
	"github.com/SukramJ/go-ha2fabric/internal/camera"
	"github.com/SukramJ/go-ha2fabric/internal/config"
	"github.com/SukramJ/go-ha2fabric/internal/expose"
	"github.com/SukramJ/go-ha2fabric/internal/ha"
	"github.com/SukramJ/go-ha2fabric/internal/nodes"
	"github.com/SukramJ/go-ha2fabric/internal/plan"
	"github.com/SukramJ/go-ha2fabric/internal/selection"
)

// served is one node the daemon serves: the go-fabric node, its bridge
// on the one Home Assistant connection, its selection and the web UI's
// events of its bridge.
type served struct {
	n      *node.Node
	br     *bridge.Bridge
	events *fabricui.Events
	fan    *ha.Fanout
	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once
}

// serve starts a node and its bridge in the cookbook's order
// (go-fabric docs/guides/embed-in-a-daemon.md §6): node.New and Start,
// the fabrics and the pairing codes to the log, the bridge on the node,
// its worker on base, and the bridge on the connection's fan-out, which
// hands it Connected at once when Home Assistant is up. in is the node's
// plan input; sel its overrides store, which the web UI writes.
func serve(base context.Context, client *ha.Client, fan *ha.Fanout, ncfg node.Config, in func() plan.Input, sel *selection.Store, log *slog.Logger) (*served, error) {
	n, err := node.New(base, ncfg)
	if err != nil {
		return nil, fmt.Errorf("node: %w", err)
	}
	if err := n.Start(base); err != nil {
		_ = n.Stop(context.WithoutCancel(base))
		return nil, fmt.Errorf("node start: %w", err)
	}
	bridge.LogFabrics(base, n, log)
	bridge.LogPairing(n, log)
	// The web UI's event stream observes the node itself (Node.Observe,
	// go-fabric-ui v0.1.2); events carries the bridge's own.
	events := fabricui.NewEvents()
	br := bridge.New(n, client, bridge.Options{
		Input: in, Logger: log, Selection: sel, CameraUpstream: camera.HA(client),
		OnChange: func(exposure []string, endpoints bool) {
			if len(exposure) > 0 {
				events.Publish(fabricui.Event{Type: fabricui.EventExposureChanged, Data: fabricui.ExposureEvent{Keys: exposure}})
			}
			if endpoints {
				events.Publish(fabricui.Event{Type: fabricui.EventEndpointsChanged, Data: struct{}{}})
			}
		},
	})
	ctx, cancel := context.WithCancel(base)
	s := &served{n: n, br: br, events: events, fan: fan, cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(s.done)
		_ = br.Run(ctx)
	}()
	fan.Add(br)
	return s, nil
}

// close takes the bridge off the connection, ends its worker and stops
// the node; a second call does nothing.
func (s *served) close(ctx context.Context) (err error) {
	s.once.Do(func() {
		s.fan.Remove(s.br)
		s.cancel()
		<-s.done
		err = s.n.Stop(ctx)
	})
	return err
}

// member is one registry node: its record and, while it runs, its served
// node (nil when it failed to start).
type member struct {
	rec nodes.Node
	s   *served
}

// fleet is the daemon's [fabricui.NodeRegistry] (go-fabric-ui contract
// 0.6.0, concept U6 C.3): the nodes beside the configured one, each a
// node of its own with its own store, selection and bridge, all on the
// daemon's one Home Assistant connection. A registry node starts with
// the daemon and in Create; Delete stops it, removes its fabrics, its
// files and its record.
//
// A registry node's exposure is its selection alone: its rule exposes
// nothing by itself (no Assist, no lists), so a new node serves nothing
// until the operator picks its entities — the configured lists are the
// configured node's.
type fleet struct {
	base   context.Context
	cfg    *config.Config
	client *ha.Client
	fan    *ha.Fanout
	reg    *nodes.Registry
	log    *slog.Logger

	// op serialises Create, Rename, Delete and Close; mu guards the list
	// List reads.
	op      sync.Mutex
	mu      sync.RWMutex
	members []*member
	closed  bool
}

// newFleet starts every node of reg on base; a node that does not start
// is logged and listed as not enabled.
func newFleet(base context.Context, cfg *config.Config, client *ha.Client, fan *ha.Fanout, reg *nodes.Registry, log *slog.Logger) *fleet {
	f := &fleet{base: base, cfg: cfg, client: client, fan: fan, reg: reg, log: log}
	for _, rec := range reg.Nodes() {
		s, err := f.start(&rec)
		if err != nil {
			log.Error("ha2fabric.node_start", slog.String("node", rec.Key), slog.String("listen", rec.Listen), slog.String("error", err.Error()))
		}
		f.members = append(f.members, &member{rec: rec, s: s})
	}
	return f
}

// nodeLabel is a registry label as the node's NodeLabel: at most 32 bytes
// (go-fabric cluster/core/basic_information.go, as config's LABEL), cut
// at a character boundary.
func nodeLabel(label string) string {
	for len(label) > 32 {
		_, size := utf8.DecodeLastRuneInString(label)
		label = label[:len(label)-size]
	}
	return label
}

// start serves rec: the daemon's configuration with the node's listen
// address, commissioning values, label and store, its selection file.
func (f *fleet) start(rec *nodes.Node) (*served, error) {
	c := *f.cfg
	c.Listen, c.Passcode, c.Discriminator = rec.Listen, rec.Passcode, rec.Discriminator
	c.Label = nodeLabel(rec.Label)
	c.StorePath = f.reg.Path(rec.StoreFile())
	log := f.log.With(slog.String("node", rec.Key))
	sel, err := selection.Open(f.reg.Path(rec.SelectionFile()), log)
	if err != nil {
		return nil, err
	}
	in := func() plan.Input { return plan.Effective(expose.Rule{}, sel, c.Camera, camera.DryPorts) }
	return serve(f.base, f.client, f.fan, bridge.NodeConfig(&c, log), in, sel, log)
}

// List implements [fabricui.NodeRegistry]: from memory.
func (f *fleet) List() []fabricui.NodeEntry {
	f.mu.RLock()
	defer f.mu.RUnlock()
	out := make([]fabricui.NodeEntry, 0, len(f.members))
	for _, m := range f.members {
		out = append(out, f.entry(m))
	}
	return out
}

// entry is m as the node selector and its pages see it; f.mu is held.
func (f *fleet) entry(m *member) fabricui.NodeEntry {
	e := fabricui.NodeEntry{
		Key: m.rec.Key, Label: m.rec.Label, Kind: "bridge",
		Config: &fabricui.NodeConfigView{
			Source: fabricui.NodeConfigSourceRegistry, Label: m.rec.Label,
			Listen: m.rec.Listen, Discriminator: m.rec.Discriminator,
			VendorID: f.cfg.VendorID, VendorName: bridge.VendorName,
			ProductID: f.cfg.ProductID, ProductName: bridge.ProductName,
			Store: m.rec.StoreFile(),
		},
	}
	if m.s != nil {
		e.Source = m.s.n
		e.Exposure = m.s.br.Exposure()
		e.Reassemble = m.s.br.Resync
		e.Events = m.s.events
	}
	return e
}

// errStopping answers a write while the daemon stops.
var errStopping = errors.New("the daemon is stopping")

// Create implements [fabricui.NodeRegistry]: allocate, start, persist.
func (f *fleet) Create(ctx context.Context, spec fabricui.NodeSpec) (fabricui.NodeEntry, error) {
	f.op.Lock()
	defer f.op.Unlock()
	if f.closed {
		return fabricui.NodeEntry{}, errStopping
	}
	if f.reg.Len() >= f.cfg.NodeLimit {
		return fabricui.NodeEntry{}, fmt.Errorf("%w: at most %d nodes beside the configured one (NODE_LIMIT)", fabricui.ErrNodeLimit, f.cfg.NodeLimit)
	}
	rec, err := f.reg.Allocate(spec.Label, nodes.Configured{Listen: f.cfg.Listen, Discriminator: f.cfg.Discriminator, TCP: f.cfg.Camera})
	if err != nil {
		return fabricui.NodeEntry{}, err
	}
	s, err := f.start(&rec)
	if err != nil {
		_ = f.reg.RemoveFiles(&rec)
		return fabricui.NodeEntry{}, fmt.Errorf("node %s: %w", rec.Key, err)
	}
	if err := f.reg.Add(rec); err != nil {
		_ = s.close(context.WithoutCancel(ctx))
		_ = f.reg.RemoveFiles(&rec)
		return fabricui.NodeEntry{}, err
	}
	m := &member{rec: rec, s: s}
	f.mu.Lock()
	f.members = append(f.members, m)
	e := f.entry(m)
	f.mu.Unlock()
	f.log.Info("ha2fabric.node_created", slog.String("node", rec.Key), slog.String("listen", rec.Listen), slog.Int("discriminator", int(rec.Discriminator)))
	return e, nil
}

// find is the member of key; f.mu or f.op is held.
func (f *fleet) find(key string) *member {
	i := slices.IndexFunc(f.members, func(m *member) bool { return m.rec.Key == key })
	if i < 0 {
		return nil
	}
	return f.members[i]
}

// Rename implements [fabricui.NodeRegistry]: the label in the registry
// and the selector at once; the node's own NodeLabel follows at its next
// start (go-fabric's facade sets BasicInformation from node.Config only,
// and a NodeLabel a controller wrote wins over it).
func (f *fleet) Rename(_ context.Context, key, label string) error {
	f.op.Lock()
	defer f.op.Unlock()
	if f.closed {
		return errStopping
	}
	if err := f.reg.Rename(key, label); err != nil {
		return err
	}
	f.mu.Lock()
	if m := f.find(key); m != nil {
		m.rec.Label = label
	}
	f.mu.Unlock()
	f.log.Info("ha2fabric.node_renamed", slog.String("node", key))
	return nil
}

// Delete implements [fabricui.NodeRegistry], finally: the node leaves
// every fabric it holds (node.RemoveFabric: the Leave event, the sessions
// and subscriptions go, so the controllers lose it), stops, and its
// store, selection and record are removed. A file that cannot be removed
// leaves the record, listed as not enabled, for a second Delete.
func (f *fleet) Delete(ctx context.Context, key string) error {
	f.op.Lock()
	defer f.op.Unlock()
	if f.closed {
		return errStopping
	}
	m := f.find(key)
	if m == nil {
		return fmt.Errorf("%w: %s", nodes.ErrNotFound, key)
	}
	left := 0
	if s := m.s; s != nil {
		left = leave(ctx, s.n, f.log.With(slog.String("node", key)))
		if err := s.close(ctx); err != nil {
			f.log.Warn("ha2fabric.node_stop", slog.String("node", key), slog.String("error", err.Error()))
		}
		f.mu.Lock()
		m.s = nil
		f.mu.Unlock()
	}
	if err := f.reg.RemoveFiles(&m.rec); err != nil {
		return fmt.Errorf("node %s: %w", key, err)
	}
	if err := f.reg.Remove(key); err != nil {
		return err
	}
	f.mu.Lock()
	f.members = slices.DeleteFunc(f.members, func(x *member) bool { return x == m })
	f.mu.Unlock()
	f.log.Info("ha2fabric.node_deleted", slog.String("node", key), slog.Int("fabrics_removed", left))
	return nil
}

// leave removes every fabric of n and returns how many it removed.
func leave(ctx context.Context, n *node.Node, log *slog.Logger) int {
	infos, err := n.FabricInfos(ctx)
	if err != nil {
		log.Warn("ha2fabric.node_fabrics", slog.String("error", err.Error()))
		return 0
	}
	removed := 0
	for _, fi := range infos {
		if err := n.RemoveFabric(ctx, fi.FabricIndex); err != nil {
			log.Warn("ha2fabric.node_remove_fabric", slog.Int("fabric_index", int(fi.FabricIndex)), slog.String("error", err.Error()))
			continue
		}
		removed++
	}
	return removed
}

// Close stops every registry node; Create, Rename and Delete fail after
// it.
func (f *fleet) Close(ctx context.Context) {
	f.op.Lock()
	defer f.op.Unlock()
	f.closed = true
	f.mu.RLock()
	members := slices.Clone(f.members)
	f.mu.RUnlock()
	for _, m := range members {
		if m.s == nil {
			continue
		}
		if err := m.s.close(ctx); err != nil {
			f.log.Warn("ha2fabric.node_stop", slog.String("node", m.rec.Key), slog.String("error", err.Error()))
		}
	}
}
