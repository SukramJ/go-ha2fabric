// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	fabricui "github.com/SukramJ/go-fabric-ui/fabricui"
	"github.com/SukramJ/go-fabric/node"

	"github.com/SukramJ/go-ha2fabric/internal/bridge"
	"github.com/SukramJ/go-ha2fabric/internal/camera"
	"github.com/SukramJ/go-ha2fabric/internal/config"
	"github.com/SukramJ/go-ha2fabric/internal/expose"
	"github.com/SukramJ/go-ha2fabric/internal/exposure"
	"github.com/SukramJ/go-ha2fabric/internal/ha"
	"github.com/SukramJ/go-ha2fabric/internal/nodes"
	"github.com/SukramJ/go-ha2fabric/internal/plan"
	"github.com/SukramJ/go-ha2fabric/internal/selection"
)

// served is one node the daemon serves, the owner that outlives its run:
// what serve was given, the web UI's exposure provider and events, made
// once over the owner, and the switch the web UI reads the node through
// (fabricui.NodeSwitch, go-fabric-ui v0.7.1). The run is the go-fabric
// node and its bridge on the one Home Assistant connection; an erase
// ends it and starts a new one (a Go node is single-use, go-fabric
// node/node.go:39-41 at v0.18.0), so the web UI's handler, which keeps
// the values it was given, follows the new node through the switch.
type served struct {
	base   context.Context
	client *ha.Client
	fan    *ha.Fanout
	ncfg   node.Config
	in     func() plan.Input
	sel    *selection.Store
	log    *slog.Logger
	events *fabricui.Events
	prov   *exposure.Provider
	sw     *fabricui.NodeSwitch

	// op serialises erase and close; closed is set by close.
	op     sync.Mutex
	closed bool

	// mu guards the current run and the last failed come-back; live
	// mirrors n != nil for the fleet's List, which does not wait on an
	// erase.
	mu     sync.RWMutex
	n      *node.Node
	br     *bridge.Bridge
	cancel context.CancelFunc
	done   chan struct{}
	lost   error
	lostAt time.Time
	live   atomic.Bool
	// discarded is the discards of the runs an erase ended, so Discards
	// counts since the daemon started (go-fabric-ui contract 0.8.0,
	// Counter.Value).
	discarded uint64
}

// newNode is node.New; the daemon test swaps it to fail a come-back.
var newNode = node.New

// eraseTimeout bounds an erase: the reset and the come-back.
const eraseTimeout = 30 * time.Second

// serve builds the owner of a node and starts its first run (start). in
// is the node's plan input, the bridge's and the provider's; sel its
// overrides store, which the provider writes.
func serve(base context.Context, client *ha.Client, fan *ha.Fanout, ncfg node.Config, in func() plan.Input, sel *selection.Store, log *slog.Logger) (*served, error) {
	s := &served{base: base, client: client, fan: fan, ncfg: ncfg, in: in, sel: sel, log: log, events: fabricui.NewEvents()}
	s.prov = exposure.New(s, exposure.Options{Input: in, Selection: sel, Logger: log})
	if err := s.start(base); err != nil {
		return nil, err
	}
	return s, nil
}

// start runs a node and its bridge in the cookbook's order (go-fabric
// docs/guides/embed-in-a-daemon.md §6): node.New and Start, the fabrics
// and the pairing codes to the log, the bridge on the node, its worker on
// base, and the bridge on the connection's fan-out, which hands it
// Connected at once when Home Assistant is up; then the switch points at
// the node. s.mu is held, or s is not shared yet.
func (s *served) start(base context.Context) error {
	n, err := newNode(base, s.ncfg)
	if err != nil {
		return fmt.Errorf("node: %w", err)
	}
	if err := n.Start(base); err != nil {
		_ = n.Stop(context.WithoutCancel(base))
		return fmt.Errorf("node start: %w", err)
	}
	logFabrics(base, n, s.log)
	logPairing(n, s.log)
	logMDNSInterfaces(n, s.ncfg.Network.MDNS.Interfaces, s.log)
	// The web UI's event stream observes the node itself (Node.Observe,
	// go-fabric-ui v0.1.2); s.events carries the bridge's own.
	br := bridge.New(n, s.client, bridge.Options{
		Input: s.in, Logger: s.log, CameraUpstream: camera.HA(s.client),
		OnChange: func(exposure []string, endpoints bool) {
			if len(exposure) > 0 {
				s.events.Publish(fabricui.Event{Type: fabricui.EventExposureChanged, Data: fabricui.ExposureEvent{Keys: exposure}})
			}
			if endpoints {
				s.events.Publish(fabricui.Event{Type: fabricui.EventEndpointsChanged, Data: struct{}{}})
			}
		},
	})
	ctx, cancel := context.WithCancel(base)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = br.Run(ctx)
	}()
	s.n, s.br, s.cancel, s.done, s.lost = n, br, cancel, done, nil
	s.live.Store(true)
	s.fan.Add(br)
	// Set only after the new node started, never on a failed one
	// (go-fabric-ui fabricui/nodeswitch.go at v0.7.1).
	if s.sw == nil {
		s.sw = fabricui.NewNodeSwitch(n)
	} else {
		s.sw.Set(n)
	}
	return nil
}

