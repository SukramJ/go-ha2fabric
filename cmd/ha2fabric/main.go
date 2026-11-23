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
	"slices"
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
	logger.Info("ha2fabric.start", slog.String("version", version.String()), slog.String("ha_url", ha.LogURL(cfg.HAURL)), slog.Bool("addon", cfg.Addon), slog.Bool("camera", cfg.Camera))
	startup := startWarnings(cfg, logger, started)
	// In the add-on the web UI has an administrator only on a Supervisor
	// that names the Ingress user's groups (supervisorGate).
	var gate *supervisorGate
	if cfg.Addon {
		gate = newSupervisorGate(cfg.SupervisorToken, logger)
	}

	reg, err := nodes.Open(filepath.Dir(cfg.StorePath), logger)
	if err != nil {
		logger.Error("ha2fabric.nodes", slog.String("error", err.Error()))
		return 1
	}
	if cfg.Passcode == 0 {
		// No PASSCODE: the random one beside the store, as a registry
		// node's (decision 14 of the 2026-11-23 review).
		p, created, err := reg.ConfiguredPasscode()
		if err != nil {
			logger.Error("ha2fabric.passcode", slog.String("error", err.Error()))
			return 1
		}
		cfg.Passcode = p
		if created {
			logger.Info("ha2fabric.passcode_created", slog.String("file", reg.Path(nodes.PasscodeFile)),
				slog.String("hint", "PASSCODE is unset, so the configured node got a random passcode, kept in this file; the pairing codes follow in ha2fabric.pairing and are on the web UI's pairing page"))
		}
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
	if gate != nil {
		wg.Go(func() { gate.run(rctx) })
	}
	webErr := make(chan error, 1)
	if cfg.WebListen != "" {
		ws, err := web.New(web.Options{Listen: cfg.WebListen, Allow: cfg.WebAllow, Logger: logger, UI: fabricui.Options{
			Node: def.sw, Exposure: def.prov, Admin: admin(cfg, gate), Reassemble: def.Resync, Events: def.events, Warnings: warnings(def, fl, hostWarnings(startup, gate)), Counters: counters(def, fl), Erase: def.erase, Logger: logger,
			Operations: web.NewOperations(ring, version.Version, started),
			Registry:   fl, NodeConfig: nodeConfig(cfg),
			AllowedHosts: cfg.WebAllowedHosts, AllowHost: allowHost(cfg),
			// The streams end when the daemon stops, before the server's
			// shutdown waits for its connections.
			Done: rctx.Done(),
		}})
		if err != nil {
			webErr <- err
		} else {
			wg.Go(func() { webErr <- ws.Run(rctx) })
		}
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

// warnings is the web UI's Options.Warnings: the host's (startWarnings),
// the configured node's findings and the registry nodes', which every
// node's status page reports (a registry node's handler inherits the
// default's, go-fabric-ui fabricui/handler.go:401 at v0.7.1).
func warnings(def *served, fl *fleet, host func() []fabricui.Warning) func(context.Context) []fabricui.Warning {
	return func(ctx context.Context) []fabricui.Warning {
		out := host()
		out = append(out, def.Warnings(ctx)...)
		return append(out, fl.Warnings(ctx)...)
	}
}

// hostWarnings are the start-up warnings and, in the add-on, the
// Supervisor gate's.
func hostWarnings(startup []fabricui.Warning, gate *supervisorGate) func() []fabricui.Warning {
	return func() []fabricui.Warning {
		out := slices.Clone(startup)
		if gate != nil {
			out = append(out, gate.Warnings()...)
		}
		return out
	}
}

// discardsMessage is the discards counter's line on the status page.
const discardsMessage = "Controller commands answered with success that called nothing (an Off on a scene's, script's, automation's or button's plug)"

// counters is the web UI's Options.Counters (go-fabric-ui contract
// 0.8.0): host-wide as warnings are — every node's status reads the
// default's list (go-fabric-ui fabricui/handler.go:393 at v0.8.0) —, so discards is the sum
// over the configured node's bridge and the registry nodes' (the
// bridge.command_discarded lines, docs/operations.md).
func counters(def *served, fl *fleet) func(context.Context) []fabricui.Counter {
	return func(context.Context) []fabricui.Counter {
		return []fabricui.Counter{{Code: "discards", Value: def.Discards() + fl.Discards(), Message: discardsMessage}}
	}
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

// admin is the web UI's administrator check. In the add-on: a request of
// the Ingress proxy whose Home Assistant user is an administrator
// (web.IngressAdmin: the user's groups, which the Supervisor sets), and
// only once gate knows the Supervisor sets them (minSupervisor); the
// proxy forwards every user's request, so its address alone decides
// nothing. Stand-alone the status page only unless WEB_ADMIN, which
// selects the UI's token mode (nil).
func admin(cfg *config.Config, gate *supervisorGate) func(*http.Request) bool {
	switch {
	case cfg.Addon:
		if gate == nil {
			return web.ReadOnly
		}
		ingress := web.IngressAdmin(config.IngressProxy)
		return func(r *http.Request) bool { return gate.open() && ingress(r) }
	case cfg.WebAdmin:
		return nil
	default:
		return web.ReadOnly
	}
}

// allowHost is the web UI's Host check beyond loopback names, IP
// addresses and WEB_ALLOWED_HOSTS: in the add-on every request the Ingress
// proxy forwards, whose Host is the browser's — Home Assistant's own name,
// which the add-on cannot know (core homeassistant/components/hassio/
// ingress.py:268-272 at 2026.10.3, Supervisor supervisor/api/ingress.py:
// 400-427 at fe91b8db; notes/reads/read-2026-11-23-ingress-host-facts.md);
// stand-alone none.
func allowHost(cfg *config.Config) func(*http.Request) bool {
	if cfg.Addon {
		return web.From(config.IngressProxy)
	}
	return nil
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
