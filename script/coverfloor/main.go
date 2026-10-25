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
// The floors are a ratchet: raise one in the commit that raises the
// coverage, never lower one without a reason in that commit.
package main

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
)

const module = "github.com/SukramJ/go-ha2fabric/"

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
	failed := false
	for _, p := range pkgs {
		pct := 100 * float64(covered[p]) / float64(max(total[p], 1))
		floor, ok := floors[p]
		mark := "  "
		if ok && pct+1e-9 < floor {
			mark, failed = "✗ ", true
		}
		fmt.Printf("%s%-28s %6.1f%%  floor %5.1f%%\n", mark, p, pct, floor)
	}
	for p := range floors {
		if _, ok := total[p]; !ok {
			fmt.Printf("✗ %-28s has a floor but no statements in the profile\n", p)
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
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