// stopRun takes the bridge off the connection and ends its worker; the
// node is the caller's to stop or reset. s.mu is held and a run exists.
func (s *served) stopRun() {
	s.fan.Remove(s.br)
	s.cancel()
	<-s.done
}

// View implements [exposure.Model]: the current bridge's, a zero view
// without a run.
func (s *served) View() bridge.View {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.br == nil {
		return bridge.View{}
	}
	return s.br.View()
}

// Resync is the web UI's reassemble on the current bridge; without a run
// there is nothing to rebuild, and the next run applies the selection.
func (s *served) Resync(ctx context.Context) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.br == nil {
		return nil
	}
	return s.br.Resync(ctx)
}

// node is the current node, nil without a run.
func (s *served) node() *node.Node {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.n
}

// running reports whether s has a run; it does not wait on an erase.
func (s *served) running() bool { return s.live.Load() }

// Discards is how many controller commands this node's bridges answered
// with success and acted on nothing (bridge.Status.Discards), since the
// daemon started: the current run's and those an erase ended.
func (s *served) Discards() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := s.discarded
	if s.br != nil {
		n += s.br.Status().Discards
	}
	return n
}

// Warnings are the provider's and, while the last erase's come-back
// failed, node_erase_failed.
func (s *served) Warnings(ctx context.Context) []fabricui.Warning {
	out := s.prov.Warnings(ctx)
	s.mu.RLock()
	lost, at := s.lost, s.lostAt
	s.mu.RUnlock()
	if lost != nil {
		out = append(out, fabricui.Warning{
			Code:    exposure.WarningNodeEraseFailed,
			Message: "the node did not come back after the erase: " + lost.Error() + "; Erase again or restart the daemon",
			Since:   at,
		})
	}
	return out
}

// erase is the web UI's Erase (go-fabric-ui contract 0.7.0): the run's
// bridge leaves the connection, the node is reset (go-fabric
// Node.FactoryReset at v0.18.0: its fabrics leave through RemoveFabric —
// only on a started node, so no Stop before it —, it stops, its store and
// endpoint records are erased), and a new run starts from the same
// configuration on the erased store. Without a run — a come-back that
// failed — it only starts. A come-back that fails leaves s without a run
// and the switch on the stopped node; the daemon keeps running. It runs
// detached from the request, bounded by eraseTimeout.
func (s *served) erase(ctx context.Context) error {
	s.op.Lock()
	defer s.op.Unlock()
	if s.closed {
		return errStopping
	}
	ectx, cancel := context.WithTimeout(context.WithoutCancel(ctx), eraseTimeout)
	defer cancel()
	var rerr error
	s.mu.Lock()
	if s.n != nil {
		// Logged before the come-back, whose fabrics line (count=0) and
		// pairing codes it explains.
		began := time.Now()
		s.stopRun()
		removed := 0
		if infos, err := s.n.FabricInfos(ectx); err == nil {
			removed = len(infos)
		}
		s.discarded += s.br.Status().Discards
		rerr = s.n.FactoryReset(ectx)
		s.n, s.br, s.cancel, s.done = nil, nil, nil, nil
		s.live.Store(false)
		attrs := []any{slog.Int("fabrics_removed", removed), slog.Duration("elapsed", time.Since(began))}
		if rerr != nil {
			s.log.Warn("ha2fabric.node_erased", append(attrs, slog.String("error", rerr.Error()))...)
		} else {
			s.log.Info("ha2fabric.node_erased", attrs...)
		}
	}
	serr := s.start(s.base) //nolint:contextcheck // the new run lives on the daemon's base, as the first one; the request governs only the reset
	if serr != nil {
		s.lost, s.lostAt = serr, time.Now()
	}
	s.mu.Unlock()
	if serr != nil {
		s.log.Error("ha2fabric.node_lost", slog.String("error", serr.Error()),
			slog.String("hint", "the node did not come back after the erase; the daemon keeps running: Erase again, or restart the daemon, which comes up on the erased store"))
	}
	return errors.Join(rerr, serr)
}

