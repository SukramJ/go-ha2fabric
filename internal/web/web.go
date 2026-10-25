// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

// Package web serves the daemon's web interface (through Home
// Assistant's Ingress in the add-on): go-fabric-ui's handler
// (fabricui.Handler at v0.1.0: the node pages — health, pairing and the
// commissioning window, fabrics, the entity selection, endpoints,
// diagnostics — and their API under /api/v1), behind the client filter.
// The 0.1.0 JSON routes GET /api/health and GET /api/plan are gone (owner
// decision 2026-10-25): /api/v1/node/status and /api/v1/node/exposure
// replace them. The UI's URLs are relative, so it
// works under Ingress's path prefix (go-fabric-ui docs/embedding.md,
// "Behind an Ingress").
package web

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"time"

	fabricui "github.com/SukramJ/go-fabric-ui/fabricui"
)

// Options configure the server.
type Options struct {
	// Listen is the address; Allow restricts the clients (empty: all).
	Listen string
	Allow  []string
	Logger *slog.Logger
	// UI are go-fabric-ui's options (Node, Exposure, Admin, Reassemble,
	// Events); their Logger defaults to Logger.
	UI fabricui.Options
}

// Server is the web UI.
type Server struct {
	opts  Options
	log   *slog.Logger
	allow []net.IP
}

// New returns the server; [Server.Run] binds it.
func New(opts Options) *Server {
	log := opts.Logger
	if log == nil {
		log = slog.Default()
	}
	if opts.UI.Logger == nil {
		opts.UI.Logger = log
	}
	s := &Server{opts: opts, log: log}
	for _, a := range opts.Allow {
		if ip := net.ParseIP(a); ip != nil {
			s.allow = append(s.allow, ip)
		}
	}
	return s
}

// Handler is go-fabric-ui's routes behind the client filter.
func (s *Server) Handler() http.Handler {
	return s.filter(fabricui.Handler(s.opts.UI))
}

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

// From is an Admin that admits a request whose client address is ip: the
// add-on's Ingress proxy, which has authenticated the Home Assistant user
// before it forwards (go-fabric-ui docs/embedding.md, "Admin gate by the
// host").
func From(ip string) func(*http.Request) bool {
	want := net.ParseIP(ip)
	return func(r *http.Request) bool {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		got := net.ParseIP(host)
		return err == nil && got != nil && want != nil && got.Equal(want)
	}
}

// ReadOnly is an Admin that admits no request: the stand-alone UI without
// WEB_ADMIN.
func ReadOnly(*http.Request) bool { return false }
