// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

// Command cite checks the repository's citations of go-fabric,
// go-fabric-ui and go-hamapping (decision 61 of the 2026-11-23 review,
// HR-P3-cite-60..62): each names go.mod's version of its module, and the
// file and lines it cites exist there, in the module cache. A bump of
// go.mod therefore fails here until every citation is read again and
// re-pinned.
//
// Usage: go run ./script/cite [-list] [PATH...]
//
// PATH is a file or a directory (walked, notes/, held/, .git and the
// history files left out: changelog.md, addon/CHANGELOG.md, docs/reports/);
// the default is the working directory. -list prints every citation the
// check recognised, checked or not.
//
// What it recognises. A citation is a path with a dot extension and line
// numbers ("dir/file.go" with ":12-34", or ":12-34, :56" for two ranges)
// or a path alone. It belongs to a module when its
// block (a Go comment group; a paragraph, list item or table row of a
// text file) names the module before it — "go-fabric", "go-fabric-ui" or
// "fabricui", "go-hamapping", or a gf:/ui:/hm: prefix on the path — and
// the path resolves in that module, as it is or as the one file whose
// path ends with it. Its version is the first vX.Y.Z after it in the
// block. A path that does not resolve in the module but exists in this
// repository is this repository's and is skipped.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// module is one cited module at go.mod's version.
type module struct {
	key, path, version, dir string
	files                   []string // slash paths relative to dir
}

// modules are the cited modules, longest name first so "go-fabric-ui"
// is not read as "go-fabric".
var modules = []struct{ key, path string }{
	{"ui", "thingfabric.dev/go-fabric-ui"},
	{"hm", "sukramj.dev/go-hamapping"},
	{"gf", "thingfabric.dev/go-fabric"},
}

// mention finds a module named in a block; its group says which.
var mention = regexp.MustCompile(`go-fabric-ui|fabricui|go-hamapping|go-fabric|\b(?:gf|ui|hm):`)

func mentionKey(m string) string {
	switch m {
	case "go-fabric-ui", "fabricui", "ui:":
		return "ui"
	case "go-hamapping", "hm:":
		return "hm"
	default:
		return "gf"
	}
}

// pathRe finds a path with a file extension.
var pathRe = regexp.MustCompile(`(?:\b(?:gf|ui|hm):)?((?:[A-Za-z0-9_.\-]+/)*[A-Za-z0-9_\-][A-Za-z0-9_.\-]*\.(?:go|md|yaml|yml|json|txt|ts|js|sh|sql|proto|py|xml|h|cpp|c|toml|html|css|svelte))\b`)

// contRe finds a continuation ":123" or ":12-34" that cites the last path
// again.
var contRe = regexp.MustCompile(`(?:^|[\s,(;])(:\s?\d+(?:-\d+)?)`)

// versionRe finds a module version; versionReach is how far after a path
// it is looked for (a sentence, as unit C's probe read it).
var versionRe = regexp.MustCompile(`\bv\d+\.\d+\.\d+\b`)

const versionReach = 300

// goModVersion stands for "at go.mod's version": a file the code reads at
// whatever version go.mod pins, cited without lines.
const goModVersion = "go.mod's version"

// versionOrMarker finds a version, or the go.mod marker, in s.
func versionOrMarker(s string) string {
	v := versionRe.FindStringIndex(s)
	m := strings.Index(s, goModVersion)
	switch {
	case m >= 0 && (len(v) == 0 || m < v[0]):
		return goModVersion
	case len(v) == 2:
		return s[v[0]:v[1]]
	}
	return ""
}

// span is a cited line range.
type span struct{ from, to int }

// citation is one recognised citation.
type citation struct {
	file    string // where, relative to the root
	line    int
	mod     string // module key
	path    string // as written
	spans   []span
	version string // the pin, "" when none
	// missing is a path the module named before it does not have
	// (HR-P4-test-63).
	missing bool
}

// line is one line of a block: its number and its text.
type line struct {
	n    int
	text string
}

