// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

// Command coverfloor checks the statement coverage of each package
// against its floor. It reads a profile of
// `go test -coverpkg=./... -coverprofile=FILE ./...` — every test binary's
// blocks, a block covered when any binary covered it — and floors.txt
// beside this file ("<package> <percent>" per line, # comments).
//
// Usage: go run ./script/coverfloor FILE
//
// The floors are a ratchet, and the check holds it both ways: a package
// below its floor fails, a floor more than staleFloorSlack points under
// the measurement fails (it has stopped detecting a regression of that
// size), and a package with statements and no floor fails. Raise a floor
// in the commit that raises the coverage, to the whole percent below the
// lowest of two runs; never lower one without a reason in that commit.
package main

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
)

const module = "thingfabric.dev/go-ha2fabric/"

// staleFloorSlack is how far a measurement may sit above its floor, as
// go-fabric's script/coverfloor/main.go:73-81 at v0.28.0
// (staleFloorSlack; review HR-P3-test-01): beyond it a regression of that many
// points passes. Three points hold the run-to-run spread (0.3-0.6 points
// measured here, internal/ha's 1.8: HR-P4-test-62) of a floor at the
// whole percent below the lowest run. A
// zero floor has no slack: a package at 0 that measures anything has
// gained tested code and needs a real floor.
const staleFloorSlack = 3.0

type block struct {
	stmts   int
	covered bool
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: coverfloor PROFILE")
		os.Exit(2)
	}
	blocks, err := readProfile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "coverfloor:", err)
		os.Exit(2)
	}
	floors, err := readFloors("script/coverfloor/floors.txt")
	if err != nil {
		fmt.Fprintln(os.Stderr, "coverfloor:", err)
		os.Exit(2)
	}
	if !check(os.Stdout, blocks, floors) {
		os.Exit(1)
	}
}

// check writes one line per package and every problem to w, and reports
// whether there was none.
func check(w io.Writer, blocks map[string]block, floors map[string]float64) bool {
	total, covered := map[string]int{}, map[string]int{}
	for key, b := range blocks {
		pkg := path.Dir(strings.TrimPrefix(key[:strings.IndexByte(key, ':')], module))
		total[pkg] += b.stmts
		if b.covered {
			covered[pkg] += b.stmts
		}
	}
	pkgs := make([]string, 0, len(total))
	for p := range total {
		pkgs = append(pkgs, p)
	}
	sort.Strings(pkgs)
	var problems []string
	for _, p := range pkgs {
		if total[p] == 0 {
			continue
		}
		pct := 100 * float64(covered[p]) / float64(total[p])
		floor, ok := floors[p]
		mark := "  "
		switch {
		case !ok:
			mark = "✗ "
			problems = append(problems, fmt.Sprintf("%s has %d statements and no floor: add \"%s %.0f\" to script/coverfloor/floors.txt", p, total[p], p, math.Floor(pct)))
		case pct+1e-9 < floor:
			mark = "✗ "
			problems = append(problems, fmt.Sprintf("%s: %.1f%% < floor %.1f%%: restore the coverage, or lower the floor deliberately and say why", p, pct, floor))
		default:
			slack := staleFloorSlack
			if floor == 0 {
				slack = 0
			}
			if pct-floor > slack+1e-9 {
				mark = "✗ "
				problems = append(problems, fmt.Sprintf("%s is %.1f points above its floor (more than %.1f): raise it to %.0f in script/coverfloor/floors.txt to keep the ratchet biting", p, pct-floor, slack, math.Floor(pct)))
			}
		}
		_, _ = fmt.Fprintf(w, "%s%-28s %6.1f%%  floor %5.1f%%\n", mark, p, pct, floor)
	}
	stale := make([]string, 0, len(floors))
	for p := range floors {
		if total[p] == 0 {
			stale = append(stale, p)
		}
	}
	sort.Strings(stale)
	for _, p := range stale {
		problems = append(problems, p+" has a floor but no statements in the profile: delete the entry, or profile the whole module")
	}
	for _, m := range problems {
		_, _ = fmt.Fprintln(w, "✗", m)
	}
	return len(problems) == 0
}

func readProfile(name string) (map[string]block, error) {
	f, err := os.Open(name) //nolint:gosec // the profile the caller names
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	blocks := map[string]block{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "mode:") || line == "" {
			continue
		}
		// file:a.b,c.d stmts count
		fields := strings.Fields(line)
		if len(fields) != 3 {
			return nil, fmt.Errorf("profile line %q", line)
		}
		stmts, err1 := strconv.Atoi(fields[1])
		count, err2 := strconv.Atoi(fields[2])
		if err1 != nil || err2 != nil {
			return nil, fmt.Errorf("profile line %q", line)
		}
		b := blocks[fields[0]]
		b.stmts = stmts
		b.covered = b.covered || count > 0
		blocks[fields[0]] = b
	}
	return blocks, sc.Err()
}

func readFloors(name string) (map[string]float64, error) {
	b, err := os.ReadFile(name) //nolint:gosec // the repository's floors file
	if err != nil {
		return nil, err
	}
	out := map[string]float64{}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("floors line %q", line)
		}
		v, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			return nil, fmt.Errorf("floors line %q", line)
		}
		out[fields[0]] = v
	}
	return out, nil
}
