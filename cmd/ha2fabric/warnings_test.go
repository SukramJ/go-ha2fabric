// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package main

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SukramJ/go-ha2fabric/internal/config"
	"github.com/SukramJ/go-ha2fabric/internal/ha/hatest"
	"github.com/SukramJ/go-ha2fabric/internal/nodes"
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

// TestPasscode: without PASSCODE the configured node's passcode is drawn
// on the first start and kept beside the store, the same at the next
// start; the public test value set explicitly is honoured with a log line
// and a status warning (HR-P0-cfg-21).
func TestPasscode(t *testing.T) {
	srv := hatest.New(t)
	dir := t.TempDir()
	envOf := func(web string, kv ...string) config.Env {
		m := map[string]string{
			"HA_URL": srv.URL(), "HA_TOKEN": hatest.Token, "STORE_PATH": filepath.Join(dir, "ha2fabric.db"),
			"LISTEN": freePort(t, "udp"), "WEB_LISTEN": web,
		}
		for i := 0; i < len(kv); i += 2 {
			m[kv[i]] = kv[i+1]
		}
		return config.MapEnv(m)
	}
	web := freePort(t, "tcp")
	d := startDaemon(t, envOf(web), web)
	d.stop()
	first, err := os.ReadFile(filepath.Join(dir, nodes.PasscodeFile))
	if err != nil || !strings.Contains(d.logs.String(), "msg=ha2fabric.passcode_created") {
		t.Fatalf("no passcode file (%v) or no log line\n%s", err, d.logs.String())
	}
	if strings.Contains(d.logs.String(), "ha2fabric.passcode_test_value") {
		t.Fatal("the random passcode warned as the test value")
	}

	web = freePort(t, "tcp")
	d = startDaemon(t, envOf(web), web)
	d.stop()
	again, err := os.ReadFile(filepath.Join(dir, nodes.PasscodeFile))
	if err != nil || !bytes.Equal(again, first) || strings.Contains(d.logs.String(), "ha2fabric.passcode_created") {
		t.Fatalf("the second start drew again: %v\n%s", err, d.logs.String())
	}

	web = freePort(t, "tcp")
	d = startDaemon(t, envOf(web, "PASSCODE", "20202021"), web)
	if st := d.status("/api/v1/node/status"); !st.warned(warningPasscodeTestValue, "") {
		t.Fatalf("no %s: %+v", warningPasscodeTestValue, st)
	}
	if !strings.Contains(d.logs.String(), "msg=ha2fabric.passcode_test_value") {
		t.Fatalf("no log line\n%s", d.logs.String())
	}
	d.stop()
}

// TestLogPairing: a node with a fabric, or whose fabrics could not be
// read, logs no pairing code, only where to find it (HR-P0-leak-24); the
// node without a fabric logs its codes (TestRun).
func TestLogPairing(t *testing.T) {
	for _, fabrics := range []int{1, 3, -1} {
		var logs safeBuffer
		logPairing(nil, fabrics, slog.New(slog.NewTextHandler(&logs, nil)))
		if out := logs.String(); !strings.Contains(out, "msg=ha2fabric.pairing") || strings.Contains(out, "manual_code") || strings.Contains(out, "qr_payload") || !strings.Contains(out, "pairing page") {
			t.Fatalf("fabrics %d: %s", fabrics, out)
		}
	}
}

// TestPasscodeNotKept: a passcode.json that cannot be read or written
// does not stop the daemon; it runs on a random passcode for this start
// and says so (HR-P0-cfg-54).
func TestPasscodeNotKept(t *testing.T) {
	srv := hatest.New(t)
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, nodes.PasscodeFile), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, nodes.PasscodeFile, "x"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	web := freePort(t, "tcp")
	d := startDaemon(t, config.MapEnv(map[string]string{
		"HA_URL": srv.URL(), "HA_TOKEN": hatest.Token, "STORE_PATH": filepath.Join(dir, "ha2fabric.db"),
		"LISTEN": freePort(t, "udp"), "WEB_LISTEN": web,
	}), web)
	if st := d.status("/api/v1/node/status"); !st.warned(warningPasscodeNotKept, "") {
		t.Fatalf("no %s: %+v", warningPasscodeNotKept, st)
	}
	if !strings.Contains(d.logs.String(), "msg=ha2fabric.passcode ") {
		t.Fatalf("no log line\n%s", d.logs.String())
	}
	d.stop()
}
