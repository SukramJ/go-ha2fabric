// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package main

import (
	"bufio"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	fabricui "github.com/SukramJ/go-fabric-ui/fabricui"

	"github.com/SukramJ/go-ha2fabric/internal/config"
	"github.com/SukramJ/go-ha2fabric/internal/web"
)

// adminRoutes are go-fabric-ui's administrator operations, read from its
// contract (contract/openapi.yaml, every operation with x-admin: true) at
// the version go.mod pins, so a route the UI adds joins the table by
// itself. A path parameter takes a sample value.
func adminRoutes(t *testing.T) []string {
	t.Helper()
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/SukramJ/go-fabric-ui").Output()
	if err != nil {
		t.Fatalf("go list go-fabric-ui: %v", err)
	}
	f, err := os.Open(filepath.Join(strings.TrimSpace(string(out)), "contract", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	sample := strings.NewReplacer("{index}", "1", "{ep}", "1", "{node}", "x", "{id}", "1")
	var routes []string
	var path, method string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "  /") && strings.HasSuffix(line, ":"):
			path, method = strings.TrimSuffix(strings.TrimSpace(line), ":"), ""
		case strings.HasPrefix(line, "    ") && !strings.HasPrefix(line, "     ") && strings.HasSuffix(line, ":"):
			method = strings.ToUpper(strings.TrimSuffix(strings.TrimSpace(line), ":"))
		case strings.TrimSpace(line) == "x-admin: true" && path != "" && method != "":
			routes = append(routes, method+" /api/v1"+sample.Replace(path))
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if len(routes) < 40 {
		t.Fatalf("only %d administrator routes in the contract: the parser lost its way", len(routes))
	}
	return routes
}

// TestAdminTable (HR-P1-test-16): every administrator route of
// go-fabric-ui's contract, as each kind of client, through the web server
// as the daemon builds it (webOptions: the client filter, the
// administrator check, the Host check). Only the add-on's Ingress request
// of a Home Assistant administrator, on a recent Supervisor, is not
// refused (403; a route this host does not mount is 404 to everyone); stand-alone without WEB_ADMIN nobody is the administrator, and
// with it no request without the UI's token.
func TestAdminTable(t *testing.T) {
	routes := adminRoutes(t)
	quiet := slog.New(slog.DiscardHandler)
	load := func(env map[string]string) *config.Config {
		env["STORE_PATH"] = filepath.Join(t.TempDir(), "ha2fabric.db")
		cfg, err := config.Load(nil, config.MapEnv(env), io.Discard)
		if err != nil {
			t.Fatal(err)
		}
		return cfg
	}
	handler := func(cfg *config.Config, gate *supervisorGate) http.Handler {
		s, err := web.New(webOptions(cfg, gate, quiet, fabricui.Options{Logger: quiet}))
		if err != nil {
			t.Fatal(err)
		}
		return s.Handler()
	}
	type client struct {
		name, remote, host, groups string
		admin                      bool
	}
	check := func(t *testing.T, h http.Handler, clients []client) {
		t.Helper()
		for _, c := range clients {
			for _, rt := range routes {
				m, p, _ := strings.Cut(rt, " ")
				req := httptest.NewRequest(m, p, strings.NewReader("{}"))
				req.Host, req.RemoteAddr = c.host, c.remote
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set(fabricui.RequestHeader, "1")
				if c.groups != "" {
					req.Header.Set(web.UserGroupsHeader, c.groups)
				}
				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)
				// 404: a route this host does not mount (the controller's),
				// refused to everyone alike.
				refused := rec.Code == http.StatusForbidden || rec.Code == http.StatusNotFound
				if c.admin && rec.Code == http.StatusForbidden || !c.admin && !refused {
					t.Errorf("%s: %s answered %d", c.name, rt, rec.Code)
				}
			}
		}
	}
	proxy := config.IngressProxy + ":4000"

	t.Run("add-on", func(t *testing.T) {
		cfg := load(map[string]string{config.SupervisorTokenEnv: "s"})
		gate := &supervisorGate{}
		gate.state.Store(int32(gateOpen))
		check(t, handler(cfg, gate), []client{
			{"a LAN peer naming the administrator group", "192.168.1.20:4000", "homeassistant.local:8123", "system-admin", false},
			{"another add-on naming the administrator group", "172.30.33.5:4000", "homeassistant.local:8123", "system-admin", false},
			{"the proxy without the groups header", proxy, "homeassistant.local:8123", "", false},
			{"the proxy for a user", proxy, "homeassistant.local:8123", "system-users", false},
			{"the proxy for an administrator", proxy, "homeassistant.local:8123", "system-users,system-admin", true},
		})
	})
	t.Run("add-on on an old Supervisor", func(t *testing.T) {
		cfg := load(map[string]string{config.SupervisorTokenEnv: "s"})
		check(t, handler(cfg, &supervisorGate{}), []client{
			{"the proxy for an administrator", proxy, "homeassistant.local:8123", "system-admin", false},
		})
	})
	t.Run("stand-alone", func(t *testing.T) {
		cfg := load(map[string]string{"HA_URL": "ws://127.0.0.1:8123/api/websocket", "HA_TOKEN": "t"})
		check(t, handler(cfg, nil), []client{
			{"loopback", "127.0.0.1:4000", "127.0.0.1:8099", "", false},
			{"loopback naming the administrator group", "127.0.0.1:4000", "127.0.0.1:8099", "system-admin", false},
		})
	})
	t.Run("stand-alone with WEB_ADMIN, without the token", func(t *testing.T) {
		cfg := load(map[string]string{"HA_URL": "ws://127.0.0.1:8123/api/websocket", "HA_TOKEN": "t", "WEB_ADMIN": "true"})
		check(t, handler(cfg, nil), []client{
			{"loopback without the token", "127.0.0.1:4000", "127.0.0.1:8099", "", false},
		})
	})
}
