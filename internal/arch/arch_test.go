// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

// Package arch holds the module's architecture guard (brief H2, hard rule
// 1): go-fabric is used through its facade only — node, device, contract,
// eligibility and the <cluster>def packages under cluster/spec/ — and the
// daemon's own packages keep their layers. Tests may import fabrictest,
// the controller's client paths and clock besides. It is go-fabric's
// script/archcheck as a test, in the form of go-fabric-server's
// internal/arch.
package arch

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const (
	module   = "github.com/SukramJ/go-ha2fabric"
	goFabric = "github.com/SukramJ/go-fabric/"
)

// facade are the go-fabric packages the daemon may import.
var facade = map[string]bool{"node": true, "device": true, "contract": true, "eligibility": true}

// testOnly are the go-fabric packages a test may import besides: the
// in-process controller, its path helpers and the wall clock.
var testOnly = map[string]bool{"fabrictest": true, "controller/client": true, "clock": true}

// layers are the module's own import rules: a package may import only the
// module packages its entry lists.
var layers = map[string][]string{
	"internal/version":   {},
	"internal/expose":    {},
	"internal/selection": {},
	"internal/ha":        {},
	"internal/ha/hatest": {"internal/ha"},
	"internal/adapt":     {"internal/ha"},
	"internal/config":    {"internal/expose"},
	"internal/bridge":    {"internal/ha", "internal/adapt", "internal/expose", "internal/config", "internal/selection", "internal/version"},
	"internal/web":       {},
	"internal/arch":      {},
	"cmd/ha2fabric":      {"internal/bridge", "internal/config", "internal/ha", "internal/selection", "internal/version", "internal/web"},
}

// testLayers widen layers for a package's tests.
var testLayers = map[string][]string{
	"internal/ha":     {"internal/ha/hatest"},
	"internal/bridge": {"internal/ha/hatest"},
	"cmd/ha2fabric":   {"internal/ha/hatest"},
}

// Other modules the daemon may import: the mapping, its vocabulary, the
// catalog, the WebSocket library. Anything else outside the standard
// library is refused.
var thirdParty = []string{
	"github.com/SukramJ/go-hamapping",
	"github.com/SukramJ/go-hamqtt/model",
	"github.com/SukramJ/go-ha-catalog",
	"github.com/coder/websocket",
	"github.com/SukramJ/go-fabric-ui/fabricui",
}

func isDef(rel string) bool {
	return strings.HasPrefix(rel, "cluster/spec/") && !strings.Contains(strings.TrimPrefix(rel, "cluster/spec/"), "/")
}

// facadeImport reports whether a file may import go-fabric's dep.
func facadeImport(dep string, isTest bool) bool {
	return facade[dep] || isDef(dep) || isTest && testOnly[dep]
}

func TestImports(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	files := 0
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
		files++
		rel, _ := filepath.Rel(root, filepath.Dir(path))
		pkg := filepath.ToSlash(rel)
		isTest := strings.HasSuffix(path, "_test.go")
		where := filepath.ToSlash(strings.TrimPrefix(path, root+string(os.PathSeparator)))
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			switch {
			case strings.HasPrefix(p, goFabric):
				if !facadeImport(strings.TrimPrefix(p, goFabric), isTest) {
					t.Errorf("%s imports %s: below go-fabric's facade", where, p)
				}
			case p == module || strings.HasPrefix(p, module+"/"):
				dep := strings.TrimPrefix(strings.TrimPrefix(p, module), "/")
				allowed, ruled := layers[pkg]
				if !ruled {
					if pkg != "." {
						t.Errorf("%s: package %s has no layer entry", where, pkg)
					}
					continue
				}
				ok := dep == pkg
				for _, a := range append(allowed, testLayersFor(pkg, isTest)...) {
					ok = ok || dep == a
				}
				if !ok {
					t.Errorf("%s imports %s: outside the layers of %s", where, p, pkg)
				}
			case strings.Contains(strings.Split(p, "/")[0], "."):
				if !allowedThirdParty(p) {
					t.Errorf("%s imports %s: not a dependency of the daemon", where, p)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if files < 20 {
		t.Fatalf("walked %d files: the root is wrong", files)
	}
}

func allowedThirdParty(p string) bool {
	for _, a := range thirdParty {
		if p == a || strings.HasPrefix(p, a+"/") {
			return true
		}
	}
	return false
}

func testLayersFor(pkg string, isTest bool) []string {
	if !isTest {
		return nil
	}
	return testLayers[pkg]
}

// TestGuardRefuses holds the guard itself: what it must refuse, it does.
func TestGuardRefuses(t *testing.T) {
	for _, c := range []struct {
		dep  string
		test bool
	}{
		{"cluster/spec", false},
		{"cluster/spec/onoff/internal", false},
		{"bridge", false},
		{"endpoint", false},
		{"im", false},
		{"tlv", false},
		{"mdns", false},
		{"fabrictest", false},
		{"controller/client", false},
		{"controller", true},
		{"secure/setup", true},
	} {
		if facadeImport(c.dep, c.test) {
			t.Errorf("%s may not be imported (test %v)", c.dep, c.test)
		}
	}
	for _, c := range []struct {
		dep  string
		test bool
	}{
		{"node", false},
		{"device", false},
		{"contract", false},
		{"eligibility", false},
		{"cluster/spec/onoff", false},
		{"fabrictest", true},
		{"controller/client", true},
		{"clock", true},
	} {
		if !facadeImport(c.dep, c.test) {
			t.Errorf("%s may be imported (test %v)", c.dep, c.test)
		}
	}
	if allowedThirdParty("gopkg.in/yaml.v3") || !allowedThirdParty("github.com/SukramJ/go-hamqtt/model") || allowedThirdParty("github.com/SukramJ/go-hamqtt/discovery") {
		t.Error("third-party list")
	}
}
