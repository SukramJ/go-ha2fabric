// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

// Package config is the daemon's configuration: environment variables,
// each overridden by its command-line flag, defaulted and validated as a
// whole (the form of go-zendure2mqtt's internal/config: every problem in
// one error). The access token is read from the environment only — a flag
// would show it in the process list — and never printed.
package config

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/SukramJ/go-fabric/node"

	"github.com/SukramJ/go-ha2fabric/internal/expose"
)

// The Supervisor's WebSocket proxy to Core and the token it puts in an
// add-on's environment (https://developers.home-assistant.io/docs/apps/
// communication, "WebSocket API" and "Your token").
const (
	SupervisorURL      = "ws://supervisor/core/websocket"
	SupervisorTokenEnv = "SUPERVISOR_TOKEN"
)

// Defaults. The commissioning values are the public test values of
// go-fabric's cookbook (docs/guides/embed-in-a-daemon.md §2, CSA test
// vendor 0xFFF1, product 0x8001); change the passcode before a real
// pairing.
const (
	DefaultListen        = ":5540"
	DefaultPasscode      = 20202021
	DefaultDiscriminator = 3840
	DefaultVendorID      = 0xFFF1
	DefaultProductID     = 0x8001
	DefaultLabel         = "go-ha2fabric"
	DefaultWebListen     = "127.0.0.1:8099"
	DefaultStoreFile     = "ha2fabric.db"
	// AddonStorePath is the store in the add-on's persistent /data.
	AddonStorePath = "/data/" + DefaultStoreFile
)

// Config is the daemon's configuration.
type Config struct {
	// HAURL is the WebSocket API's URL; in an add-on (SUPERVISOR_TOKEN
	// set, HA_URL unset) the Supervisor's proxy.
	HAURL string
	// HAToken is the access token (HA_TOKEN, else SUPERVISOR_TOKEN). It
	// is never logged or persisted.
	HAToken string
	// Addon is whether the daemon runs as an add-on (SUPERVISOR_TOKEN
	// set).
	Addon bool

	Listen        string
	Passcode      uint32
	Discriminator uint16
	VendorID      uint16
	ProductID     uint16
	Label         string
	StorePath     string

	Include, Exclude []string
	ExposeAssist     bool

	LogLevel slog.Level
	// WebListen is the web UI's address; empty disables it.
	WebListen string
	// WebAllow restricts the web UI to these client addresses (the
	// Ingress proxy's 172.30.32.2 in the add-on); empty allows every one.
	WebAllow []string

	// DumpPlan prints every candidate entity's verdict and exits.
	DumpPlan bool
	// Version prints the version and exits.
	Version bool
}

// Exposure is the configured exposure rule.
func (c *Config) Exposure() expose.Rule {
	return expose.Rule{Assist: c.ExposeAssist, Include: c.Include, Exclude: c.Exclude}
}

// ValidationError aggregates every problem [Load] found.
type ValidationError struct{ Issues []string }

func (e *ValidationError) Error() string {
	if len(e.Issues) == 1 {
		return "config: " + e.Issues[0]
	}
	return fmt.Sprintf("config: %d issues:\n  - %s", len(e.Issues), strings.Join(e.Issues, "\n  - "))
}

// Env reads the environment; tests pass a map.
type Env func(key string) (string, bool)

// OSEnv is the process environment.
func OSEnv(key string) (string, bool) { return os.LookupEnv(key) }

// MapEnv is an environment of a map.
func MapEnv(m map[string]string) Env {
	return func(k string) (string, bool) {
		v, ok := m[k]
		return v, ok
	}
}