// logFabrics writes the fabrics the store holds at start — their count,
// and per fabric the index, the vendor and the label — so an empty or lost
// store is visible in the log itself and not only through the commissioning
// window it opens (2026-10-30: a lost store was diagnosed from the pairing
// hint alone). With no fabric the line says what that means and how a
// store comes back.
func logFabrics(ctx context.Context, n *node.Node, logger *slog.Logger) {
	infos, err := n.FabricInfos(ctx)
	if err != nil {
		logger.Warn("ha2fabric.fabrics", slog.String("error", err.Error()))
		return
	}
	attrs := []any{slog.Int("count", len(infos))}
	for _, f := range infos {
		attrs = append(attrs, slog.Group(fmt.Sprintf("fabric_%d", f.FabricIndex),
			slog.Int("vendor_id", int(f.VendorID)), slog.String("label", f.Label)))
	}
	if len(infos) == 0 {
		attrs = append(attrs, slog.String("hint", "no controller has paired, so the commissioning window opens; a store lost with its fabrics comes back from the add-on's backup (docs/operations.md, Files)"))
	}
	logger.Info("ha2fabric.fabrics", attrs...)
}

// logMDNSInterfaces writes, at start, what go-fabric's diagnosis finds
// about the configured mDNS interfaces (MDNS_INTERFACES): a name the host
// does not have, a named bridge the default filter keeps out, a named
// interface without an advertised address (Node.DiagnoseMDNS, the
// interface_* codes, at v0.19.1). Warnings carry no mDNS finding, so the
// start-up log is where an operator who set the list reads it; the web
// UI's mDNS page shows the whole diagnosis. Without the list nothing is
// written.
func logMDNSInterfaces(n *node.Node, names []string, logger *slog.Logger) {
	if len(names) == 0 {
		return
	}
	for _, f := range n.DiagnoseMDNS() {
		if !strings.HasPrefix(f.Code, "interface_") {
			continue
		}
		logger.Warn("ha2fabric.mdns_interface", slog.String("code", f.Code), slog.String("severity", string(f.Severity)), slog.String("message", f.Message))
	}
}

// logPairing writes the pairing codes to the log (cookbook §7).
func logPairing(n *node.Node, logger *slog.Logger) {
	payload, err := n.SetupPayload()
	if err != nil {
		logger.Warn("ha2fabric.pairing_codes", slog.String("error", err.Error()))
		return
	}
	logger.Info("ha2fabric.pairing",
		slog.String("manual_code", payload.Manual), slog.String("qr_payload", payload.QR),
		slog.String("hint", "pair from a Matter controller with the manual code or the QR code in the web UI; the commissioning window is open while no controller has paired"))
}

// close takes the bridge off the connection, ends its worker and stops
// the node; an erase running meanwhile ends first, and a later one
// answers errStopping. A second call does nothing.
func (s *served) close(ctx context.Context) error {
	s.op.Lock()
	defer s.op.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.n == nil {
		return nil
	}
	s.stopRun()
	return s.n.Stop(ctx)
}

// member is one registry node: its record and its served node (nil when
// it failed to start; without a run when its erase did not come back).
type member struct {
	rec nodes.Node
	s   *served
}

