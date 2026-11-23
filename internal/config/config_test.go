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
	if c.Addon || c.Listen != DefaultListen || c.Passcode != 0 || c.Discriminator != DefaultDiscriminator ||
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

// TestWebAllowedHosts: WEB_ALLOWED_HOSTS splits on commas and white space
// into the names go-fabric-ui's Host check admits; unset is none.
func TestWebAllowedHosts(t *testing.T) {
	base := map[string]string{"HA_URL": "ws://a/api/websocket", "HA_TOKEN": "t"}
	c, err := Load([]string{"--store-path", t.TempDir() + "/s.db"}, MapEnv(base), io.Discard)
	if err != nil || len(c.WebAllowedHosts) != 0 {
		t.Fatalf("default: %+v %v", c.WebAllowedHosts, err)
	}
	base["WEB_ALLOWED_HOSTS"] = "bridge.lan, ha2fabric.home.arpa"
	c, err = Load([]string{"--store-path", t.TempDir() + "/s.db"}, MapEnv(base), io.Discard)
	if err != nil || !slices.Equal(c.WebAllowedHosts, []string{"bridge.lan", "ha2fabric.home.arpa"}) {
		t.Fatalf("env: %+v %v", c.WebAllowedHosts, err)
	}
	for bad, want := range map[string]string{
		"bridge.lan:8099": "has a port", "http://bridge.lan": "has a scheme", "bridge.lan/ui": "has a path",
		"*.lan": "wildcard", "fe80::1": "IP address", "192.168.1.5": "IP address",
	} {
		base["WEB_ALLOWED_HOSTS"] = bad
		_, err := Load([]string{"--store-path", t.TempDir() + "/s.db"}, MapEnv(base), io.Discard)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: %v, want %q", bad, err, want)
		}
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
	c, err := Load([]string{"--store-path", t.TempDir() + "/n.db"}, MapEnv(map[string]string{"HA_URL": "ws://a/api/websocket", "HA_TOKEN": "t", "PASSCODE": "34567890"}), nil)
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
	if nc.ProductDescription.DeviceType != 0x000E {
		t.Fatalf("advertised device type %#x, want the Aggregator 0x000e", nc.ProductDescription.DeviceType)
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
	base := map[string]string{"HA_URL": "ws://a/api/websocket", "HA_TOKEN": "t", "PASSCODE": "34567890"}
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

// TestHelpHidesPasscode: the usage text does not show an environment
// PASSCODE as the flag's default (HR-P0-leak-29).
func TestHelpHidesPasscode(t *testing.T) {
	var out strings.Builder
	if _, err := Load([]string{"-h"}, MapEnv(map[string]string{"PASSCODE": "34567890"}), &out); !IsHelp(err) {
		t.Fatalf("-h: %v", err)
	}
	if strings.Contains(out.String(), "34567890") || !strings.Contains(out.String(), "-passcode") {
		t.Fatalf("usage text:\n%s", out.String())
	}
	c, err := Load(nil, MapEnv(map[string]string{"HA_URL": "ws://a", "HA_TOKEN": "t", "PASSCODE": "34567890"}), io.Discard)
	if err != nil || c.Passcode != 34567890 {
		t.Fatalf("environment passcode: %+v %v", c, err)
	}
}

// TestHAURL: HA_URL parses as a URL with a host (HR-P0-err-03's
// configuration half), and one with userinfo is refused without echoing
// it (HR-P0-leak-25).
func TestHAURL(t *testing.T) {
	for _, u := range []string{"ws://ha.local:8123/api/websocket", "wss://[::1]:8123/api/websocket", "ws://10.0.0.2/api/websocket?x=1"} {
		if _, err := Load(nil, MapEnv(map[string]string{"HA_URL": u, "HA_TOKEN": "t"}), io.Discard); err != nil {
			t.Errorf("%s: %v", u, err)
		}
	}
	for _, u := range []string{"ws://::/websocket", "ws:///api/websocket", "ws://", "ws://ha.local:http/x", "ws://ha.local:99999/x", "wss://%zz/x"} {
		if _, err := Load(nil, MapEnv(map[string]string{"HA_URL": u, "HA_TOKEN": "t"}), io.Discard); err == nil || !strings.Contains(err.Error(), "HA_URL") {
			t.Errorf("%s passed: %v", u, err)
		}
	}
	_, err := Load(nil, MapEnv(map[string]string{"HA_URL": "wss://me:hunter2@ha.local/api/websocket", "HA_TOKEN": "t"}), io.Discard)
	if err == nil || !strings.Contains(err.Error(), "user") || strings.Contains(err.Error(), "hunter2") {
		t.Fatalf("userinfo: %v", err)
	}
}

// TestTokenInClear: ws:// to a host that is neither loopback nor the
// Supervisor sends the token unencrypted (HR-P0-ha-23).
func TestTokenInClear(t *testing.T) {
	for u, want := range map[string]bool{
		"ws://homeassistant.local:8123/api/websocket": true,
		"ws://192.168.1.2:8123/api/websocket":         true,
		"ws://[fd00::2]:8123/api/websocket":           true,
		"wss://homeassistant.local/api/websocket":     false,
		"ws://127.0.0.1:8123/api/websocket":           false,
		"ws://[::1]:8123/api/websocket":               false,
		"ws://localhost:8123/api/websocket":           false,
		SupervisorURL:                                 false,
	} {
		c := &Config{HAURL: u}
		if got := c.TokenInClear(); got != want {
			t.Errorf("%s: %v, want %v", u, got, want)
		}
	}
}

// TestWebAdminOffLoopback: WEB_ADMIN on a listener that is not loopback
// needs WEB_ALLOW; the refusal names both remedies (HR-P0-cfg-22).
func TestWebAdminOffLoopback(t *testing.T) {
	base := map[string]string{"HA_URL": "ws://127.0.0.1/api/websocket", "HA_TOKEN": "t", "WEB_ADMIN": "true"}
	with := func(kv ...string) Env {
		m := map[string]string{}
		for k, v := range base {
			m[k] = v
		}
		for i := 0; i < len(kv); i += 2 {
			m[kv[i]] = kv[i+1]
		}
		return MapEnv(m)
	}
	for _, listen := range []string{"0.0.0.0:8099", ":8099", "192.168.1.2:8099", "[::]:8099", "bridge.lan:8099"} {
		_, err := Load(nil, with("WEB_LISTEN", listen), io.Discard)
		if err == nil || !strings.Contains(err.Error(), "WEB_ALLOW") || !strings.Contains(err.Error(), "loopback") {
			t.Errorf("WEB_ADMIN on %s without WEB_ALLOW: %v", listen, err)
		}
		if _, err := Load(nil, with("WEB_LISTEN", listen, "WEB_ALLOW", "192.168.1.10"), io.Discard); err != nil {
			t.Errorf("WEB_ADMIN on %s with WEB_ALLOW: %v", listen, err)
		}
		if _, err := Load(nil, with("WEB_LISTEN", listen, "WEB_ADMIN", "false"), io.Discard); err != nil {
			t.Errorf("status page only on %s: %v", listen, err)
		}
	}
	for _, listen := range []string{"127.0.0.1:8099", "[::1]:8099", "localhost:8099", ""} {
		if _, err := Load(nil, with("WEB_LISTEN", listen), io.Discard); err != nil {
			t.Errorf("WEB_ADMIN on %q: %v", listen, err)
		}
	}
}

// TestPasscodeUnset: without PASSCODE the passcode is 0, the daemon's
// random one; the public test value set explicitly is honoured and
// reported (HR-P0-cfg-21).
func TestPasscodeUnset(t *testing.T) {
	env := map[string]string{"HA_URL": "ws://a/api/websocket", "HA_TOKEN": "t"}
	c, err := Load(nil, MapEnv(env), io.Discard)
	if err != nil || c.Passcode != 0 || c.PasscodeIsTestValue() {
		t.Fatalf("unset: %+v %v", c, err)
	}
	env["PASSCODE"] = "20202021"
	c, err = Load(nil, MapEnv(env), io.Discard)
	if err != nil || c.Passcode != TestPasscode || !c.PasscodeIsTestValue() {
		t.Fatalf("the test value: %+v %v", c, err)
	}
}

// TestAddonWebAllow: in the add-on the client filter is the Ingress proxy:
// WEB_ALLOW defaults to it and another value is refused (HR-P0-cfg-30).
func TestAddonWebAllow(t *testing.T) {
	store := []string{"--store-path", t.TempDir() + "/s.db"}
	c, err := Load(store, MapEnv(map[string]string{SupervisorTokenEnv: "sup"}), io.Discard)
	if err != nil || !slices.Equal(c.WebAllow, []string{IngressProxy}) {
		t.Fatalf("default: %+v %v", c, err)
	}
	for _, allow := range []string{"192.168.1.2", "172.30.32.2,192.168.1.2"} {
		if _, err := Load(store, MapEnv(map[string]string{SupervisorTokenEnv: "sup", "WEB_ALLOW": allow}), io.Discard); err == nil || !strings.Contains(err.Error(), "WEB_ALLOW") {
			t.Errorf("%s: %v", allow, err)
		}
	}
}

// TestHelpHidesHAURL: the usage text does not show an environment HA_URL,
// which may carry a credential before Load refuses it (HR-P0-ha-56).
func TestHelpHidesHAURL(t *testing.T) {
	var out strings.Builder
	if _, err := Load([]string{"-h"}, MapEnv(map[string]string{"HA_URL": "wss://me:hunter2@ha.local/api/websocket?k=v"}), &out); !IsHelp(err) {
		t.Fatalf("-h: %v", err)
	}
	if strings.Contains(out.String(), "hunter2") || strings.Contains(out.String(), "ha.local") || !strings.Contains(out.String(), "-ha-url") {
		t.Fatalf("usage text:\n%s", out.String())
	}
	if c, err := Load(nil, MapEnv(map[string]string{"HA_URL": "ws://ha.local/api/websocket", "HA_TOKEN": "t"}), io.Discard); err != nil || c.HAURL != "ws://ha.local/api/websocket" {
		t.Fatalf("environment HA_URL: %+v %v", c, err)
	}
	if c, err := Load([]string{"--ha-url", "ws://flag.local/api/websocket"}, MapEnv(map[string]string{"HA_URL": "ws://ha.local/api/websocket", "HA_TOKEN": "t"}), io.Discard); err != nil || c.HAURL != "ws://flag.local/api/websocket" {
		t.Fatalf("the flag wins: %+v %v", c, err)
	}
}

// TestLabelText (HR-P1-data-20): LABEL is valid UTF-8 without control
// characters, as it becomes the node's NodeLabel.
func TestLabelText(t *testing.T) {
	for _, l := range []string{"\xff\xfe", "Bridge\x07", "a\nb"} {
		_, err := Load(nil, MapEnv(map[string]string{"HA_URL": "ws://a", "HA_TOKEN": "t", "LABEL": l}), io.Discard)
		if err == nil || !strings.Contains(err.Error(), "LABEL") {
			t.Errorf("LABEL %q: %v", l, err)
		}
	}
	if _, err := Load(nil, MapEnv(map[string]string{"HA_URL": "ws://a", "HA_TOKEN": "t", "LABEL": "Küche Brücke"}), io.Discard); err != nil {
		t.Fatal(err)
	}
}
