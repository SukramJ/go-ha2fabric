// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

// Package web serves the daemon's web interface (through Home
// Assistant's Ingress in the add-on): go-fabric-ui's handler
// (fabricui.Handler at v0.1.0: the node pages — health, pairing and the
// commissioning window, fabrics, the entity selection, endpoints,
// diagnostics, and the Operation page over the daemon's log ring
// ([Operations]) — and their API under /api/v1), behind the client filter.
// The 0.1.0 JSON routes GET /api/health and GET /api/plan are gone (owner
// decision 2026-10-25): /api/v1/node/status and /api/v1/node/exposure
// replace them. GET /healthz (owner, 2026-11-27, #71) is the daemon's
// own, ahead of the client filter and go-fabric-ui's handler, and answers
// a loopback client under a loopback Host only: 200 "ok" while Home Assistant is connected, 503
// "unavailable" otherwise — what --health probes. The UI's URLs are relative, so it
// works under Ingress's path prefix (go-fabric-ui docs/embedding.md,
// "Behind an Ingress", at v0.16.0). Since go-fabric-ui v0.10 the handler checks every
// request's Host: loopback names and IP addresses pass, a DNS name only
// when it is in UI.AllowedHosts (WEB_ALLOWED_HOSTS) or UI.AllowHost admits
// the request — in the add-on the Ingress proxy's source address, since
// the proxy forwards the browser's Host unchanged
// (notes/reads/read-2026-11-23-ingress-host-facts.md).
package web

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"strings"
	"time"

	fabricui "thingfabric.dev/go-fabric-ui/fabricui"
)

// Options configure the server.
type Options struct {
	// Listen is the address; Allow restricts the clients (empty: all).
	Listen string
	Allow  []string
	// Loopback refuses, in Run, a listener that did not bind a loopback
	// address: the configuration judged WEB_LISTEN loopback by its text
	// ("localhost" by name), the bound address is what counts.
	Loopback bool
	// Healthy, when set, serves GET /healthz to a loopback client ahead of
	// the client filter ([Server.Handler]): 200 "ok" while it reports
	// true, 503 "unavailable" otherwise. nil serves no such route.
	Healthy func() bool
	Logger  *slog.Logger
	// UI are go-fabric-ui's options (Node, Exposure, Admin, Reassemble,
	// Events, Operations — [NewOperations] over [NewLogger]'s ring); their
	// Logger defaults to Logger.
	UI fabricui.Options
}

// Server is the web UI.
type Server struct {
	opts  Options
	log   *slog.Logger
	allow []net.IP
	h     http.Handler
}

// New returns the server; [Server.Run] binds it. It fails on an Allow
// entry that is not an IP address and on options go-fabric-ui refuses,
// such as an AllowedHosts entry with a port.
func New(opts Options) (*Server, error) {
	log := opts.Logger
	if log == nil {
		log = slog.Default()
	}
	if opts.UI.Logger == nil {
		opts.UI.Logger = log
	}
	s := &Server{opts: opts, log: log}
	for _, a := range opts.Allow {
		ip := net.ParseIP(a)
		if ip == nil {
			// Dropping it could leave no entry, which serves everyone.
			return nil, fmt.Errorf("web: Allow entry %q is not an IP address", a)
		}
		s.allow = append(s.allow, ip)
	}
	h, err := fabricui.NewHandler(opts.UI)
	if err != nil {
		return nil, fmt.Errorf("web: %w", err)
	}
	s.h = s.health(s.filter(h))
	return s, nil
}

// Handler is go-fabric-ui's routes behind the client filter, and
// GET /healthz ahead of it.
func (s *Server) Handler() http.Handler { return s.h }

