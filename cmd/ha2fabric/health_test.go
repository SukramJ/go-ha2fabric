// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package main

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"net/http/httptest"
	"strings"
	"testing"

	fabricui "thingfabric.dev/go-fabric-ui/fabricui"

	"thingfabric.dev/go-ha2fabric/internal/config"
	"thingfabric.dev/go-ha2fabric/internal/web"
)

// healthToken is a set HA_TOKEN that must not reach the probe's output.
const healthToken = "health-test-token-c0ffee"

// probe runs the daemon's command line with --health under WEB_LISTEN
// listen and extra environment, without HA_URL: it needs none. An empty
// listen is --web-listen= (an empty WEB_LISTEN in the environment is the
// default). It checks
// the exit code and that stdout is empty, and that stderr is want (""
// for none) and holds neither listen nor HA_TOKEN.
func probe(t *testing.T, listen string, extra map[string]string, code int, want string) {
	t.Helper()
	env := map[string]string{"WEB_LISTEN": listen, "HA_TOKEN": healthToken}
	for k, v := range extra {
		env[k] = v
	}
	args := []string{"--health"}
	if listen == "" {
		args = append(args, "--web-listen=")
	}
	var stdout, stderr bytes.Buffer
	if got := run(context.Background(), args, config.MapEnv(env), &stdout, &stderr); got != code {
		t.Fatalf("WEB_LISTEN %q: exit %d, want %d; stderr %q", listen, got, code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout %q", stdout.String())
	}
	errs := stderr.String()
	if strings.TrimSpace(errs) != want {
		t.Fatalf("stderr %q, want %q", errs, want)
	}
	if listen != "" && strings.Contains(errs, listen) || strings.Contains(errs, healthToken) {
		t.Fatalf("stderr names the address or the token: %q", errs)
	}
}

// healthServer is the daemon's web handler with Healthy reporting up, on
// a loopback port; it returns the address.
func healthServer(t *testing.T, up bool) string {
	t.Helper()
	quiet := slog.New(slog.DiscardHandler)
	ws, err := web.New(web.Options{Healthy: func() bool { return up }, Logger: quiet, UI: fabricui.Options{Logger: quiet}})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(ws.Handler())
	t.Cleanup(srv.Close)
	return srv.Listener.Addr().String()
}

// TestHealthDaemonUp (#71): the daemon answers and Home Assistant is
// connected: exit 0, nothing printed.
func TestHealthDaemonUp(t *testing.T) {
	probe(t, healthServer(t, true), nil, 0, "")
}

// TestHealthListenerDown (#71): nothing listens on WEB_LISTEN: exit 1.
func TestHealthListenerDown(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	probe(t, addr, nil, 1, healthUnreached)
}

// TestHealthHomeAssistantAway (#71): the daemon answers 503 while Home
// Assistant is not connected: exit 1.
func TestHealthHomeAssistantAway(t *testing.T) {
	probe(t, healthServer(t, false), nil, 1, healthNoHA)
}

// TestHealthUnspecifiedHostDialsLoopback: WEB_LISTEN on every interface
// is probed on loopback.
func TestHealthUnspecifiedHostDialsLoopback(t *testing.T) {
	_, port, err := net.SplitHostPort(healthServer(t, true))
	if err != nil {
		t.Fatal(err)
	}
	probe(t, "0.0.0.0:"+port, nil, 0, "")
	probe(t, ":"+port, nil, 0, "")
}

// TestHealthNotPerformable: no web UI, a specific non-loopback host and
// the add-on are refused with exit 1 and their line, without a request.
func TestHealthNotPerformable(t *testing.T) {
	probe(t, "", nil, 1, healthNoWeb)
	probe(t, "192.0.2.10:8099", nil, 1, healthNotLocal)
	probe(t, "0.0.0.0:8486", map[string]string{config.SupervisorTokenEnv: "supervisor-test-token"}, 1, healthAddon)
}

// TestHealthTarget: the address the probe dials for each WEB_LISTEN.
func TestHealthTarget(t *testing.T) {
	for listen, want := range map[string]string{
		":8099":          "127.0.0.1:8099",
		"0.0.0.0:8099":   "127.0.0.1:8099",
		"[::]:8099":      "[::1]:8099",
		"127.0.0.2:8099": "127.0.0.2:8099",
		"[::1]:8099":     "[::1]:8099",
		"localhost:8099": "localhost:8099",
	} {
		if got, why := healthTarget(&config.Config{WebListen: listen}); got != want || why != "" {
			t.Errorf("WEB_LISTEN %q: %q %q, want %q", listen, got, why, want)
		}
	}
	for listen, want := range map[string]string{
		"":                healthNoWeb,
		"192.0.2.10:8099": healthNotLocal,
		"[2001:db8::1]:1": healthNotLocal,
		"example.org:80":  healthNotLocal,
		"8099":            healthBadListen,
	} {
		if got, why := healthTarget(&config.Config{WebListen: listen}); got != "" || why != want {
			t.Errorf("WEB_LISTEN %q: %q %q, want not performable %q", listen, got, why, want)
		}
	}
}
