// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// profile is a package's blocks at pct percent of 100 statements.
func profile(pkg string, pct int) map[string]block {
	return map[string]block{
		module + pkg + "/a.go:1.1,2.2": {stmts: pct, covered: true},
		module + pkg + "/a.go:3.1,4.2": {stmts: 100 - pct, covered: false},
	}
}

func run(t *testing.T, blocks map[string]block, floors map[string]float64) (ok bool, out string) {
	t.Helper()
	var b strings.Builder
	ok = check(&b, blocks, floors)
	return ok, b.String()
}

// TestCheck holds the ratchet both ways (HR-P3-test-01): below the floor,
// a floor trailing the measurement by more than staleFloorSlack, and a
// package without a floor each fail.
func TestCheck(t *testing.T) {
	for _, c := range []struct {
		name   string
		pct    int
		floors map[string]float64
		ok     bool
		want   string
	}{
		{"at the floor", 94, map[string]float64{"internal/x": 93}, true, ""},
		{"within the slack", 94, map[string]float64{"internal/x": 91}, true, ""},
		{"below the floor", 90, map[string]float64{"internal/x": 93}, false, "internal/x: 90.0% < floor 93.0%"},
		{"stale floor", 94, map[string]float64{"internal/x": 82}, false, "raise it to 94"},
		{"no floor", 94, map[string]float64{}, false, `internal/x has 100 statements and no floor: add "internal/x 94"`},
		{"zero floor measures", 1, map[string]float64{"internal/x": 0}, false, "raise it to 1"},
		{"zero floor at zero", 0, map[string]float64{"internal/x": 0}, true, ""},
		{"floor without a package", 94, map[string]float64{"internal/x": 93, "internal/gone": 50}, false, "internal/gone has a floor but no statements"},
	} {
		ok, out := run(t, profile("internal/x", c.pct), c.floors)
		if ok != c.ok || !strings.Contains(out, c.want) {
			t.Errorf("%s: ok %v, want %v; output:\n%s", c.name, ok, c.ok, out)
		}
	}
}

// TestReadProfile merges a block's counts across test binaries.
func TestReadProfile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "c.out")
	data := "mode: set\n" +
		module + "internal/x/a.go:1.1,2.2 3 0\n" +
		module + "internal/x/a.go:1.1,2.2 3 1\n" +
		module + "internal/x/a.go:3.1,4.2 1 0\n"
	if err := os.WriteFile(f, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	b, err := readProfile(f)
	if err != nil {
		t.Fatal(err)
	}
	if ok, out := run(t, b, map[string]float64{"internal/x": 75}); !ok {
		t.Errorf("75%% at floor 75: %s", out)
	}
	if err := os.WriteFile(f, []byte("mode: set\nbroken\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readProfile(f); err == nil {
		t.Error("a broken line was read")
	}
}

// TestFloorsFile reads the repository's floors.
func TestFloorsFile(t *testing.T) {
	floors, err := readFloors("floors.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(floors) == 0 {
		t.Fatal("no floors")
	}
	dir := t.TempDir()
	for _, bad := range []string{"internal/x\n", "internal/x many\n"} {
		f := filepath.Join(dir, "floors.txt")
		if err := os.WriteFile(f, []byte(bad), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := readFloors(f); err == nil {
			t.Errorf("%q was read", bad)
		}
	}
}

// TestHAFloorHoldsItsSpread (HR-P4-test-62): internal/ha's floor holds
// both ends of its measured run-to-run spread (phase 3: 94.7 to 96.5),
// so the coverage job is not red on a high or a low run. The values are
// in tenths of a point.
func TestHAFloorHoldsItsSpread(t *testing.T) {
	floors, err := readFloors("floors.txt")
	if err != nil {
		t.Fatal(err)
	}
	for _, tenths := range []int{965, 959, 952, 947} {
		blocks := map[string]block{
			module + "internal/ha/a.go:1.1,2.2": {stmts: tenths, covered: true},
			module + "internal/ha/a.go:3.1,4.2": {stmts: 1000 - tenths, covered: false},
		}
		if ok, out := run(t, blocks, map[string]float64{"internal/ha": floors["internal/ha"]}); !ok {
			t.Errorf("internal/ha at %.1f, floor %.0f:\n%s", float64(tenths)/10, floors["internal/ha"], out)
		}
	}
}
