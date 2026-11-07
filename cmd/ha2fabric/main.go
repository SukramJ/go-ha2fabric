// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

// Command ha2fabric is the daemon: Home Assistant's entities, read over
// its WebSocket API, served as the bridged endpoints of go-fabric nodes to
// Matter controllers (Apple Home, Google Home, Amazon Alexa): the
// configured node, and the nodes the web UI creates beside it, each with
// its own selection (internal/nodes). In the
// Home Assistant add-on it reaches Core through the Supervisor's proxy
// with SUPERVISOR_TOKEN; stand-alone it takes HA_URL and HA_TOKEN.
//
// --dump-plan prints every candidate entity with the exposure rule's word
// and go-hamapping's verdict, and exits.
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"text/tabwriter"
	"time"

	fabricui "github.com/SukramJ/go-fabric-ui/fabricui"

	"github.com/SukramJ/go-ha2fabric/internal/camera"
	"github.com/SukramJ/go-ha2fabric/internal/config"
	"github.com/SukramJ/go-ha2fabric/internal/ha"
	"github.com/SukramJ/go-ha2fabric/internal/nodes"
	"github.com/SukramJ/go-ha2fabric/internal/plan"
	"github.com/SukramJ/go-ha2fabric/internal/selection"
	"github.com/SukramJ/go-ha2fabric/internal/version"
	"github.com/SukramJ/go-ha2fabric/internal/web"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], config.OSEnv, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

// events are the subscriptions the bridge reads.
var events = []string{ha.EventStateChanged, ha.EventEntityRegistryUpdated, ha.EventDeviceRegistryUpdated, ha.EventLabelRegistryUpdated}

func run(ctx context.Context, args []string, env config.Env, stdout, stderr io.Writer) int {
	cfg, err := config.Load(args, env, stderr)
	if config.IsHelp(err) {
		return 0
	}
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 2
	}
	if cfg.Version {
		_, _ = fmt.Fprintln(stdout, version.String())
		return 0
	}
	// LOG_LEVEL is the level at start; the web UI's Operation page moves
	// it at run time (lvl gates the ring and stderr alike).
	lvl := new(slog.LevelVar)
	lvl.Set(cfg.LogLevel)
	logger, ring := web.NewLogger(stderr, lvl)
	started := time.Now()
	client := ha.New(ha.Options{URL: cfg.HAURL, Token: cfg.HAToken, Events: events, Logger: logger})
	sel, err := selection.Open(cfg.SelectionPath, logger)
	if err != nil {
		logger.Error("ha2fabric.selection", slog.String("error", err.Error()))
		return 1
	}
	// The configured node's plan input: its rule with the web UI's
	// overrides, read at every call, and the dry camera ports.
	input := func() plan.Input { return plan.Effective(cfg.Exposure(), sel, cfg.Camera, camera.DryPorts) }
	if cfg.DumpPlan {
		return dumpPlan(ctx, client, input, stdout, logger)
	}
	logger.Info("ha2fabric.start", slog.String("version", version.String()), slog.String("ha_url", cfg.HAURL), slog.Bool("addon", cfg.Addon), slog.Bool("camera", cfg.Camera))

	reg, err := nodes.Open(filepath.Dir(cfg.StorePath), logger)
	if err != nil {
		logger.Error("ha2fabric.nodes", slog.String("error", err.Error()))
		return 1
	}

	// The cookbook's order (docs/guides/embed-in-a-daemon.md §6): every
	// node starts before the backend runs and stops after it. The one Home
	// Assistant connection fans out to every node's bridge (concept U6,
	// C.3): the configured node's and the registry's.
	rctx, cancel := context.WithCancel(ctx)
	defer cancel()
	fan := &ha.Fanout{}
	def, err := serve(rctx, client, fan, cfg.Node(logger), input, sel, logger)
	if err != nil {
		logger.Error("ha2fabric.node", slog.String("error", err.Error()))
		return 1
	}
	fl := newFleet(rctx, cfg, client, fan, reg, logger)

	var wg sync.WaitGroup
	wg.Go(func() { _ = client.Run(rctx, fan) })
	webErr := make(chan error, 1)
	if cfg.WebListen != "" {
		ws := web.New(web.Options{Listen: cfg.WebListen, Allow: cfg.WebAllow, Logger: logger, UI: fabricui.Options{
			Node: def.n, Exposure: def.prov, Admin: admin(cfg), Reassemble: def.br.Resync, Events: def.events, Warnings: def.prov.Warnings, Logger: logger,
			Operations: web.NewOperations(ring, version.Version, started),
			Registry:   fl, NodeConfig: nodeConfig(cfg),
		}})
		wg.Go(func() { webErr <- ws.Run(rctx) })
	}
	code := 0
	select {
	case <-ctx.Done():
	case err := <-webErr:
		if err != nil {
			logger.Error("ha2fabric.web", slog.String("error", err.Error()))
			code = 1
		}
	}
	logger.Info("ha2fabric.stopping")
	cancel()
	wg.Wait()
	sctx, scancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer scancel()
	fl.Close(sctx)
	if err := def.close(sctx); err != nil {
		logger.Warn("ha2fabric.node_stop", slog.String("error", err.Error()))
	}
	return code
}