func main() {
	list := flag.Bool("list", false, "print every recognised citation")
	flag.Parse()
	mods, err := loadModules(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, "cite:", err)
		os.Exit(2)
	}
	roots := flag.Args()
	if len(roots) == 0 {
		roots = []string{"."}
	}
	local, err := localFiles(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, "cite:", err)
		os.Exit(2)
	}
	var files []string
	for _, r := range roots {
		found, err := walk(r)
		if err != nil {
			fmt.Fprintln(os.Stderr, "cite:", err)
			os.Exit(2)
		}
		files = append(files, found...)
	}
	problems := 0
	for _, f := range files {
		blocks, err := fileBlocks(f)
		if err != nil {
			fmt.Fprintln(os.Stderr, "cite:", err)
			os.Exit(2)
		}
		for _, b := range blocks {
			for _, c := range cites(f, b, mods, local) {
				p := check(c, mods)
				if *list {
					fmt.Printf("%s:%d: %s %s:%s %s %s\n", c.file, c.line, c.mod, c.path, spansString(c.spans), orNone(c.version), orOK(p))
				} else if p != "" {
					fmt.Printf("%s:%d: %s %s%s: %s\n", c.file, c.line, mods[c.mod].path, c.path, colonSpans(c.spans), p)
				}
				if p != "" {
					problems++
				}
			}
		}
	}
	if problems > 0 {
		fmt.Printf("cite: %d citation(s) of go-fabric, go-fabric-ui or go-hamapping do not hold at go.mod's version\n", problems)
		os.Exit(1)
	}
}

func orNone(v string) string {
	if v == "" {
		return "(no version)"
	}
	return v
}

func orOK(p string) string {
	if p == "" {
		return "ok"
	}
	return "PROBLEM: " + p
}

// loadModules reads go.mod's version and directory of each cited module
// (go mod download, which fetches it when the cache lacks it).
func loadModules(ctx context.Context) (map[string]*module, error) {
	args := make([]string, 0, 3+len(modules))
	args = append(args, "mod", "download", "-json")
	for _, m := range modules {
		args = append(args, m.path)
	}
	out, err := exec.CommandContext(ctx, "go", args...).Output() //nolint:gosec // fixed arguments
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return nil, fmt.Errorf("go mod download: %w: %s", err, ee.Stderr)
		}
		return nil, fmt.Errorf("go mod download: %w", err)
	}
	return parseModules(out)
}

// parseModules reads go mod download -json's stream and lists each
// module's files.
func parseModules(out []byte) (map[string]*module, error) {
	mods := map[string]*module{}
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var m struct{ Path, Version, Dir, Error string }
		if err := dec.Decode(&m); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, err
		}
		if m.Error != "" {
			return nil, fmt.Errorf("%s: %s", m.Path, m.Error)
		}
		for _, k := range modules {
			if k.path != m.Path {
				continue
			}
			files, err := localFiles(m.Dir)
			if err != nil {
				return nil, err
			}
			mods[k.key] = &module{key: k.key, path: m.Path, version: m.Version, dir: m.Dir, files: files}
		}
	}
	for _, k := range modules {
		if mods[k.key] == nil {
			return nil, fmt.Errorf("%s is not in go.mod", k.path)
		}
	}
	return mods, nil
}

// localFiles lists the files under root as slash paths, hidden
// directories left out.
func localFiles(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && p != root {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(out)
	return out, err
}

// history are the files and directories whose citations record a past
// state and keep the version they name.
var history = []string{"notes", "held", "changelog.md", "addon/CHANGELOG.md", "docs/reports"}

// walk lists the checked files under root.
func walk(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := filepath.ToSlash(filepath.Clean(p))
		if d.IsDir() {
			if p != root && (strings.HasPrefix(d.Name(), ".") || slices.Contains(history, rel)) {
				return filepath.SkipDir
			}
			return nil
		}
		if slices.Contains(history, rel) {
			return nil
		}
		switch filepath.Ext(p) {
		case ".go", ".md", ".json", ".yaml", ".yml", ".sh", ".txt":
			out = append(out, rel)
		}
		return nil
	})
	return out, err
}

// fileBlocks splits a file into its blocks: a Go file's comment groups,
// a text file's paragraphs, list items, table rows and headings.
func fileBlocks(name string) ([][]line, error) {
	src, err := os.ReadFile(name) //nolint:gosec // the files the caller names
	if err != nil {
		return nil, err
	}
	if strings.HasSuffix(name, ".go") {
		return goBlocks(name, src)
	}
	return textBlocks(string(src)), nil
}

func goBlocks(name string, src []byte) ([][]line, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	var out [][]line
	for _, g := range f.Comments {
		var b []line
		for _, c := range g.List {
			n := fset.Position(c.Slash).Line
			text := c.Text
			if strings.HasPrefix(text, "/*") {
				for i, l := range strings.Split(strings.TrimSuffix(strings.TrimPrefix(text, "/*"), "*/"), "\n") {
					b = append(b, line{n + i, l})
				}
				continue
			}
			b = append(b, line{n, strings.TrimPrefix(text, "//")})
		}
		out = append(out, b)
	}
	return out, nil
}

