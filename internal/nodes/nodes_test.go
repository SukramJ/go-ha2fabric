// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package nodes_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/SukramJ/go-fabric/node"

	"github.com/SukramJ/go-ha2fabric/internal/nodes"
)

var quiet = slog.New(slog.DiscardHandler)

func open(t *testing.T, dir string) *nodes.Registry {
	t.Helper()
	r, err := nodes.Open(dir, quiet)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestSlug(t *testing.T) {
	for label, want := range map[string]string{
		"Alexa":                 "alexa",
		"  Apple Home  ":        "apple-home",
		"Küche/Wohnzimmer":      "kueche-wohnzimmer",
		"Küche":                 "kueche",
		"ÄÖÜ äöü ß":             "aeoeue-aeoeue-ss",
		"Straße":                "strasse",
		"Café Crème":            "cafe-creme",
		"Ålesund":               "alesund",
		"Façade":                "facade",
		"Señor Niño":            "senor-nino",
		"Łódź Øresund":          "lodz-oresund",
		"日本":                    "node",
		"---":                   "node",
		"":                      "node",
		"Google_Home 2":         "google-home-2",
		strings.Repeat("a", 90): strings.Repeat("a", 56),
	} {
		if got := nodes.Slug(label); got != want {
			t.Errorf("Slug(%q) = %q, want %q", label, got, want)
		}
		if !nodes.ValidKey(nodes.Slug(label)) {
			t.Errorf("Slug(%q) is no valid key", label)
		}
	}
	for _, k := range []string{"default", "status", "pair", "-a", "", strings.Repeat("a", 65), "a b"} {
		if nodes.ValidKey(k) {
			t.Errorf("ValidKey(%q)", k)
		}
	}
	for _, k := range []string{"a", "A.b_c-1", strings.Repeat("a", 64)} {
		if !nodes.ValidKey(k) {
			t.Errorf("!ValidKey(%q)", k)
		}
	}
}

// TestAllocate: keys are unique slugs clear of reserved names and files on
// disk, ports count from the configured one + 1 past taken and busy
// ones, discriminators are unique 12-bit values and passcodes valid.
func TestAllocate(t *testing.T) {
	dir := t.TempDir()
	r := open(t, dir)
	busy := map[string]bool{"udp [::1]:5542": true, "tcp [::1]:5543": true}
	var probes []string
	r.SetSources(nil, func(network, addr string) bool {
		probes = append(probes, network+" "+addr)
		return !busy[network+" "+addr]
	}, func() time.Time { return time.Date(2026, 11, 5, 10, 0, 0, 5, time.UTC) })
	c := nodes.Configured{Listen: "[::1]:5540", Discriminator: 3840, TCP: true}

	a, err := r.Allocate("Alexa", c)
	if err != nil {
		t.Fatal(err)
	}
	if a.Key != "alexa" || a.Listen != "[::1]:5541" || a.Label != "Alexa" || !a.CreatedAt.Equal(time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("first node %+v", a)
	}
	if err := r.Add(a); err != nil {
		t.Fatal(err)
	}
	// 5541 is the registry's, 5542 is busy over UDP, 5543 over TCP.
	b, err := r.Allocate("Alexa", c)
	if err != nil {
		t.Fatal(err)
	}
	if b.Key != "alexa-2" || b.Listen != "[::1]:5544" {
		t.Fatalf("second node %+v (probes %q)", b, probes)
	}
	if b.Discriminator == a.Discriminator || b.Discriminator == 3840 || b.Discriminator > node.MaxDiscriminator {
		t.Fatalf("discriminators %d, %d", a.Discriminator, b.Discriminator)
	}
	for _, n := range []nodes.Node{a, b} {
		if err := node.ValidateSetupPIN(n.Passcode); err != nil {
			t.Fatalf("passcode %d: %v", n.Passcode, err)
		}
	}
	if err := r.Add(b); err != nil {
		t.Fatal(err)
	}
	if err := r.Add(b); err == nil {
		t.Fatal("a key added twice")
	}

	// A reserved name and a key whose files are still on disk are skipped.
	if n, _ := r.Allocate("Default", c); n.Key != "default-2" {
		t.Fatalf("reserved: %q", n.Key)
	}
	if err := os.WriteFile(filepath.Join(dir, "selection-hue.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if n, _ := r.Allocate("Hue", c); n.Key != "hue-2" {
		t.Fatalf("on disk: %q", n.Key)
	}

	// Without a port in the configured address, or with port 0, the count
	// starts at 5540.
	if n, _ := r.Allocate("x", nodes.Configured{Listen: "127.0.0.1:0"}); n.Listen != "127.0.0.1:5542" {
		t.Fatalf("ephemeral configured port: %q", n.Listen)
	}
}

// TestAllocateDiscriminatorCollision: a drawn discriminator another node
// has is drawn again.
func TestAllocateDiscriminatorCollision(t *testing.T) {
	r := open(t, t.TempDir())
	// big-endian 2-byte draws: 3840 (the configured one), then 7.
	r.SetSources(bytes.NewReader([]byte{0x0f, 0x00, 0x00, 0x07, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}), func(string, string) bool { return true }, nil)
	n, err := r.Allocate("a", nodes.Configured{Listen: ":5540", Discriminator: 3840})
	if err != nil {
		t.Fatal(err)
	}
	if n.Discriminator != 7 {
		t.Fatalf("discriminator %d", n.Discriminator)
	}
	// An exhausted source is an error, not a zero value.
	r.SetSources(bytes.NewReader(nil), nil, nil)
	if _, err := r.Allocate("b", nodes.Configured{Listen: ":5540"}); err == nil {
		t.Fatal("no error from an empty source")
	}
}

func TestAllocateRealProbe(t *testing.T) {
	// The real probe: a port bound here is skipped.
	c, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	_, p, _ := net.SplitHostPort(c.LocalAddr().String())
	r := open(t, t.TempDir())
	base := "127.0.0.1:" + p
	n, err := r.Allocate("a", nodes.Configured{Listen: base, TCP: true})
	if err != nil {
		t.Fatal(err)
	}
	if n.Listen == base {
		t.Fatalf("allocated the configured port %s", n.Listen)
	}
}

// TestPersistence: Add, Rename and Remove write the file, Open reads it
// back, and the file holds the documented format.
func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	r := open(t, dir)
	r.SetSources(nil, func(string, string) bool { return true }, nil)
	a, _ := r.Allocate("Alexa", nodes.Configured{Listen: ":5540"})
	b, _ := r.Allocate("Google", nodes.Configured{Listen: ":5540"})
	b.Listen = ":5542"
	for _, n := range []nodes.Node{a, b} {
		if err := r.Add(n); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.Rename("alexa", "Echo"); err != nil {
		t.Fatal(err)
	}
	if err := r.Rename("nope", "x"); !errors.Is(err, nodes.ErrNotFound) {
		t.Fatalf("rename unknown: %v", err)
	}
	if err := r.Remove("nope"); !errors.Is(err, nodes.ErrNotFound) {
		t.Fatalf("remove unknown: %v", err)
	}
	st, err := os.Stat(filepath.Join(dir, nodes.File))
	if err != nil {
		t.Fatal(err)
	}
	// Windows has no POSIX modes (the write asks 0600 everywhere; there
	// the data directory's ACL, the service's concern, protects the file).
	if runtime.GOOS != "windows" && st.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v: the file holds passcodes", st.Mode())
	}
	raw, _ := os.ReadFile(filepath.Join(dir, nodes.File))
	var doc struct {
		Nodes []map[string]any `json:"nodes"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil || len(doc.Nodes) != 2 {
		t.Fatalf("file %s: %v", raw, err)
	}
	for _, k := range []string{"key", "label", "listen", "discriminator", "passcode", "created_at"} {
		if _, ok := doc.Nodes[0][k]; !ok {
			t.Errorf("no %q in %s", k, raw)
		}
	}

	r2 := open(t, dir)
	got := r2.Nodes()
	if len(got) != 2 || got[0].Label != "Echo" || got[1].Key != "google" || got[1].Listen != ":5542" || got[0].Passcode != a.Passcode {
		t.Fatalf("read back %+v", got)
	}
	if err := r2.Remove("alexa"); err != nil {
		t.Fatal(err)
	}
	if r3 := open(t, dir); r3.Len() != 1 || r3.Nodes()[0].Key != "google" {
		t.Fatalf("after remove %+v", r3.Nodes())
	}
	if err := r2.Remove("google"); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(filepath.Join(dir, nodes.File))
	if !strings.Contains(string(raw), `"nodes": []`) {
		t.Fatalf("empty registry %s", raw)
	}
}

// TestCorruptFile: a file that does not parse, or holds a node that is not
// usable, is moved aside and the registry starts empty.
func TestCorruptFile(t *testing.T) {
	for name, body := range map[string]string{
		"syntax":     "{",
		"key":        `{"nodes":[{"key":"status","listen":":5541","passcode":34567890}]}`,
		"repeated":   `{"nodes":[{"key":"a","listen":":5541","passcode":34567890},{"key":"a","listen":":5542","passcode":34567890}]}`,
		"listen":     `{"nodes":[{"key":"a","passcode":34567890}]}`,
		"passcode":   `{"nodes":[{"key":"a","listen":":5541","passcode":12345678}]}`,
		"discrimin.": `{"nodes":[{"key":"a","listen":":5541","passcode":34567890,"discriminator":4096}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, nodes.File), []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			r := open(t, dir)
			if r.Len() != 0 {
				t.Fatalf("nodes %+v", r.Nodes())
			}
			if m, _ := filepath.Glob(filepath.Join(dir, nodes.File+".corrupt-*")); len(m) != 1 {
				t.Fatalf("moved aside: %q", m)
			}
		})
	}
	// An unreadable path is an error.
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, nodes.File), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := nodes.Open(dir, nil); err == nil {
		t.Fatal("a directory read as the file")
	}
}

