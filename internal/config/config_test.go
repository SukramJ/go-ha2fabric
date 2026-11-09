// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package config

import (
	"errors"
	"io"
	"log/slog"
	"path/filepath"
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
	env := MapEnv(map[string]string{
		SupervisorTokenEnv: "sup", "INCLUDE": "light.*, switch.a", "EXPOSE_ASSIST": "false", "WEB_ALLOW": "172.30.32.2",
		"INCLUDE_LABELS": "Matter\nLiving room", "EXCLUDE_LABELS": "no_bridge, ",
	})
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
	if r := c.Exposure(); r.Assist || len(r.Include) != 2 || !slices.Equal(r.IncludeLabels, []string{"Matter", "Living room"}) || !slices.Equal(r.ExcludeLabels, []string{"no_bridge"}) {
		t.Fatalf("rule %+v", r)
	}
	if want := filepath.Join(filepath.Dir(c.StorePath), SelectionFile); c.SelectionPath != want || c.WebAdmin {
		t.Fatalf("selection %q (want %q), web admin %v", c.SelectionPath, want, c.WebAdmin)
	}
}

func TestWebAdmin(t *testing.T) {
	env := MapEnv(map[string]string{"HA_URL": "ws://a/api/websocket", "HA_TOKEN": "t", "WEB_ADMIN": "true"})
	c, err := Load([]string{"--store-path", t.TempDir() + "/s.db"}, env, io.Discard)
	if err != nil || !c.WebAdmin {
		t.Fatalf("%+v %v", c, err)
	}
}

func TestCamera(t *testing.T) {
	base := map[string]string{"HA_URL": "ws://a/api/websocket", "HA_TOKEN": "t"}
	c, err := Load([]string{"--store-path", t.TempDir() + "/s.db"}, MapEnv(base), io.Discard)
	if err != nil || c.Camera {
		t.Fatalf("default: %+v %v", c, err)
	}
	base["CAMERA"] = "true"
	c, err = Load([]string{"--store-path", t.TempDir() + "/s.db"}, MapEnv(base), io.Discard)
	if err != nil || !c.Camera {
		t.Fatalf("CAMERA=true: %+v %v", c, err)
	}
	c, err = Load([]string{"--store-path", t.TempDir() + "/s.db", "--camera=false"}, MapEnv(base), io.Discard)
	if err != nil || c.Camera {
		t.Fatalf("--camera=false: %+v %v", c, err)
	}
}

func TestNodeLimit(t *testing.T) {
	base := map[string]string{"HA_URL": "ws://a/api/websocket", "HA_TOKEN": "t"}
	c, err := Load([]string{"--store-path", t.TempDir() + "/s.db"}, MapEnv(base), io.Discard)
	if err != nil || c.NodeLimit != DefaultNodeLimit {
		t.Fatalf("default: %+v %v", c, err)
	}
	base["NODE_LIMIT"] = "0"
	c, err = Load([]string{"--store-path", t.TempDir() + "/s.db"}, MapEnv(base), io.Discard)
	if err != nil || c.NodeLimit != 0 {
		t.Fatalf("NODE_LIMIT=0: %+v %v", c, err)
	}
	c, err = Load([]string{"--store-path", t.TempDir() + "/s.db", "--node-limit", "32"}, MapEnv(base), io.Discard)
	if err != nil || c.NodeLimit != MaxNodeLimit {
		t.Fatalf("--node-limit 32: %+v %v", c, err)
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
		"WEB_ADMIN": "maybe", "CAMERA": "sometimes", "NODE_LIMIT": "33",
	})
	_, err := Load([]string{"extra"}, env, io.Discard)
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("%v", err)
	}
	for _, want := range []string{"unexpected arguments", "HA_URL must be", "HA_TOKEN is required", "LISTEN", "PASSCODE 11111111", "DISCRIMINATOR", "VENDOR_ID", "PRODUCT_ID", "LABEL is 33", "STORE_PATH", "INCLUDE/EXCLUDE", "EXPOSE_ASSIST", "LOG_LEVEL", "WEB_LISTEN", "WEB_ALLOW", "WEB_ADMIN", "CAMERA", "NODE_LIMIT"} {
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

// TestNodeConfig: the node configuration the daemon hands node.New
// validates, carries the identity, and accepts TCP with the camera only.
func TestNodeConfig(t *testing.T) {
	c, err := Load([]string{"--store-path", t.TempDir() + "/n.db"}, MapEnv(map[string]string{"HA_URL": "ws://a/api/websocket", "HA_TOKEN": "t"}), nil)
	if err != nil {
		t.Fatal(err)
	}
	nc := c.Node(nil)
	if err := nc.Validate(); err != nil {
		t.Fatalf("node.Config: %v", err)
	}
	if nc.BasicInformation.NodeLabel != DefaultLabel || nc.Network.Listen != DefaultListen || nc.Store.Path != c.StorePath || nc.BasicInformation.SoftwareVersion == 0 {
		t.Fatalf("node config %+v", nc)
	}
	if nc.Network.TCP.Incoming || nc.Network.TCP.Outgoing {
		t.Fatalf("TCP without the camera: %+v", nc.Network.TCP)
	}
	c.Camera = true
	nc = c.Node(nil)
	if err := nc.Validate(); err != nil || !nc.Network.TCP.Incoming || nc.Network.TCP.Outgoing {
		t.Fatalf("TCP with the camera: %+v %v", nc.Network.TCP, err)
	}
}

// TestMDNSInterfaces: MDNS_INTERFACES and the flag split on commas and
// newlines (the add-on's list), refuse an empty or doubled name, and reach
// the node's configuration (issue #41).
func TestMDNSInterfaces(t *testing.T) {
	base := map[string]string{"HA_URL": "ws://a/api/websocket", "HA_TOKEN": "t"}
	store := func() string { return t.TempDir() + "/s.db" }
	c, err := Load([]string{"--store-path", store()}, MapEnv(base), io.Discard)
	if err != nil || c.MDNSInterfaces != nil || c.Node(nil).Network.MDNS.Interfaces != nil {
		t.Fatalf("default: %+v %v", c, err)
	}
	base["MDNS_INTERFACES"] = "eth0\nbr30 "
	c, err = Load([]string{"--store-path", store()}, MapEnv(base), io.Discard)
	if err != nil || !slices.Equal(c.MDNSInterfaces, []string{"eth0", "br30"}) {
		t.Fatalf("env: %+v %v", c, err)
	}
	nc := c.Node(nil)
	if err := nc.Validate(); err != nil || !slices.Equal(nc.Network.MDNS.Interfaces, []string{"eth0", "br30"}) {
		t.Fatalf("node config: %+v %v", nc.Network.MDNS, err)
	}
	c, err = Load([]string{"--store-path", store(), "--mdns-interfaces", "enp1s0"}, MapEnv(base), io.Discard)
	if err != nil || !slices.Equal(c.MDNSInterfaces, []string{"enp1s0"}) {
		t.Fatalf("flag: %+v %v", c, err)
	}
	for _, bad := range []string{"eth0,,br30", "eth0, ", "eth0,eth0", "eth0\n\nbr30"} {
		base["MDNS_INTERFACES"] = bad
		var v *ValidationError
		if _, err := Load([]string{"--store-path", store()}, MapEnv(base), io.Discard); !errors.As(err, &v) || !strings.Contains(err.Error(), "MDNS_INTERFACES") {
			t.Errorf("%q: %v", bad, err)
		}
	}
}
