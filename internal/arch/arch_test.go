// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

// Package arch holds the module's architecture guard (brief H2, hard rule
// 1): go-fabric is used through its facade only — node, device, contract,
// eligibility, logring and the <cluster>def packages under cluster/spec/;
// the camera host alone also cluster/camera, and the pure core
// (internal/plan) not even node — and the daemon's own packages keep their
// layers (brief A1): cmd/ha2fabric composes, internal/plan decides,
// internal/bridge reconciles without the web UI or the configuration,
// internal/exposure is the web UI's provider; go-fabric-ui is imported
// only behind internal/exposure, internal/web, internal/nodes and the
// composition root. Tests may import fabrictest,
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
	"slices"
	"strconv"
	"strings"
	"testing"
)

const (
	module   = "thingfabric.dev/go-ha2fabric"
	goFabric = "thingfabric.dev/go-fabric/"
)

// facade are the go-fabric packages the daemon may import.
var facade = map[string]bool{
	"node": true, "device": true, "contract": true, "eligibility": true,
	// The daemon's log ring (issue #34, go-fabric #393; as in
	// go-fabric-server's internal/arch): the slog.Handler in front of the
	// daemon's own, whose records, level and stream go-fabric-ui's
	// Operations port reads (go-fabric docs/guides/observability.md at v0.27.0).
	"logring": true,
}

// testOnly are the go-fabric packages a test may import besides: the
// in-process controller, its path helpers, the wall clock and the
// scalar values a controller's attribute write carries
// (cluster/spec/value.go:62 at v0.27.0, Uint8, which fabrictest.Write takes as a spec.Encodable).
var testOnly = map[string]bool{"fabrictest": true, "controller/client": true, "clock": true, "cluster/spec": true}

// cameraOnly are the go-fabric packages internal/camera alone may import
// besides the facade: cluster/camera, whose *camera.Provider carries the
// camera's signals to the controller (go-fabric docs/guides/camera.md:
// 93-114 at v0.27.0), whose ErrUnknownSession the relay retries on and
// whose ErrPayloadTooLarge is the one error a signal over a UDP session
// returns (:137-144). No other package reaches it.
var cameraOnly = map[string]bool{"cluster/camera": true}

// cameraPkg is the package cameraOnly is for.
const cameraPkg = "internal/camera"

// nodeFree are the packages that may not import go-fabric's node: the
// pure core decides without one (internal/plan; brief A1, section 4).
var nodeFree = map[string]bool{"internal/plan": true}

// adaptGoFabric is all of go-fabric that internal/adapt's code may import
// (HR-P3-api-05): the two def packages whose values its commands carry
// today. A2 empties it (CLAUDE.md, "Not built yet (A2)"); until then new
// code does not deepen what A2 removes.
var adaptGoFabric = map[string]bool{"cluster/spec/keypadinput": true, "cluster/spec/levelcontrol": true}

// adaptPkg is the package adaptGoFabric is for.
const adaptPkg = "internal/adapt"

// layers are the module's own import rules: a package may import only the
// module packages its entry lists.
// Every package has an entry (HR-P3-api-06), the root package's doc and
// the tools under script/ too.
var layers = map[string][]string{
	".":                  {},
	"script/coverfloor":  {},
	"script/cite":        {},
	"internal/version":   {},
	"internal/text":      {},
	"internal/expose":    {},
	"internal/selection": {"internal/text"},
	"internal/ha":        {},
	"internal/ha/hatest": {"internal/ha"},
	"internal/adapt":     {"internal/ha", "internal/text"},
	"internal/camera":    {"internal/ha"},
	"internal/config":    {"internal/expose", "internal/text", "internal/version"},
	"internal/plan":      {"internal/ha", "internal/adapt", "internal/expose"},
	"internal/bridge":    {"internal/ha", "internal/adapt", "internal/camera", "internal/plan"},
	"internal/exposure":  {"internal/bridge", "internal/plan", "internal/ha", "internal/expose", "internal/selection"},
	"internal/nodes":     {"internal/text"},
	"internal/web":       {},
	"internal/arch":      {},
	"cmd/ha2fabric":      {"internal/bridge", "internal/camera", "internal/config", "internal/expose", "internal/exposure", "internal/ha", "internal/nodes", "internal/plan", "internal/selection", "internal/text", "internal/version", "internal/web"},
}

// testLayers widen layers for a package's tests. internal/bridge's end to
// end tests compose the stack as cmd/ha2fabric does: the rule, the
// overrides store and the web UI's provider on the bridge.
var testLayers = map[string][]string{
	"internal/ha":       {"internal/ha/hatest"},
	"internal/plan":     {"internal/ha/hatest", "internal/camera"},
	"internal/bridge":   {"internal/ha/hatest", "internal/config", "internal/expose", "internal/selection", "internal/exposure"},
	"internal/exposure": {"internal/ha/hatest"},
	"internal/camera":   {"internal/ha/hatest"},
	"cmd/ha2fabric":     {"internal/ha/hatest"},
}

// Other modules the daemon may import: the mapping, its vocabulary, the
// catalog, the WebSocket library. Anything else outside the standard
// library is refused.
var thirdParty = []string{
	"sukramj.dev/go-hamapping",
	"sukramj.dev/go-hamqtt/model",
	"sukramj.dev/go-ha-catalog",
	"github.com/coder/websocket",
	"thingfabric.dev/go-fabric-ui/fabricui",
}

