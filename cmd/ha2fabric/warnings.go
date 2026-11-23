// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package main

import (
	"log/slog"
	"net/url"
	"time"

	fabricui "github.com/SukramJ/go-fabric-ui/fabricui"

	"github.com/SukramJ/go-ha2fabric/internal/config"
)

// Host-wide warning codes the daemon sets on every status page.
const (
	// warningTokenInClear: HA_URL is ws:// to another host (decision 17
	// of the 2026-11-23 review: a warning, not a refusal).
	warningTokenInClear = "token_in_clear"
)

// startWarnings are what the configuration holds that the daemon accepts
// but warns about: each is logged once here and shown on every status
// page while the daemon runs.
func startWarnings(cfg *config.Config, logger *slog.Logger, now time.Time) []fabricui.Warning {
	var out []fabricui.Warning
	if cfg.TokenInClear() {
		host := ""
		if u, err := url.Parse(cfg.HAURL); err == nil {
			host = u.Hostname()
		}
		logger.Warn("ha2fabric.token_in_clear", slog.String("host", host),
			slog.String("hint", "HA_URL is ws://, so the access token crosses the network unencrypted; use wss:// where Home Assistant has TLS, or run the daemon on Home Assistant's host (docs/operations.md, The token)"))
		out = append(out, fabricui.Warning{
			Code:    warningTokenInClear,
			Message: "HA_URL is ws:// to " + host + ": the access token crosses the network unencrypted; use wss:// where Home Assistant has TLS",
			Since:   now,
		})
	}
	return out
}
