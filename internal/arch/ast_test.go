// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package arch

import (
	"bufio"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// pureStd are the standard-library packages the pure layers' code may
// import (HR-P3-api-04): internal/plan ("pure, with no node and no
// goroutines", CLAUDE.md, Architecture) and internal/expose, the rule's
// other pure package. No sync, os, time, net, io: no state, no clock, no
// I/O. A package added here is a decision about the layer.
var pureStd = map[string][]string{
	"internal/plan":   {"encoding/json", "fmt", "maps", "slices", "sort", "strings"},
	"internal/expose": {"fmt", "maps", "path", "slices", "strings"},
}

// envReaders are the packages whose code may read the process
// environment (HR-P3-api-07; CLAUDE.md, Rules, Credentials): the
// configuration alone (config.OSEnv), so the tokens reach every other
// package through config.Config.
var envReaders = map[string]bool{"internal/config": true}

// envFuncs are the environment's readers.
var envFuncs = map[string][]string{
	"os":      {"Getenv", "LookupEnv", "Environ", "ExpandEnv"},
	"syscall": {"Getenv", "Environ"},
}

// goFile is one parsed Go file of the module.
type goFile struct {
	pkg, where string
	test       bool
	f          *ast.File
}

// moduleFiles parses every Go file of the module, testdata and hidden
// directories left out.
func moduleFiles(t *testing.T) []goFile {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var out []goFile
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); name == "testdata" || strings.HasPrefix(name, ".") && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, filepath.Dir(path))
		out = append(out, goFile{
			pkg:   filepath.ToSlash(rel),
			where: filepath.ToSlash(strings.TrimPrefix(path, root+string(os.PathSeparator))),
			test:  strings.HasSuffix(path, "_test.go"),
			f:     f,
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) < 20 {
		t.Fatalf("parsed %d files: the root is wrong", len(out))
	}
	return out
}

// pureProblems are what a file of a pure package must not have: a
// standard-library import outside allowed, and a go statement.
func pureProblems(f *ast.File, allowed []string) []string {
	var out []string
	for _, imp := range f.Imports {
		p, _ := strconv.Unquote(imp.Path.Value)
		if !strings.Contains(strings.Split(p, "/")[0], ".") && !slices.Contains(allowed, p) {
			out = append(out, "imports "+p)
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		if _, ok := n.(*ast.GoStmt); ok {
			out = append(out, "starts a goroutine")
		}
		return true
	})
	return out
}

// envReads are the calls of f that read the process environment.
func envReads(f *ast.File) []string {
	names := map[string]string{} // local name → import path
	for _, imp := range f.Imports {
		p, _ := strconv.Unquote(imp.Path.Value)
		if _, ok := envFuncs[p]; !ok {
			continue
		}
		name := p
		if imp.Name != nil {
			name = imp.Name.Name
		}
		names[name] = p
	}
	var out []string
	ast.Inspect(f, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		id, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		if p, ok := names[id.Name]; ok && slices.Contains(envFuncs[p], sel.Sel.Name) {
			out = append(out, p+"."+sel.Sel.Name)
		}
		return true
	})
	return out
}

// TestPure holds the pure layers (HR-P3-api-04).
func TestPure(t *testing.T) {
	seen := map[string]bool{}
	for _, g := range moduleFiles(t) {
		allowed, ok := pureStd[g.pkg]
		if !ok || g.test {
			continue
		}
		seen[g.pkg] = true
		for _, p := range pureProblems(g.f, allowed) {
			t.Errorf("%s %s: %s is pure", g.where, p, g.pkg)
		}
	}
	for pkg := range pureStd {
		if !seen[pkg] {
			t.Errorf("%s has no files: the rule is stale", pkg)
		}
	}
}

// TestEnvironment holds "credentials from the environment only" to one
// reader (HR-P3-api-07): no code outside envReaders reads the process
// environment. Tests may.
func TestEnvironment(t *testing.T) {
	for _, g := range moduleFiles(t) {
		if g.test || envReaders[g.pkg] {
			continue
		}
		for _, call := range envReads(g.f) {
			t.Errorf("%s calls %s: the environment is internal/config's to read (config.Load)", g.where, call)
		}
	}
}

