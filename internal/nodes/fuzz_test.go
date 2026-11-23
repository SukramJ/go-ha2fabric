// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package nodes_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"thingfabric.dev/go-fabric/node"

	"github.com/SukramJ/go-ha2fabric/internal/nodes"
)

// keyPattern is the contract's node key pattern (see nodes.go maxKey).
var keyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// FuzzOpen holds Open against any content of nodes.json: it never panics
// and never fails on content; a file that does not parse or check is moved
// aside and the registry starts empty, and every node a file that does
// keeps has a key of the contract's pattern, given once, whose store and
// selection files stay in the registry's directory, a listen address, a
// 12-bit discriminator and a valid passcode.
func FuzzOpen(f *testing.F) {
	for _, s := range []string{
		`{"nodes":[{"key":"kitchen","label":"Kitchen","listen":"[::]:5541","discriminator":3841,"passcode":20202022,"created_at":"2026-11-23T10:00:00Z"}]}`,
		`{"nodes":[{"key":"a","listen":"x","discriminator":1,"passcode":20202022},{"key":"a","listen":"x","discriminator":2,"passcode":20202023}]}`,
		`{"nodes":[{"key":"../x","listen":"x","discriminator":1,"passcode":20202022}]}`,
		`{"nodes":[{"key":"status","listen":"x","discriminator":1,"passcode":20202022}]}`,
		`{"nodes":[{"key":"a","listen":"x","discriminator":4096,"passcode":11111111}]}`,
		`{"nodes":null}`, `{}`, `{`, ``, `[]`,
	} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, nodes.File), b, 0o600); err != nil {
			t.Fatal(err)
		}
		r, err := nodes.Open(dir, quiet)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		seen := map[string]bool{}
		for _, n := range r.Nodes() {
			if !keyPattern.MatchString(n.Key) || seen[n.Key] {
				t.Fatalf("kept key %q", n.Key)
			}
			seen[n.Key] = true
			for _, name := range []string{n.StoreFile(), n.SelectionFile()} {
				if filepath.Dir(r.Path(name)) != filepath.Clean(dir) {
					t.Fatalf("key %q: %s leaves the directory", n.Key, name)
				}
			}
			if n.Listen == "" || n.Discriminator > node.MaxDiscriminator || node.ValidateSetupPIN(n.Passcode) != nil {
				t.Fatalf("kept node %+v", n)
			}
		}
	})
}

// FuzzSlug holds the key a web UI label becomes against any label: it
// never panics, and the slug matches the contract's key pattern with room
// for a "-<n>" suffix.
func FuzzSlug(f *testing.F) {
	for _, s := range []string{"Kitchen", "Küche & Bad", "", "---", "Status", "ﬁ", strings.Repeat("a", 100), "\xff\xfe"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, label string) {
		s := nodes.Slug(label)
		if !keyPattern.MatchString(s) || len(s) > 56 {
			t.Fatalf("Slug(%q) = %q", label, s)
		}
	})
}
