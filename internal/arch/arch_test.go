// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

// Package arch holds the module's architecture guard (brief H2, hard rule
// 1): go-fabric is used through its facade only — node, device, contract,
// eligibility and the <cluster>def packages under cluster/spec/; the
// camera host alone also cluster/camera and bridge — and the daemon's own
// packages keep their layers. Tests may import fabrictest,
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
// in-process controller, its path helpers, the wall clock and the
// scalar values a controller's attribute write carries (cluster/spec
// value.go Uint8, which fabrictest.Write takes as a spec.Encodable).
var testOnly = map[string]bool{"fabrictest": true, "controller/client": true, "clock": true, "cluster/spec": true}

// cameraOnly are the go-fabric packages internal/camera alone may import
// besides the facade: cluster/camera, whose *camera.Provider carries the
// camera's signals to the controller (go-fabric docs/guides/camera.md:
// 84-87 at v0.15.5) and whose ErrUnknownSession the relay retries on, and
// bridge for bridge.ErrPayloadTooLarge, the one error a signal over a
// UDP session returns (:121-128). No other package reaches them.
var cameraOnly = map[string]bool{"cluster/camera": true, "bridge": true}

// cameraPkg is the package cameraOnly is for.
const cameraPkg = "internal/camera"

// layers are the module's own import rules: a package may import only the
// module packages its entry lists.
var layers = map[string][]string{
	"internal/version":   {},
	"internal/expose":    {},
	"internal/selection": {},
	"internal/ha":        {},
	"internal/ha/hatest": {"internal/ha"},
	"internal/adapt":     {"internal/ha"},
	"internal/camera":    {"internal/ha"},
	"internal/config":    {"internal/expose"},
	"internal/bridge":    {"internal/ha", "internal/adapt", "internal/camera", "internal/expose", "internal/config", "internal/selection", "internal/version"},
	"internal/web":       {},
	"internal/arch":      {},
	"cmd/ha2fabric":      {"internal/bridge", "internal/config", "internal/ha", "internal/selection", "internal/version", "internal/web"},
}

// testLayers widen layers for a package's tests.
var testLayers = map[string][]string{
	"internal/ha":     {"internal/ha/hatest"},
	"internal/bridge": {"internal/ha/hatest"},
	"internal/camera": {"internal/ha/hatest"},
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

// facadeImport reports whether a file of pkg may import go-fabric's dep.
func facadeImport(pkg, dep string, isTest bool) bool {
	return facade[dep] || isDef(dep) || isTest && testOnly[dep] || pkg == cameraPkg && cameraOnly[dep]
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
				if !facadeImport(pkg, strings.TrimPrefix(p, goFabric), isTest) {
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
		{"cluster/camera", false},
		{"cluster/camera", true},
	} {
		if facadeImport("internal/bridge", c.dep, c.test) {
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
		{"cluster/spec", true},
	} {
		if !facadeImport("internal/bridge", c.dep, c.test) {
			t.Errorf("%s may be imported (test %v)", c.dep, c.test)
		}
	}
	if !facadeImport(cameraPkg, "cluster/camera", false) || !facadeImport(cameraPkg, "bridge", false) || facadeImport(cameraPkg, "im", false) {
		t.Error("the camera package's rule")
	}
	if allowedThirdParty("gopkg.in/yaml.v3") || !allowedThirdParty("github.com/SukramJ/go-hamqtt/model") || allowedThirdParty("github.com/SukramJ/go-hamqtt/discovery") {
		t.Error("third-party list")
	}
}
