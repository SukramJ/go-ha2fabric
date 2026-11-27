// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"thingfabric.dev/go-ha2fabric/internal/config"
	"thingfabric.dev/go-ha2fabric/internal/web"
)

// healthTimeout bounds the whole probe: the connection, the request and
// the answer.
const healthTimeout = 3 * time.Second

// The probe's lines on stderr. Each is fixed: none names the address, the
// answer's body or any configuration value.
const (
	healthNoWeb      = "ha2fabric: health: not performable: WEB_LISTEN is empty, the web UI is off"
	healthNotLocal   = "ha2fabric: health: not performable: WEB_LISTEN names a non-loopback address, and /healthz answers loopback clients only"
	healthAddon      = "ha2fabric: health: not performable in the add-on"
	healthBadListen  = "ha2fabric: health: not performable: WEB_LISTEN is not a host and port"
	healthUnreached  = "ha2fabric: health: the web UI is not reachable"
	healthNoHA       = "ha2fabric: health: not connected to Home Assistant"
	healthUnexpected = "ha2fabric: health: unexpected answer from the web UI"
)

// health is --health: it asks the running daemon's GET /healthz on the
// web listener of cfg (WEB_LISTEN) and returns 0 when the daemon answers
// 200 (Home Assistant connected), 1 otherwise or when the probe is not
// performable. It reads no credential, prints nothing on stdout and one
// fixed line on stderr per failure.
func health(ctx context.Context, cfg *config.Config, stderr io.Writer) int {
	target, why := healthTarget(cfg)
	if why != "" {
		_, _ = fmt.Fprintln(stderr, why)
		return 1
	}
	ctx, cancel := context.WithTimeout(ctx, healthTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+target+web.HealthPath, http.NoBody)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, healthBadListen)
		return 1
	}
	cl := &http.Client{
		Transport: &http.Transport{Proxy: nil},
		// A redirect is not the daemon's answer.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	res, err := cl.Do(req)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, healthUnreached)
		return 1
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 512))
	_ = res.Body.Close()
	switch res.StatusCode {
	case http.StatusOK:
		return 0
	case http.StatusServiceUnavailable:
		_, _ = fmt.Fprintln(stderr, healthNoHA)
	default:
		_, _ = fmt.Fprintln(stderr, healthUnexpected)
	}
	return 1
}

// healthTarget is the host and port the probe dials, from cfg's
// WEB_LISTEN: an empty or unspecified host (every interface) is dialled on
// loopback (127.0.0.1, ::1 for "::"), a loopback host or "localhost" as
// given. Else it is "" and the line saying why the probe is not
// performable: the add-on (its listener's address comes from the
// Supervisor at start, cmd/ha2fabric addonWebListen), no web UI, or a
// specific non-loopback host, from which the probe would not come as a
// loopback client.
func healthTarget(cfg *config.Config) (target, notPerformable string) {
	switch {
	case cfg.Addon:
		return "", healthAddon
	case cfg.WebListen == "":
		return "", healthNoWeb
	}
	host, port, err := net.SplitHostPort(cfg.WebListen)
	if err != nil || port == "" {
		return "", healthBadListen
	}
	ip := net.ParseIP(host)
	switch {
	case host == "" || ip != nil && ip.IsUnspecified() && ip.To4() != nil:
		host = "127.0.0.1"
	case ip != nil && ip.IsUnspecified():
		host = "::1"
	case host == "localhost" || ip != nil && ip.IsLoopback():
	default:
		return "", healthNotLocal
	}
	return net.JoinHostPort(host, port), ""
}
