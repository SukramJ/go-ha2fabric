// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

// Package web serves the daemon's one read-only page (through Home
// Assistant's Ingress in the add-on): the pairing QR code — rendered in
// the browser from the payload text by an embedded encoder, so no image
// dependency — and the manual code, the commissioned fabrics, the
// candidate entities with their verdicts and reasons, the Home Assistant
// connection and the node's health. GET /api/health and GET /api/plan
// serve the same as JSON. The page's URLs are relative, so it works under
// Ingress's path prefix.
package web

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"time"

	"github.com/SukramJ/go-fabric/node"

	"github.com/SukramJ/go-ha2fabric/internal/bridge"
	"github.com/SukramJ/go-ha2fabric/internal/version"
)

//go:embed assets
var assets embed.FS

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
	s := &Server{br: br, n: n, opts: opts, log: log}
	for _, a := range opts.Allow {
		if ip := net.ParseIP(a); ip != nil {
			s.allow = append(s.allow, ip)
		}
	}
	return s
}

// Health is GET /api/health.
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

// Handler is the routes behind the client filter.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/plan", s.plan)
	sub, err := fs.Sub(assets, "assets")
	if err != nil {
		panic("web: " + err.Error()) // a constant path of the embed
	}
	mux.Handle("GET /", http.FileServerFS(sub))
	return s.filter(mux)
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
	}
	if s.n != nil {
		if sample, ok := s.n.Health(); ok {
			h.Node = &NodeHealth{Healthy: sample.Healthy, Note: sample.Note}
			if !sample.Healthy {
				h.Problems = append(h.Problems, "node: "+sample.Note)
			}
		}
		if qr, manual, err := s.n.PairingCodes(); err == nil {
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
	srv := &http.Server{
		Addr: s.opts.Listen, Handler: s.Handler(),
		ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second,
		WriteTimeout: 30 * time.Second, IdleTimeout: 120 * time.Second,
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