// nodeConfig is the configured node's configuration block on its status
// page (go-fabric-ui contract 0.6.0, "The configuration block"): where
// its values come from — the add-on options or the flags — and the
// values, never the passcode.
func nodeConfig(cfg *config.Config) *fabricui.NodeConfigView {
	note := "the flags and environment (--listen/LISTEN, --discriminator/DISCRIMINATOR, --vendor-id, --product-id, --label, --store-path)"
	if cfg.Addon {
		note = "the add-on options (port, discriminator, vendor_id, product_id, label)"
	}
	return &fabricui.NodeConfigView{
		Source: fabricui.NodeConfigSourceConfiguration, SourceNote: note,
		Label: cfg.Label, Listen: cfg.Listen, Discriminator: cfg.Discriminator,
		VendorID: cfg.VendorID, VendorName: config.VendorName,
		ProductID: cfg.ProductID, ProductName: config.ProductName,
		Store: cfg.StorePath,
	}
}

// admin is the web UI's administrator check: in the add-on the Ingress
// proxy, which has authenticated the Home Assistant user; stand-alone
// read-only unless WEB_ADMIN, which selects the UI's token mode (nil).
func admin(cfg *config.Config) func(*http.Request) bool {
	switch {
	case cfg.Addon:
		return web.From(config.IngressProxy)
	case cfg.WebAdmin:
		return nil
	default:
		return web.ReadOnly
	}
}

// waiter is the handler of a one-shot connection: --dump-plan.
type waiter struct {
	once sync.Once
	ok   chan struct{}
	mu   sync.Mutex
	err  error
}

func (w *waiter) Connected(string) { w.once.Do(func() { close(w.ok) }) }
func (w *waiter) Event(ha.Event)   {}
func (w *waiter) Disconnected(err error) {
	w.mu.Lock()
	w.err = err
	w.mu.Unlock()
}

// dumpPlan connects once, reads everything and prints every candidate's
// verdict under the configured node's input.
func dumpPlan(ctx context.Context, client *ha.Client, input func() plan.Input, out io.Writer, logger *slog.Logger) int {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	w := &waiter{ok: make(chan struct{})}
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = client.Run(ctx, w)
	}()
	defer func() {
		cancel()
		<-done
	}()
	select {
	case <-w.ok:
	case <-ctx.Done():
		w.mu.Lock()
		err := w.err
		w.mu.Unlock()
		if err == nil {
			err = ctx.Err()
		}
		logger.Error("ha2fabric.dump_plan", slog.String("error", err.Error()))
		return 1
	}
	in := input()
	snap, err := ha.ReadSnapshot(ctx, client, in.Rule.Assist)
	if err != nil {
		logger.Error("ha2fabric.dump_plan", slog.String("error", err.Error()))
		return 1
	}
	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 2, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ENTITY\tDEVICE\tLABELS\tEXPOSED\tSOURCE\tWHY\tROW\tVERDICT\tREASON")
	for _, l := range in.Rule.Unresolved(plan.Labels(snap)) {
		logger.Warn("ha2fabric.label_unresolved", slog.String("label", l))
	}
	verdicts := plan.Dry(snap, in)
	for i := range verdicts {
		v := &verdicts[i]
		exposed := "no"
		if v.Exposed {
			exposed = "yes"
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", v.EntityID, v.Device, strings.Join(v.Labels, ","), exposed, v.Source, v.ExposeReason, v.Row, v.State, v.Reason)
	}
	_ = tw.Flush()
	if _, err := out.Write(buf.Bytes()); err != nil && !errors.Is(err, io.ErrClosedPipe) {
		return 1
	}
	return 0
}
