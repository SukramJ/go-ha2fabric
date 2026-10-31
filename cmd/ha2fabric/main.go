// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

// Command ha2fabric is the daemon: Home Assistant's entities, read over
// its WebSocket API, served as the bridged endpoints of one go-fabric node
// to Matter controllers (Apple Home, Google Home, Amazon Alexa). In the
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
	"strings"
	"sync"
	"syscall"
	"text/tabwriter"
	"time"

	fabricui "github.com/SukramJ/go-fabric-ui/fabricui"
	"github.com/SukramJ/go-fabric/node"

	"github.com/SukramJ/go-ha2fabric/internal/bridge"
	"github.com/SukramJ/go-ha2fabric/internal/camera"
	"github.com/SukramJ/go-ha2fabric/internal/config"
	"github.com/SukramJ/go-ha2fabric/internal/ha"
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
	logger := slog.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: cfg.LogLevel}))
	client := ha.New(ha.Options{URL: cfg.HAURL, Token: cfg.HAToken, Events: events, Logger: logger})
	sel, err := selection.Open(cfg.SelectionPath, logger)
	if err != nil {
		logger.Error("ha2fabric.selection", slog.String("error", err.Error()))
		return 1
	}
	if cfg.DumpPlan {
		return dumpPlan(ctx, client, cfg, sel, stdout, logger)
	}
	logger.Info("ha2fabric.start", slog.String("version", version.String()), slog.String("ha_url", cfg.HAURL), slog.Bool("addon", cfg.Addon), slog.Bool("camera", cfg.Camera))

	// The cookbook's order (docs/guides/embed-in-a-daemon.md §6): the node
	// starts before the backend runs and stops after it.
	// The web UI's event stream observes the node itself (Node.Observe,
	// go-fabric-ui v0.1.2); uiEvents carries the bridge's own events.
	uiEvents := fabricui.NewEvents()
	n, err := node.New(bridge.NodeConfig(cfg, logger)) //nolint:contextcheck // the node's constructor takes no context
	if err != nil {
		logger.Error("ha2fabric.node", slog.String("error", err.Error()))
		return 1
	}
	if err := n.Start(ctx); err != nil {
		_ = n.Stop(context.WithoutCancel(ctx))
		logger.Error("ha2fabric.node_start", slog.String("error", err.Error()))
		return 1
	}
	bridge.LogFabrics(ctx, n, logger)
	bridge.LogPairing(n, logger)
	br := bridge.New(n, client, bridge.Options{
		Rule: cfg.Exposure(), Logger: logger, Selection: sel, Camera: cfg.Camera, CameraUpstream: camera.HA(client),
		OnChange: func(exposure []string, endpoints bool) {
			if len(exposure) > 0 {
				uiEvents.Publish(fabricui.Event{Type: fabricui.EventExposureChanged, Data: fabricui.ExposureEvent{Keys: exposure}})
			}
			if endpoints {
				uiEvents.Publish(fabricui.Event{Type: fabricui.EventEndpointsChanged, Data: struct{}{}})
			}
		},
	})

	var wg sync.WaitGroup
	rctx, cancel := context.WithCancel(ctx)
	wg.Go(func() { _ = br.Run(rctx) })
	wg.Go(func() { _ = client.Run(rctx, br) })
	webErr := make(chan error, 1)
	if cfg.WebListen != "" {
		ws := web.New(web.Options{Listen: cfg.WebListen, Allow: cfg.WebAllow, Logger: logger, UI: fabricui.Options{
			Node: n, Exposure: br.Exposure(), Admin: admin(cfg), Reassemble: br.Resync, Events: uiEvents, Warnings: br.Warnings, Logger: logger,
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
	if err := n.Stop(sctx); err != nil {
		logger.Warn("ha2fabric.node_stop", slog.String("error", err.Error()))
	}
	return code
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
// verdict.
func dumpPlan(ctx context.Context, client *ha.Client, cfg *config.Config, sel *selection.Store, out io.Writer, logger *slog.Logger) int {
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
	snap, err := bridge.Read(ctx, client, cfg.Exposure())
	if err != nil {
		logger.Error("ha2fabric.dump_plan", slog.String("error", err.Error()))
		return 1
	}
	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 2, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ENTITY\tDEVICE\tLABELS\tEXPOSED\tSOURCE\tWHY\tROW\tVERDICT\tREASON")
	rule := cfg.Exposure()
	rule.Overrides = sel.Enabled()
	names := map[string]string{}
	for id, e := range sel.Entries() {
		if e.Name != "" {
			names[id] = e.Name
		}
	}
	for _, l := range rule.Unresolved(snap.LabelRegistry()) {
		logger.Warn("ha2fabric.label_unresolved", slog.String("label", l))
	}
	verdicts := bridge.Dry(snap, rule, names, cfg.Camera)
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
