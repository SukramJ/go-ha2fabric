// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package main

import (
	"os"
	"strings"
	"testing"

	"thingfabric.dev/go-ha2fabric/internal/version"
)

// TestVersionBannerFindsItsModules (HR-P4-test-67): the version banner
// names the go-fabric and go-hamapping versions go.mod requires. This
// test binary links both, as the daemon does, so internal/version's
// debug.ReadBuildInfo lookup finds them by their module paths; a path
// that misses (a module moved) answers "unknown" and fails here.
func TestVersionBannerFindsItsModules(t *testing.T) {
	b, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	required := map[string]string{}
	for l := range strings.Lines(string(b)) {
		if f := strings.Fields(l); len(f) >= 2 && strings.HasPrefix(f[1], "v") {
			required[f[0]] = f[1]
		}
	}
	for path, got := range map[string]string{
		"thingfabric.dev/go-fabric": version.GoFabric(),
		"sukramj.dev/go-hamapping":  version.GoHamapping(),
	} {
		if want := required[path]; want == "" || got != want {
			t.Errorf("the banner names %s %q, go.mod requires %q", path, got, want)
		}
	}
	if s := version.String(); !strings.Contains(s, "go-fabric "+version.GoFabric()) || !strings.Contains(s, "go-hamapping "+version.GoHamapping()) {
		t.Errorf("String() = %q", s)
	}
}