// fleet is the daemon's [fabricui.NodeRegistry] (go-fabric-ui contract
// 0.6.0, concept U6 C.3): the nodes beside the configured one, each a
// node of its own with its own store, selection and bridge, all on the
// daemon's one Home Assistant connection. A registry node starts with
// the daemon and in Create; Erase resets it and starts it again on its
// erased store; Delete stops it, removes its fabrics, its files and its
// record.
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

	// op serialises Create, Rename, Erase, Delete and Close; mu guards
	// the list List reads.
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
	return serve(f.base, f.client, f.fan, c.Node(log), in, sel, log)
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
			VendorID: f.cfg.VendorID, VendorName: config.VendorName,
			ProductID: f.cfg.ProductID, ProductName: config.ProductName,
			Store: m.rec.StoreFile(),
		},
	}
	if s := m.s; s != nil {
		// The switch, the provider and the events are the owner's, the
		// same values across an erase, so the node keeps its handler
		// (go-fabric-ui fabricui/registry.go:163-177 at v0.7.1). Without
		// a run Source is nil: listed not enabled, its routes answer as
		// without a node.
		if s.running() {
			e.Source = s.sw
		}
		e.Exposure = s.prov
		e.Reassemble = s.Resync
		e.Events = s.events
		key := m.rec.Key
		e.Erase = func(ctx context.Context) error { return f.Erase(ctx, key) }
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
// and the selector, then the running node's own NodeLabel
// (Node.SetNodeLabel, go-fabric node/node_label.go:39 at v0.19.1: a host
// write, persisted, reported to the controllers' subscriptions), cut as
// at start (nodeLabel). The node's error is logged and leaves the
// registry's rename standing; a node without a run takes the label from
// its record at its next start, unless its store holds one written since
// (go-fabric restores a persisted label over the configured one).
func (f *fleet) Rename(ctx context.Context, key, label string) error {
	f.op.Lock()
	defer f.op.Unlock()
	if f.closed {
		return errStopping
	}
	if err := f.reg.Rename(key, label); err != nil {
		return err
	}
	f.mu.Lock()
	m := f.find(key)
	if m != nil {
		m.rec.Label = label
	}
	f.mu.Unlock()
	var n *node.Node
	if m != nil && m.s != nil {
		n = m.s.node()
	}
	if n == nil {
		f.log.Info("ha2fabric.node_renamed", slog.String("node", key))
		return nil
	}
	if err := n.SetNodeLabel(ctx, nodeLabel(label)); err != nil {
		f.log.Warn("ha2fabric.node_renamed", slog.String("node", key), slog.String("error", err.Error()))
		return nil
	}
	f.log.Info("ha2fabric.node_renamed", slog.String("node", key), slog.Bool("node_label", true))
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
		if n := s.node(); n != nil {
			left = leave(ctx, n, f.log.With(slog.String("node", key)))
		}
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

// Erase is a registry node's Erase (fabricui.NodeEntry.Erase): served's
// erase on the member of key.
func (f *fleet) Erase(ctx context.Context, key string) error {
	f.op.Lock()
	defer f.op.Unlock()
	if f.closed {
		return errStopping
	}
	f.mu.RLock()
	m := f.find(key)
	f.mu.RUnlock()
	if m == nil {
		return fmt.Errorf("%w: %s", nodes.ErrNotFound, key)
	}
	if m.s == nil {
		return fmt.Errorf("node %s: not started", key)
	}
	return m.s.erase(ctx)
}

// Warnings are every registry node's findings, each message under its
// key: a registry node's handler reports the default's Warnings
// (go-fabric-ui fabricui/handler.go:401 at v0.7.1), so every status page
// shows them.
func (f *fleet) Warnings(ctx context.Context) []fabricui.Warning {
	f.mu.RLock()
	defer f.mu.RUnlock()
	var out []fabricui.Warning
	for _, m := range f.members {
		if m.s == nil {
			continue
		}
		for _, w := range m.s.Warnings(ctx) {
			w.Message = "node " + m.rec.Key + ": " + w.Message
			out = append(out, w)
		}
	}
	return out
}

// Discards is the sum of every registry node's [served.Discards]; a
// deleted node's leave with it.
func (f *fleet) Discards() uint64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	var n uint64
	for _, m := range f.members {
		if m.s != nil {
			n += m.s.Discards()
		}
	}
	return n
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