// Load reads the environment, then the flags of args (which win),
// defaults what is unset and validates the whole. The usage text goes to
// out on -h.
func Load(args []string, env Env, out io.Writer) (*Config, error) {
	get := func(k string) string {
		v, _ := env(k)
		return strings.TrimSpace(v)
	}
	var issues []string
	add := func(format string, a ...any) { issues = append(issues, fmt.Sprintf(format, a...)) }

	fs := flag.NewFlagSet("ha2fabric", flag.ContinueOnError)
	fs.SetOutput(out)
	str := func(name, envKey, def, usage string) *string {
		v := def
		if e := get(envKey); e != "" {
			v = e
		}
		return fs.String(name, v, usage+" ("+envKey+")")
	}
	haURL := str("ha-url", "HA_URL", "", "Home Assistant WebSocket URL, e.g. ws://homeassistant.local:8123/api/websocket")
	listen := str("listen", "LISTEN", DefaultListen, "UDP listen address of the Matter node")
	passcode := str("passcode", "PASSCODE", strconv.Itoa(DefaultPasscode), "setup passcode")
	discr := str("discriminator", "DISCRIMINATOR", strconv.Itoa(DefaultDiscriminator), "12-bit discriminator")
	vendor := str("vendor-id", "VENDOR_ID", "0xFFF1", "vendor id")
	product := str("product-id", "PRODUCT_ID", "0x8001", "product id")
	label := str("label", "LABEL", DefaultLabel, "the bridge's node label")
	store := str("store-path", "STORE_PATH", "", "the node's SQLite store file")
	include := str("include", "INCLUDE", "", "entity ids or globs always exposed (comma-separated)")
	exclude := str("exclude", "EXCLUDE", "", "entity ids or globs never exposed (comma-separated)")
	assist := str("expose-assist", "EXPOSE_ASSIST", "true", "follow Home Assistant's expose-to-Assist setting")
	level := str("log-level", "LOG_LEVEL", "info", "debug, info, warn or error")
	web := str("web-listen", "WEB_LISTEN", DefaultWebListen, "web UI address; empty disables it")
	allow := str("web-allow", "WEB_ALLOW", "", "client addresses allowed on the web UI (comma-separated); empty allows all")
	dump := fs.Bool("dump-plan", false, "print every candidate entity's verdict and exit")
	ver := fs.Bool("version", false, "print the version and exit")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if fs.NArg() > 0 {
		add("unexpected arguments %q", fs.Args())
	}

	c := &Config{DumpPlan: *dump, Version: *ver}
	supervisor := get(SupervisorTokenEnv)
	c.Addon = supervisor != ""
	c.HAToken = get("HA_TOKEN")
	if c.HAToken == "" {
		c.HAToken = supervisor
	}
	c.HAURL = *haURL
	if c.HAURL == "" && c.Addon {
		c.HAURL = SupervisorURL
	}
	c.Listen = *listen
	c.Label = strings.TrimSpace(*label)
	c.StorePath = *store
	if c.StorePath == "" {
		c.StorePath = DefaultStoreFile
		if c.Addon {
			c.StorePath = AddonStorePath
		}
	}
	c.Include, c.Exclude = expose.Split(*include), expose.Split(*exclude)
	c.WebListen = strings.TrimSpace(*web)
	c.WebAllow = expose.Split(*allow)

	if c.Version {
		return c, nil
	}
	if c.HAURL == "" {
		add("HA_URL is required outside the add-on")
	} else if !strings.HasPrefix(c.HAURL, "ws://") && !strings.HasPrefix(c.HAURL, "wss://") {
		add("HA_URL must be a ws:// or wss:// URL, got %q", c.HAURL)
	}
	if c.HAToken == "" {
		add("HA_TOKEN is required outside the add-on (a long-lived access token)")
	}
	if err := hostPort(c.Listen); err != nil {
		add("LISTEN %v", err)
	}
	if p, err := strconv.ParseUint(*passcode, 10, 32); err != nil {
		add("PASSCODE must be a number, got %q", *passcode)
	} else if err := node.ValidateSetupPIN(uint32(p)); err != nil {
		add("PASSCODE %d is invalid or trivially guessable (%v)", p, err)
	} else {
		c.Passcode = uint32(p)
	}
	if d, err := strconv.ParseUint(*discr, 0, 16); err != nil || d > node.MaxDiscriminator {
		add("DISCRIMINATOR must be 0..%d, got %q", node.MaxDiscriminator, *discr)
	} else {
		c.Discriminator = uint16(d)
	}
	c.VendorID = id16(*vendor, "VENDOR_ID", add)
	c.ProductID = id16(*product, "PRODUCT_ID", add)
	if c.Label == "" {
		add("LABEL must not be empty")
	} else if len(c.Label) > 32 {
		// NodeLabel is at most 32 bytes (go-fabric
		// cluster/core/basic_information.go:853 at v0.11.0).
		add("LABEL is %d bytes, at most 32", len(c.Label))
	}
	if dir := filepath.Dir(c.StorePath); dir != "." {
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			add("STORE_PATH directory %q does not exist", dir)
		}
	}
	if err := c.Exposure().Validate(); err != nil {
		add("INCLUDE/EXCLUDE %v", err)
	}
	if b, err := strconv.ParseBool(*assist); err != nil {
		add("EXPOSE_ASSIST must be true or false, got %q", *assist)
	} else {
		c.ExposeAssist = b
	}
	if err := c.LogLevel.UnmarshalText([]byte(*level)); err != nil {
		add("LOG_LEVEL must be debug, info, warn or error, got %q", *level)
	}
	if c.WebListen != "" {
		if err := hostPort(c.WebListen); err != nil {
			add("WEB_LISTEN %v", err)
		}
	}
	for _, a := range c.WebAllow {
		if net.ParseIP(a) == nil {
			add("WEB_ALLOW %q is not an IP address", a)
		}
	}
	if len(issues) > 0 {
		return nil, &ValidationError{Issues: issues}
	}
	return c, nil
}

func id16(s, key string, add func(string, ...any)) uint16 {
	v, err := strconv.ParseUint(s, 0, 16)
	if err != nil || v == 0 {
		add("%s must be 1..0xFFFF, got %q", key, s)
		return 0
	}
	return uint16(v)
}

func hostPort(s string) error {
	_, port, err := net.SplitHostPort(s)
	if err != nil {
		return fmt.Errorf("must be host:port, got %q", s)
	}
	p, err := strconv.Atoi(port)
	if err != nil || p < 1 || p > 65535 {
		return fmt.Errorf("port must be 1..65535, got %q", port)
	}
	return nil
}

// ErrHelp is flag.ErrHelp: -h was given.
var ErrHelp = flag.ErrHelp

// IsHelp reports whether err is the -h of [Load].
func IsHelp(err error) bool { return errors.Is(err, flag.ErrHelp) }