var itemRe = regexp.MustCompile(`^\s*(?:[-*+]|\d+\.)\s`)

func textBlocks(src string) [][]line {
	var out [][]line
	var cur []line
	flush := func() {
		if len(cur) > 0 {
			out = append(out, cur)
			cur = nil
		}
	}
	for i, l := range strings.Split(src, "\n") {
		t := strings.TrimSpace(l)
		switch {
		case t == "":
			flush()
		case strings.HasPrefix(t, "|") || strings.HasPrefix(t, "#") && !strings.HasPrefix(t, "#!"):
			flush()
			out = append(out, []line{{i + 1, l}})
		case itemRe.MatchString(l):
			flush()
			cur = append(cur, line{i + 1, l})
		default:
			cur = append(cur, line{i + 1, l})
		}
	}
	flush()
	return out
}

// cites are the module citations of one block.
func cites(file string, b []line, mods map[string]*module, local []string) []citation {
	var text strings.Builder
	starts := make([]int, 0, len(b))
	for _, l := range b {
		starts = append(starts, text.Len())
		text.WriteString(l.text)
		text.WriteByte(' ')
	}
	s := text.String()
	lineAt := func(off int) int {
		i := sort.SearchInts(starts, off+1) - 1
		return b[max(i, 0)].n
	}
	var out []citation
	paths := pathRe.FindAllStringSubmatchIndex(s, -1)
	for i, m := range paths {
		p := s[m[2]:m[3]]
		version := versionOrMarker(s[m[1]:min(len(s), m[1]+versionReach)])
		key, missing := attribute(s[:m[0]], s[m[0]:m[2]], p, version, mods, local)
		if key == "" {
			continue
		}
		end := len(s)
		if i+1 < len(paths) {
			end = paths[i+1][0]
		}
		spans, rest := readSpans(s[m[1]:end])
		if missing {
			// Only a pin of this path: lines, and no other module named
			// between the path and the version ("… (rows/camera.yaml at
			// v0.8.0)" after a go-hamapping mention is not this path's).
			tail := s[m[1]:min(len(s), m[1]+versionReach)]
			before, _, _ := strings.Cut(tail, version)
			if len(spans) == 0 || mention.MatchString(before) {
				continue
			}
		}
		// Continuations cite the same file again: ", :122-127".
		after := m[1] + rest
		for _, c := range contRe.FindAllStringSubmatchIndex(s[after:end], -1) {
			sp, _ := readSpans(s[after+c[2] : end])
			spans = append(spans, sp...)
		}
		out = append(out, citation{file: file, line: lineAt(m[0]), mod: key, path: p, spans: spans, version: version, missing: missing})
	}
	return out
}

// attribute says which module a path cites ("" for none): the prefix
// gf:/ui:/hm: when it has one; else the module the block names last
// before it, when the path resolves there and is not this repository's
// own (a path this repository has, with no version, is its own); else
// the one module whose tree has the path exactly, when it has a
// directory and this repository has no such file. missing is a path with
// a directory and a vX.Y.Z pin after the module the block names last,
// which neither that module nor any other nor this repository has: a
// file a bump renamed, moved or deleted, or a typo (HR-P4-test-63); the
// caller counts it only when it cites lines.
func attribute(before, prefix, p, version string, mods map[string]*module, local []string) (key string, missing bool) {
	if prefix != "" {
		return strings.TrimSuffix(prefix, ":"), false
	}
	own := hasSuffixPath(local, p)
	named := ""
	if ms := mention.FindAllString(before, -1); len(ms) > 0 {
		named = mentionKey(ms[len(ms)-1])
		if _, ok := resolve(mods[named].files, p); ok && (version != "" || !own) {
			return named, false
		}
	}
	if !strings.Contains(p, "/") || own {
		return "", false
	}
	hit := ""
	for _, k := range modules {
		if _, ok := slices.BinarySearch(mods[k.key].files, p); ok {
			if hit != "" {
				return "", false
			}
			hit = k.key
		}
	}
	if hit == "" && named != "" && strings.HasPrefix(version, "v") && !anySuffix(mods, p) {
		return named, true
	}
	return hit, false
}

