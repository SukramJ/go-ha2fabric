// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package version

import (
	"strings"
	"testing"
)

func TestSoftware(t *testing.T) {
	saved := Version
	defer func() { Version = saved }()
	for v, want := range map[string]uint32{"0.1.0": 1000, "v1.2.3": 1002003, "dev": 0, "1.2": 0, "1.1000.0": 0, "1.x.0": 0} {
		Version = v
		if got := Software(); got != want {
			t.Errorf("Software(%q) = %d, want %d", v, got, want)
		}
	}
}

func TestString(t *testing.T) {
	if s := String(); !strings.HasPrefix(s, "go-ha2fabric "+Version) || !strings.Contains(s, "go-fabric") {
		t.Fatalf("String() = %q", s)
	}
	if GoFabric() == "" || GoHamapping() == "" {
		t.Fatal("empty dependency version")
	}
}