// filter refuses a client outside Allow: the add-on runs on the host's
// network, and Ingress requires that only its proxy (172.30.32.2) reaches
// the page (developers.home-assistant.io docs/apps/presentation.md
// "Ingress" at 50b5188).
func (s *Server) filter(next http.Handler) http.Handler {
	if len(s.allow) == 0 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		ip := net.ParseIP(host)
		if err != nil || ip == nil || !slices.ContainsFunc(s.allow, ip.Equal) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// HealthPath is the route [Options.Healthy] answers.
const HealthPath = "/healthz"

// health answers GET and HEAD of HealthPath from a loopback client under
// a loopback Host (localhost or a loopback IP literal) with Healthy; every other request, and every request when Healthy is nil,
// goes to next unchanged. It stands ahead of the client filter: a probe
// inside the container or on the host comes from loopback, which WEB_ALLOW
// need not name.
func (s *Server) health(next http.Handler) http.Handler {
	if s.opts.Healthy == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != HealthPath || r.Method != http.MethodGet && r.Method != http.MethodHead || !loopbackClient(r) || !loopbackHost(r.Host) {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if s.opts.Healthy() {
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "ok\n")
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, "unavailable\n")
	})
}

// loopbackClient reports whether r's client address is a loopback one.
func loopbackClient(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	ip := net.ParseIP(host)
	return err == nil && ip != nil && ip.IsLoopback()
}

// loopbackHost reports whether host, its port stripped, is localhost or
// a loopback IP literal (an IPv6 one in brackets).
func loopbackHost(host string) bool {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	} else if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		host = host[1 : len(host)-1]
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// Run serves until ctx ends.
func (s *Server) Run(ctx context.Context) error {
	// No WriteTimeout: the UI's event stream (GET /api/v1/node/stream)
	// is a response that stays open; the handler's heartbeat ends it on a
	// dead client.
	srv := &http.Server{
		Addr: s.opts.Listen, Handler: s.Handler(),
		ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second,
		IdleTimeout: 120 * time.Second,
	}
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", s.opts.Listen)
	if err != nil {
		return fmt.Errorf("web: %w", err)
	}
	if a, ok := ln.Addr().(*net.TCPAddr); s.opts.Loopback && (!ok || !a.IP.IsLoopback()) {
		_ = ln.Close()
		return fmt.Errorf("web: %s bound %s, which is not a loopback address; with WEB_ADMIN and no WEB_ALLOW the web UI serves loopback only", s.opts.Listen, ln.Addr())
	}
	s.log.Info("web.listening", slog.String("addr", ln.Addr().String()), slog.Int("allow", len(s.allow)))
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	select {
	case <-ctx.Done():
		sctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		return srv.Shutdown(sctx)
	case err := <-errc:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// From is a predicate that admits a request whose client address is ip:
// the add-on's Host check admits the Ingress proxy by it. The proxy's
// address says who sent the request, not who the user is; the add-on's
// administrator check is [IngressAdmin].
func From(ip string) func(*http.Request) bool {
	want := net.ParseIP(ip)
	return func(r *http.Request) bool {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		got := net.ParseIP(host)
		return err == nil && got != nil && want != nil && got.Equal(want)
	}
}

// Ingress user headers: the Supervisor sets the Home Assistant user's
// groups on every request it forwards through Ingress, comma-separated
// (supervisor/api/ingress.py:412-419, const.py:115, :625, :653-659 at
// fe91b8db; from Supervisor 2026.10.4).
const (
	UserGroupsHeader = "X-Remote-User-Groups"
	AdminGroup       = "system-admin"
)

// IngressAdmin is the add-on's Admin: a request from the Ingress proxy at
// proxy whose user is a Home Assistant administrator — UserGroupsHeader
// lists AdminGroup (an owner's groups include it). Without the header the
// request is no administrator's. The daemon applies it on a Supervisor of
// 2026.10.4 or later, which it reads at start (cmd/ha2fabric
// supervisorGate).
func IngressAdmin(proxy string) func(*http.Request) bool {
	from := From(proxy)
	return func(r *http.Request) bool {
		if !from(r) {
			return false
		}
		for _, v := range r.Header.Values(UserGroupsHeader) {
			for g := range strings.SplitSeq(v, ",") {
				if strings.TrimSpace(g) == AdminGroup {
					return true
				}
			}
		}
		return false
	}
}

// ReadOnly is an Admin that admits no request: the stand-alone UI without
// WEB_ADMIN, which since go-fabric-ui v0.10 serves the status page only —
// exposure, fabrics, endpoints, diagnostics, pairing and the operation log
// are administrator routes (owner, 2026-11-22).
func ReadOnly(*http.Request) bool { return false }