// moduleName is the last element of the module path of key:
// "go-fabric" for gf.
func moduleName(key string) string {
	for _, k := range modules {
		if k.key == key {
			return k.path[strings.LastIndex(k.path, "/")+1:]
		}
	}
	return key
}

// anySuffix reports whether a module's tree has p as the end of a path.
func anySuffix(mods map[string]*module, p string) bool {
	for _, k := range modules {
		if hasSuffixPath(mods[k.key].files, p) {
			return true
		}
	}
	return false
}

// readSpans reads ":12-34, 56" at the start of s and returns the spans
// and how far it read; a number followed by "." and a digit is a version,
// not a line.
func readSpans(s string) (spans []span, read int) {
	i := 0
	skip := func() {
		for i < len(s) && (s[i] == ' ' || s[i] == '`') {
			i++
		}
	}
	if i >= len(s) || s[i] != ':' {
		return nil, 0
	}
	i++
	read = i
	for {
		skip()
		j := i
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		if j == i {
			break
		}
		from, _ := strconv.Atoi(s[i:j])
		to := from
		k := j
		if k+1 < len(s) && s[k] == '-' && s[k+1] >= '0' && s[k+1] <= '9' {
			k++
			e := k
			for e < len(s) && s[e] >= '0' && s[e] <= '9' {
				e++
			}
			to, _ = strconv.Atoi(s[k:e])
			k = e
		}
		if k+1 < len(s) && s[k] == '.' && s[k+1] >= '0' && s[k+1] <= '9' {
			break // a version such as 2026.10.3
		}
		spans = append(spans, span{from, to})
		i, read = k, k
		if i < len(s) && s[i] == ',' {
			n := i + 1
			for n < len(s) && s[n] == ' ' {
				n++
			}
			if n < len(s) && s[n] >= '0' && s[n] <= '9' {
				i = n
				continue
			}
		}
		break
	}
	return spans, read
}

// resolve finds path in files: as it is, or as the one file whose path
// ends with "/"+path.
func resolve(files []string, path string) (string, bool) {
	if _, ok := slices.BinarySearch(files, path); ok {
		return path, true
	}
	var hit string
	for _, f := range files {
		if strings.HasSuffix(f, "/"+path) {
			if hit != "" {
				return "", false
			}
			hit = f
		}
	}
	return hit, hit != ""
}

func hasSuffixPath(files []string, path string) bool {
	for _, f := range files {
		if f == path || strings.HasSuffix(f, "/"+path) {
			return true
		}
	}
	return false
}

// check says what is wrong with c at go.mod's version, or "".
func check(c citation, mods map[string]*module) string {
	mod := mods[c.mod]
	if c.missing {
		return "not in " + moduleName(c.mod) + " at " + mod.version
	}
	var problems []string
	switch {
	case c.version == goModVersion && len(c.spans) == 0:
		// A file the code reads at whatever version go.mod pins.
	case c.version == goModVersion:
		problems = append(problems, "names go.mod's version, which a citation of lines may not: name "+mod.version)
	case c.version == "":
		problems = append(problems, "names no version (go.mod: "+mod.version+")")
	case c.version != mod.version:
		problems = append(problems, "names "+c.version+", go.mod has "+mod.version)
	}
	full, _ := resolve(mod.files, c.path)
	if len(c.spans) > 0 {
		n, err := lineCount(filepath.Join(mod.dir, full))
		if err != nil {
			return err.Error()
		}
		for _, sp := range c.spans {
			if sp.from < 1 || sp.to < sp.from || sp.to > n {
				problems = append(problems, fmt.Sprintf("lines %d-%d are not in %s (%d lines) at %s", sp.from, sp.to, full, n, mod.version))
			}
		}
	}
	return strings.Join(problems, "; ")
}

func lineCount(name string) (int, error) {
	b, err := os.ReadFile(name) //nolint:gosec // a file of the module cache
	if err != nil {
		return 0, err
	}
	n := bytes.Count(b, []byte("\n"))
	if len(b) > 0 && b[len(b)-1] != '\n' {
		n++
	}
	return n, nil
}

func spansString(sp []span) string {
	parts := make([]string, 0, len(sp))
	for _, s := range sp {
		if s.from == s.to {
			parts = append(parts, strconv.Itoa(s.from))
		} else {
			parts = append(parts, fmt.Sprintf("%d-%d", s.from, s.to))
		}
	}
	return strings.Join(parts, ",")
}

func colonSpans(sp []span) string {
	if len(sp) == 0 {
		return ""
	}
	return ":" + spansString(sp)
}
