// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// fakeModules is a module cache of three small modules.
func fakeModules(t *testing.T) map[string]*module {
	t.Helper()
	dir := t.TempDir()
	write := func(rel string, lines int) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(strings.Repeat("x\n", lines)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("gf/node/config.go", 1000)
	write("gf/docs/guides/camera.md", 50)
	write("gf/cluster/camera/state.go", 30)
	write("gf/node/state.go", 30)
	write("ui/fabricui/handler.go", 700)
	write("ui/contract/openapi.yaml", 10)
	write("hm/plan.go", 200)
	mods := map[string]*module{}
	for key, v := range map[string]string{"gf": "v0.26.0", "ui": "v0.14.1", "hm": "v0.8.0"} {
		files, err := localFiles(filepath.Join(dir, key))
		if err != nil {
			t.Fatal(err)
		}
		mods[key] = &module{key: key, path: key, version: v, dir: filepath.Join(dir, key), files: files}
	}
	return mods
}

// local is this repository's files as the tests see them.
var local = []string{"internal/plan/plan.go", "internal/config/config.go", "docs/by_design.md"}

func problems(t *testing.T, src string, mods map[string]*module) []string {
	t.Helper()
	var out []string
	for _, b := range textBlocks(src) {
		for _, c := range cites("x.md", b, mods, local) {
			out = append(out, c.mod+" "+c.path+colonSpans(c.spans)+" "+orOK(check(c, mods)))
		}
	}
	return out
}

func TestCites(t *testing.T) {
	mods := fakeModules(t)
	for _, c := range []struct {
		src  string
		want []string
	}{
		// Pinned at go.mod's version, in range.
		{"go-fabric node/config.go:956-965 at v0.26.0.", []string{"gf node/config.go:956-965 ok"}},
		// An old pin, and lines past the end.
		{"go-fabric node/config.go:958-966 at v0.25.0.", []string{"gf node/config.go:958-966 PROBLEM: names v0.25.0, go.mod has v0.26.0"}},
		{"go-fabric node/config.go:995-1001 at v0.26.0.", []string{"gf node/config.go:995-1001 PROBLEM: lines 995-1001 are not in node/config.go (1000 lines) at v0.26.0"}},
		// No version.
		{"go-fabric (docs/guides/camera.md:10-12).", []string{"gf docs/guides/camera.md:10-12 PROBLEM: names no version (go.mod: v0.26.0)"}},
		// A continuation cites the same file; a core range after it does not.
		{"go-fabric docs/guides/camera.md:10-12 at v0.26.0, and :40-60.", []string{"gf docs/guides/camera.md:10-12,40-60 PROBLEM: lines 40-60 are not in docs/guides/camera.md (50 lines) at v0.26.0"}},
		{"go-fabric docs/guides/camera.md:10 at v0.26.0; core services.py:137-148, :193-203.", []string{"gf docs/guides/camera.md:10 ok"}},
		// A version is not a line: "2026.10.3".
		{"go-fabric docs/guides/camera.md:10, 2026.10.3, at v0.26.0", []string{"gf docs/guides/camera.md:10 ok"}},
		// go-fabric-ui is not go-fabric.
		{"go-fabric-ui fabricui/handler.go:601-610 at v0.14.1", []string{"ui fabricui/handler.go:601-610 ok"}},
		// A bare name: go-hamapping's with a version, this repository's
		// without one.
		{"go-hamapping plan.go:160-168 at v0.8.0", []string{"hm plan.go:160-168 ok"}},
		{"go-hamapping's Map (plan.go:12)", nil},
		{"the docs (docs/by_design.md) of go-hamapping", nil},
		// No module named, but the path is one module's exactly.
		{"the kit (cluster/camera/state.go:13-28 at v0.26.0)", []string{"gf cluster/camera/state.go:13-28 ok"}},
		// An ambiguous suffix is no citation.
		{"go-fabric state.go:13-28 at v0.26.0", nil},
		// This repository's paths are not the module's.
		{"go-fabric is wrapped in internal/config/config.go:12.", nil},
		// A prefix names the module.
		{"see hm:plan.go:1 at v0.8.0", []string{"hm plan.go:1 ok"}},
		// The go.mod marker, for a file read at go.mod's version, without lines only.
		{"go-fabric-ui's contract (contract/openapi.yaml at go.mod's version)", []string{"ui contract/openapi.yaml ok"}},
		{"go-fabric-ui's contract (contract/openapi.yaml:3 at go.mod's version)", []string{"ui contract/openapi.yaml:3 PROBLEM: names go.mod's version, which a citation of lines may not: name v0.14.1"}},
	} {
		if got := problems(t, c.src, mods); !slices.Equal(got, c.want) {
			t.Errorf("%q:\n got  %q\n want %q", c.src, got, c.want)
		}
	}
}

func TestBlocks(t *testing.T) {
	src := "# Title go-fabric\n\npara one\nstill one\n- item\n- item two\n\n| a | b |\n"
	b := textBlocks(src)
	if len(b) != 5 {
		t.Fatalf("%d blocks: %v", len(b), b)
	}
	if b[1][1].n != 4 || b[1][1].text != "still one" {
		t.Errorf("paragraph: %v", b[1])
	}
	g, err := goBlocks("x.go", []byte("package x\n\n// go-fabric node/config.go:1\n// at v0.26.0.\nvar x = 1 // trailing\n\n/* block\nline */\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(g) != 3 || g[0][1].n != 4 || len(g[2]) != 2 {
		t.Errorf("go blocks: %v", g)
	}
	mods := fakeModules(t)
	cs := cites("x.go", g[0], mods, local)
	if len(cs) != 1 || cs[0].line != 3 || cs[0].version != "v0.26.0" {
		t.Errorf("a pin on the next line: %+v", cs)
	}
}

func TestWalk(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"a.go", "docs/x.md", "docs/reports/R7.md", "notes/n.md", "changelog.md", ".git/c.md", "bin/x.bin"} {
		p := filepath.Join(dir, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)
	got, err := walk(".")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a.go", "docs/x.md"}; !slices.Equal(got, want) {
		t.Errorf("walked %v, want %v", got, want)
	}
}

func TestParseModules(t *testing.T) {
	dir := t.TempDir()
	stream := ""
	for _, m := range modules {
		d := filepath.Join(dir, m.key)
		if err := os.MkdirAll(d, 0o750); err != nil {
			t.Fatal(err)
		}
		stream += `{"Path":"` + m.path + `","Version":"v1.0.0","Dir":"` + filepath.ToSlash(d) + `"}` + "\n"
	}
	mods, err := parseModules([]byte(stream))
	if err != nil || len(mods) != 3 || mods["ui"].version != "v1.0.0" {
		t.Fatalf("%v, %v", mods, err)
	}
	if _, err := parseModules([]byte(`{"Path":"x","Error":"not found"}`)); err == nil {
		t.Error("an error entry was read")
	}
	if _, err := parseModules(nil); err == nil {
		t.Error("a stream without the modules was read")
	}
}

// TestRepository runs the check over the repository's code, as make
// cite does: it holds at go.mod's versions.
func TestRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("reads the module cache")
	}
	mods, err := loadModules()
	if err != nil {
		t.Skip("module cache:", err)
	}
	root := filepath.Join("..", "..")
	all, err := localFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, dir := range []string{"cmd", "internal", "testdata", "script"} {
		files, err := walk(filepath.Join(root, dir))
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			blocks, err := fileBlocks(f)
			if err != nil {
				t.Fatal(err)
			}
			for _, b := range blocks {
				for _, c := range cites(f, b, mods, all) {
					n++
					if p := check(c, mods); p != "" {
						t.Errorf("%s:%d: %s: %s", c.file, c.line, c.path, p)
					}
				}
			}
		}
	}
	if n < 40 {
		t.Errorf("%d citations found: the parser lost some", n)
	}
}