// fuzzTargets reads the Makefile's FUZZ_TARGETS as "<dir>:<name>".
func fuzzTargets(t *testing.T, makefile string) []string {
	t.Helper()
	f, err := os.Open(makefile) //nolint:gosec // the repository's Makefile
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	var value strings.Builder
	in := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if !in {
			v, ok := strings.CutPrefix(line, "FUZZ_TARGETS :=")
			if !ok {
				continue
			}
			in, line = true, v
		}
		more := strings.HasSuffix(strings.TrimSpace(line), "\\")
		value.WriteString(strings.TrimSuffix(strings.TrimSpace(line), "\\") + " ")
		if !more {
			break
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, tgt := range strings.Fields(value.String()) {
		dir, name, ok := strings.Cut(tgt, ":")
		if !ok {
			t.Fatalf("FUZZ_TARGETS entry %q is not <package>:<name>", tgt)
		}
		out = append(out, strings.Trim(strings.TrimPrefix(dir, "./"), "/")+":"+name)
	}
	sort.Strings(out)
	return out
}

// fuzzFuncs are the module's fuzz targets as "<dir>:<name>".
func fuzzFuncs(files []goFile) []string {
	var out []string
	for _, g := range files {
		if !g.test {
			continue
		}
		for _, d := range g.f.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok && fn.Recv == nil && strings.HasPrefix(fn.Name.Name, "Fuzz") {
				out = append(out, g.pkg+":"+fn.Name.Name)
			}
		}
	}
	sort.Strings(out)
	return out
}

// TestFuzzTargets holds the Makefile's FUZZ_TARGETS equal to the module's
// fuzz targets (HR-P3-test-13): one left out never runs in CI.
func TestFuzzTargets(t *testing.T) {
	listed := fuzzTargets(t, filepath.Join("..", "..", "Makefile"))
	found := fuzzFuncs(moduleFiles(t))
	for _, f := range found {
		if !slices.Contains(listed, f) {
			t.Errorf("fuzz target %s is not in the Makefile's FUZZ_TARGETS", f)
		}
	}
	for _, l := range listed {
		if !slices.Contains(found, l) {
			t.Errorf("FUZZ_TARGETS names %s, which is no fuzz target", l)
		}
	}
	if len(found) == 0 {
		t.Error("no fuzz target found")
	}
}

// TestASTGuardsRefuse holds the guards above on what they must refuse.
func TestASTGuardsRefuse(t *testing.T) {
	parse := func(src string) *ast.File {
		f, err := parser.ParseFile(token.NewFileSet(), "x.go", src, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		return f
	}
	for _, src := range []string{
		"package plan\nimport _ \"sync\"\n",
		"package plan\nimport _ \"os\"\n",
		"package plan\nimport _ \"time\"\n",
		"package plan\nimport _ \"net\"\n",
		"package plan\nfunc init() { go func() {}() }\n",
	} {
		if len(pureProblems(parse(src), pureStd["internal/plan"])) == 0 {
			t.Errorf("not refused as pure: %q", src)
		}
	}
	if p := pureProblems(parse("package plan\nimport (\n\t\"fmt\"\n\t\"sukramj.dev/go-hamapping\"\n)\n"), pureStd["internal/plan"]); len(p) != 0 {
		t.Errorf("refused: %v", p)
	}
	for _, src := range []string{
		"package main\nimport \"os\"\nvar _ = os.Getenv(\"SUPERVISOR_TOKEN\")\n",
		"package main\nimport \"os\"\nvar _, _ = os.LookupEnv(\"HA_TOKEN\")\n",
		"package main\nimport \"os\"\nvar _ = os.Environ()\n",
		"package main\nimport sys \"syscall\"\nvar _, _ = sys.Getenv(\"HA_TOKEN\")\n",
	} {
		if len(envReads(parse(src))) == 0 {
			t.Errorf("not refused as an environment read: %q", src)
		}
	}
	if r := envReads(parse("package main\nimport \"os\"\nvar _ = os.Args\n")); len(r) != 0 {
		t.Errorf("refused: %v", r)
	}
	if !slices.Equal(fuzzFuncs([]goFile{{pkg: "internal/x", test: true, f: parse("package x\nfunc FuzzX(f *testing.F) {}\n")}}), []string{"internal/x:FuzzX"}) {
		t.Error("a fuzz target was not found")
	}
}