func TestRemoveFiles(t *testing.T) {
	dir := t.TempDir()
	r := open(t, dir)
	n := nodes.Node{Key: "alexa"}
	names := []string{"ha2fabric-alexa.db", "ha2fabric-alexa.db-journal", "ha2fabric-alexa.db-wal", "selection-alexa.json", "selection-alexa.json.corrupt-1", "ha2fabric.db", "selection.json"}
	for _, f := range names {
		if err := os.WriteFile(filepath.Join(dir, f), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.RemoveFiles(&n); err != nil {
		t.Fatal(err)
	}
	left, _ := filepath.Glob(filepath.Join(dir, "*"))
	if len(left) != 2 {
		t.Fatalf("left %q: the configured node's files stay, the node's go", left)
	}
}

func TestWriteFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no read-only directory by mode on Windows: the data directory's ACL is the service's concern")
	}
	dir := t.TempDir()
	r := open(t, dir)
	r.SetSources(nil, func(string, string) bool { return true }, nil)
	n, _ := r.Allocate("a", nodes.Configured{})
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	if os.Geteuid() == 0 {
		t.Skip("root writes a read-only directory")
	}
	if err := r.Add(n); err == nil {
		t.Fatal("Add wrote into a read-only directory")
	}
	if r.Len() != 0 {
		t.Fatal("a node added although the file was not written")
	}
}
