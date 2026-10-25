// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package config

import (
	"errors"
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"
)

func TestDefaultsStandAlone(t *testing.T) {
	c, err := Load(nil, MapEnv(map[string]string{"HA_URL": "ws://ha.local:8123/api/websocket", "HA_TOKEN": "secret"}), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if c.Addon || c.Listen != DefaultListen || c.Passcode != DefaultPasscode || c.Discriminator != DefaultDiscriminator ||
		c.VendorID != DefaultVendorID || c.ProductID != DefaultProductID || c.Label != DefaultLabel || c.StorePath != DefaultStoreFile ||
		!c.ExposeAssist || c.LogLevel != slog.LevelInfo || c.WebListen != DefaultWebListen || c.HAToken != "secret" {
		t.Fatalf("defaults %+v", c)
	}
}

func TestAddon(t *testing.T) {
	env := MapEnv(map[string]string{SupervisorTokenEnv: "sup", "INCLUDE": "light.*, switch.a", "EXPOSE_ASSIST": "false", "WEB_ALLOW": "172.30.32.2"})
	c, err := Load(nil, env, io.Discard)
	if err == nil {
		// /data does not exist on the test host.
		t.Fatalf("an add-on store without /data passed: %+v", c)
	}
	c, err = Load([]string{"--store-path", t.TempDir() + "/s.db"}, env, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Addon || c.HAURL != SupervisorURL || c.HAToken != "sup" || c.ExposeAssist || !slices.Equal(c.Include, []string{"light.*", "switch.a"}) || !slices.Equal(c.WebAllow, []string{"172.30.32.2"}) {
		t.Fatalf("add-on %+v", c)
	}
	if r := c.Exposure(); r.Assist || len(r.Include) != 2 {
		t.Fatalf("rule %+v", r)
	}
}

func TestFlagsWin(t *testing.T) {
	env := MapEnv(map[string]string{"HA_URL": "ws://a/api/websocket", "HA_TOKEN": "t", "LISTEN": ":6000", "LOG_LEVEL": "debug"})
	c, err := Load([]string{"--listen", "[::]:5541", "--passcode", "34567890", "--discriminator", "0xABC", "--vendor-id", "65521", "--label", "Home", "--web-listen", "", "--dump-plan"}, env, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if c.Listen != "[::]:5541" || c.Passcode != 34567890 || c.Discriminator != 0xABC || c.VendorID != 65521 || c.Label != "Home" || c.WebListen != "" || !c.DumpPlan || c.LogLevel != slog.LevelDebug {
		t.Fatalf("flags %+v", c)
	}
}

func TestValidation(t *testing.T) {
	env := MapEnv(map[string]string{
		"HA_URL": "http://x", "LISTEN": "5540", "PASSCODE": "11111111", "DISCRIMINATOR": "5000", "VENDOR_ID": "0",
		"PRODUCT_ID": "zz", "LABEL": strings.Repeat("x", 33), "STORE_PATH": "/no/such/dir/x.db", "INCLUDE": "light.[",
		"EXPOSE_ASSIST": "perhaps", "LOG_LEVEL": "loud", "WEB_LISTEN": "localhost", "WEB_ALLOW": "nobody",
	})
	_, err := Load([]string{"extra"}, env, io.Discard)
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("%v", err)
	}
	for _, want := range []string{"unexpected arguments", "HA_URL must be", "HA_TOKEN is required", "LISTEN", "PASSCODE 11111111", "DISCRIMINATOR", "VENDOR_ID", "PRODUCT_ID", "LABEL is 33", "STORE_PATH", "INCLUDE/EXCLUDE", "EXPOSE_ASSIST", "LOG_LEVEL", "WEB_LISTEN", "WEB_ALLOW"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("no %q in %v", want, err)
		}
	}
	if _, err := Load(nil, MapEnv(map[string]string{"HA_TOKEN": "t"}), io.Discard); err == nil || !strings.Contains(err.Error(), "HA_URL is required") || strings.Contains(err.Error(), "\n") {
		t.Fatalf("one issue: %v", err)
	}
	if _, err := Load([]string{"-h"}, MapEnv(nil), io.Discard); !IsHelp(err) {
		t.Fatalf("-h: %v", err)
	}
	if c, err := Load([]string{"--version"}, MapEnv(nil), io.Discard); err != nil || !c.Version {
		t.Fatalf("--version: %v", err)
	}
	if _, err := Load([]string{"--label", " "}, MapEnv(map[string]string{"HA_URL": "ws://a", "HA_TOKEN": "t"}), io.Discard); err == nil || !strings.Contains(err.Error(), "LABEL must not be empty") {
		t.Fatalf("empty label: %v", err)
	}
	if _, err := Load(nil, MapEnv(map[string]string{"HA_URL": "ws://a", "HA_TOKEN": "t", "PASSCODE": "x"}), io.Discard); err == nil || !strings.Contains(err.Error(), "PASSCODE must be a number") {
		t.Fatalf("passcode: %v", err)
	}
}

func TestTrivialPasscodes(t *testing.T) {
	for _, p := range []string{"12345678", "87654321", "00000000", "99999999", "100000000"} {
		if _, err := Load([]string{"--passcode", p}, MapEnv(map[string]string{"HA_URL": "ws://a", "HA_TOKEN": "t"}), io.Discard); err == nil {
			t.Errorf("passcode %s passed", p)
		}
	}
}

func TestTokenNotInErrors(t *testing.T) {
	_, err := Load(nil, MapEnv(map[string]string{"HA_TOKEN": "very-secret", "LISTEN": "bad"}), io.Discard)
	if err == nil || strings.Contains(err.Error(), "very-secret") {
		t.Fatalf("%v", err)
	}
}