// restricted are third-party imports only some packages may make:
// go-fabric-ui is the web UI's, behind internal/exposure and internal/web,
// wired by the composition root; internal/nodes, the store of the web
// UI's node registry, keeps its node keys off the UI's default one
// (nodes.go reservedKeys, fabricui.DefaultNodeKey). A package's tests may
// compose the stack.
var restricted = map[string][]string{
	"thingfabric.dev/go-fabric-ui/fabricui": {"internal/web", "internal/exposure", "internal/nodes", "cmd/ha2fabric"},
}

func isDef(rel string) bool {
	return strings.HasPrefix(rel, "cluster/spec/") && !strings.Contains(strings.TrimPrefix(rel, "cluster/spec/"), "/")
}

// facadeImport reports whether a file of pkg may import go-fabric's dep.
func facadeImport(pkg, dep string, isTest bool) bool {
	if dep == "node" && nodeFree[pkg] {
		return false
	}
	if pkg == adaptPkg && !isTest {
		return adaptGoFabric[dep]
	}
	return facade[dep] || isDef(dep) || isTest && testOnly[dep] || pkg == cameraPkg && cameraOnly[dep]
}

func TestImports(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	files := 0
	unruled := map[string]bool{}
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
		if _, ruled := layers[pkg]; !ruled && !unruled[pkg] {
			unruled[pkg] = true
			t.Errorf("%s: package %s has no layer entry", where, pkg)
		}
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
				if dep == "" {
					dep = "."
				}
				if !layerAllows(pkg, dep, isTest) {
					t.Errorf("%s imports %s: outside the layers of %s", where, p, pkg)
				}
			case strings.Contains(strings.Split(p, "/")[0], "."):
				if !allowedThirdParty(pkg, p, isTest) {
					t.Errorf("%s imports %s: not a dependency of %s", where, p, pkg)
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

// layerAllows reports whether a file of pkg may import the module's dep.
func layerAllows(pkg, dep string, isTest bool) bool {
	if dep == pkg {
		return true
	}
	return slices.Contains(layers[pkg], dep) || slices.Contains(testLayersFor(pkg, isTest), dep)
}

// allowedThirdParty reports whether a file of pkg may import p of
// another module: p is a dependency, and pkg is one restricted names, or
// the file is a test.
func allowedThirdParty(pkg, p string, isTest bool) bool {
	for _, a := range thirdParty {
		if p == a || strings.HasPrefix(p, a+"/") {
			only, ok := restricted[a]
			return !ok || isTest || slices.Contains(only, pkg)
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
	if !facadeImport(cameraPkg, "cluster/camera", false) || facadeImport(cameraPkg, "bridge", false) || facadeImport(cameraPkg, "im", false) {
		t.Error("the camera package's rule")
	}
	if facadeImport("internal/plan", "node", false) || !facadeImport("internal/plan", "device", false) || !facadeImport("internal/plan", "eligibility", false) {
		t.Error("the pure core's rule: no node")
	}
	// internal/adapt's go-fabric reach is frozen (HR-P3-api-05).
	for _, dep := range []string{"cluster/spec/onoff", "node", "device", "eligibility"} {
		if facadeImport(adaptPkg, dep, false) {
			t.Errorf("internal/adapt may not import %s", dep)
		}
	}
	if !facadeImport(adaptPkg, "cluster/spec/levelcontrol", false) || !facadeImport(adaptPkg, "eligibility", true) {
		t.Error("internal/adapt's frozen set, and its tests' facade")
	}
	if _, ok := layers["internal/foo"]; ok {
		t.Error("an unknown package has a layer entry")
	}
	if allowedThirdParty("internal/bridge", "gopkg.in/yaml.v3", false) || !allowedThirdParty("internal/bridge", "sukramj.dev/go-hamqtt/model", false) || allowedThirdParty("internal/bridge", "sukramj.dev/go-hamqtt/discovery", false) {
		t.Error("third-party list")
	}
	// go-fabric-ui is the web UI's: the provider, the handler and the
	// composition root, and tests that compose the stack.
	const ui = "thingfabric.dev/go-fabric-ui/fabricui"
	for _, pkg := range []string{"internal/bridge", "internal/plan", "internal/adapt"} {
		if allowedThirdParty(pkg, ui, false) {
			t.Errorf("%s may not import go-fabric-ui", pkg)
		}
	}
	for _, pkg := range []string{"internal/web", "internal/exposure", "cmd/ha2fabric"} {
		if !allowedThirdParty(pkg, ui, false) {
			t.Errorf("%s may import go-fabric-ui", pkg)
		}
	}
	if !allowedThirdParty("internal/bridge", ui, true) {
		t.Error("internal/bridge's tests may import go-fabric-ui")
	}
	// The layers of the cut (brief A1, section 5).
	for _, c := range []struct{ pkg, dep string }{
		{"internal/bridge", "internal/config"},
		{"internal/bridge", "internal/selection"},
		{"internal/bridge", "internal/exposure"},
		{"internal/plan", "internal/bridge"},
		{"internal/plan", "internal/selection"},
		{"internal/plan", "internal/config"},
		{"internal/plan", "internal/camera"},
		{"internal/exposure", "internal/config"},
		{"internal/config", "internal/bridge"},
	} {
		if layerAllows(c.pkg, c.dep, false) {
			t.Errorf("%s may not import %s", c.pkg, c.dep)
		}
	}
	if !layerAllows("internal/bridge", "internal/exposure", true) || !layerAllows("internal/plan", "internal/camera", true) {
		t.Error("the tests' layers")
	}
}
