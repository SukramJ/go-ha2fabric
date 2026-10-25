// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

// Package web serves the daemon's web interface (through Home
// Assistant's Ingress in the add-on): go-fabric-ui's handler
// (fabricui.Handler at v0.1.0: the node pages — health, pairing and the
// commissioning window, fabrics, the entity selection, endpoints,
// diagnostics — and their API under /api/v1), behind the client filter.
// GET /api/health and GET /api/plan, the JSON of 0.1.0, stay as aliases
// for the 0.2 releases and go in 0.3.0. The UI's URLs are relative, so it
// works under Ingress's path prefix (go-fabric-ui docs/embedding.md,
// "Behind an Ingress").
package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"time"

	fabricui "github.com/SukramJ/go-fabric-ui/fabricui"
	"github.com/SukramJ/go-fabric/node"

	"github.com/SukramJ/go-ha2fabric/internal/bridge"
	"github.com/SukramJ/go-ha2fabric/internal/version"
)

// Bridge is what the page reads of the bridge.
type Bridge interface {
	Status() bridge.Status
	Plan() []bridge.Verdict
}

// Node is what the page reads of the node; *node.Node is one.
type Node interface {
	PairingCodes() (qr, manual string, err error)
	FabricInfos(ctx context.Context) ([]node.FabricInfo, error)
	Health() (node.HealthSample, bool)
}

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
	br    Bridge
	n     Node
	opts  Options
	log   *slog.Logger
	allow []net.IP
}

// New returns the server; [Server.Run] binds it.
func New(br Bridge, n Node, opts Options) *Server {
	log := opts.Logger
	if log == nil {
		log = slog.Default()
	}
	if opts.UI.Logger == nil {
		opts.UI.Logger = log
	}
	s := &Server{br: br, n: n, opts: opts, log: log}
	for _, a := range opts.Allow {
		if ip := net.ParseIP(a); ip != nil {
			s.allow = append(s.allow, ip)
		}
	}
	return s
}

// Health is GET /api/health (0.1.0's form; the UI reads /api/v1).
type Health struct {
	Status   string        `json:"status"`
	Version  string        `json:"version"`
	Bridge   bridge.Status `json:"bridge"`
	Node     *NodeHealth   `json:"node,omitempty"`
	Pairing  *Pairing      `json:"pairing,omitempty"`
	Fabrics  []Fabric      `json:"fabrics"`
	Problems []string      `json:"problems,omitempty"`
}

// NodeHealth is the node's last health sample.
type NodeHealth struct {
	Healthy bool   `json:"healthy"`
	Note    string `json:"note,omitempty"`
}

// Pairing is the pairing codes (go-fabric cookbook §7).
type Pairing struct {
	QR     string `json:"qr"`
	Manual string `json:"manual"`
}

// Fabric is one commissioned fabric.
type Fabric struct {
	Index    uint8  `json:"index"`
	FabricID string `json:"fabric_id"`
	VendorID string `json:"vendor_id"`
	Label    string `json:"label,omitempty"`
}

// Handler is the routes behind the client filter: the two 0.1.0
// aliases, everything else go-fabric-ui's.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/plan", s.plan)
	mux.Handle("/", fabricui.Handler(s.opts.UI))
	return s.filter(mux)
}

// admin is whether the request may read a secret: the UI's Admin. In
// the UI's token mode (Admin nil) the alias shows no pairing code, which
// the UI hands out only against its token.
func (s *Server) admin(r *http.Request) bool {
	return s.opts.UI.Admin != nil && s.opts.UI.Admin(r)
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

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	h := Health{Status: "ok", Version: version.Version, Bridge: s.br.Status(), Fabrics: []Fabric{}}
	if !h.Bridge.HAConnected {
		h.Problems = append(h.Problems, "Home Assistant is not connected")
	} else if h.Bridge.LastSync.IsZero() {
		h.Problems = append(h.Problems, "Home Assistant is not read yet")
	}
	if s.n != nil {
		if sample, ok := s.n.Health(); ok {
			h.Node = &NodeHealth{Healthy: sample.Healthy, Note: sample.Note}
			if !sample.Healthy {
				h.Problems = append(h.Problems, "node: "+sample.Note)
			}
		}
		if qr, manual, err := s.n.PairingCodes(); err == nil && s.admin(r) {
			h.Pairing = &Pairing{QR: qr, Manual: manual}
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if fabrics, err := s.n.FabricInfos(ctx); err == nil {
			for _, f := range fabrics {
				h.Fabrics = append(h.Fabrics, Fabric{Index: f.FabricIndex, FabricID: fmt.Sprintf("%016X", f.FabricID), VendorID: fmt.Sprintf("0x%04X", f.VendorID), Label: f.Label})
			}
		} else {
			h.Problems = append(h.Problems, "fabrics: "+err.Error())
		}
	}
	code := http.StatusOK
	if len(h.Problems) > 0 {
		h.Status, code = "degraded", http.StatusServiceUnavailable
	}
	writeJSON(w, code, h)
}

func (s *Server) plan(w http.ResponseWriter, _ *http.Request) {
	p := s.br.Plan()
	if p == nil {
		p = []bridge.Verdict{}
	}
	writeJSON(w, http.StatusOK, p)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetIndent("", " ")
	_ = enc.Encode(v)
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
