// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/SukramJ/go-ha2fabric/internal/config"
)

// TestStartWarnings: a configuration the daemon accepts but warns about
// is a start-up log line and a status-page warning — the token over ws://
// to another host (HR-P0-ha-23).
func TestStartWarnings(t *testing.T) {
	web := freePort(t, "tcp")
	env := config.MapEnv(map[string]string{
		// TEST-NET-1 (RFC 5737): nothing answers, the daemon keeps trying.
		"HA_URL": "ws://192.0.2.1:8123/api/websocket", "HA_TOKEN": "t",
		"STORE_PATH": filepath.Join(t.TempDir(), "ha2fabric.db"), "LISTEN": freePort(t, "udp"), "WEB_LISTEN": web,
	})
	d := startDaemon(t, env, web)
	if st := d.status("/api/v1/node/status"); !st.warned(warningTokenInClear, "") {
		t.Fatalf("no %s: %+v", warningTokenInClear, st)
	}
	if !strings.Contains(d.logs.String(), "msg=ha2fabric.token_in_clear") {
		t.Fatalf("no log line\n%s", d.logs.String())
	}
	d.stop()
}
