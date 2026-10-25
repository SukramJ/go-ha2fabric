// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SukramJ/go-fabric/node"

	"github.com/SukramJ/go-ha2fabric/internal/bridge"
)

type fakeBridge struct{ status bridge.Status }

func (f fakeBridge) Status() bridge.Status { return f.status }
func (f fakeBridge) Plan() []bridge.Verdict {
	return []bridge.Verdict{{EntityID: "light.kitchen", Exposed: true, ExposeReason: "exposed to Assist", Row: "light/extended_color", State: "mappable", Endpoint: 3}}
}

type fakeNode struct {
	healthy bool
	err     error
}

func (fakeNode) PairingCodes() (qr, manual string, err error) {
	return "MT:Y.K9042C00KA0648G00", "34970112332", nil
}

func (f fakeNode) FabricInfos(context.Context) ([]node.FabricInfo, error) {
	return []node.FabricInfo{{FabricIndex: 1, FabricID: 0xABC, VendorID: 0xFFF1, Label: "home"}}, f.err
}

func (f fakeNode) Health() (node.HealthSample, bool) {
	return node.HealthSample{Healthy: f.healthy, Note: "listener closed"}, true
}

func get(t *testing.T, h http.Handler, path, remote string) (code int, body []byte) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
	if remote != "" {
		req.RemoteAddr = remote
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	res := rec.Result()
	defer func() { _ = res.Body.Close() }()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, b
}

func TestHealthAndPlan(t *testing.T) {
	s := New(fakeBridge{bridge.Status{HAConnected: true, LastSync: time.Now(), Endpoints: 4}}, fakeNode{healthy: true}, Options{})
	code, b := get(t, s.Handler(), "/api/health", "")
	var h Health
	if err := json.Unmarshal(b, &h); err != nil || code != http.StatusOK || h.Status != "ok" || h.Pairing.Manual != "34970112332" ||
		len(h.Fabrics) != 1 || h.Fabrics[0].VendorID != "0xFFF1" || h.Fabrics[0].FabricID != "0000000000000ABC" || !h.Node.Healthy || h.Bridge.Endpoints != 4 {
		t.Fatalf("health %d %s", code, b)
	}
	code, b = get(t, s.Handler(), "/api/plan", "")
	var p []bridge.Verdict
	if err := json.Unmarshal(b, &p); err != nil || code != http.StatusOK || len(p) != 1 || p[0].Endpoint != 3 {
		t.Fatalf("plan %s", b)
	}
	// Connected but not read yet.
	s = New(fakeBridge{bridge.Status{HAConnected: true}}, nil, Options{})
	if code, b := get(t, s.Handler(), "/api/health", ""); code != http.StatusServiceUnavailable || !strings.Contains(string(b), "not read yet") {
		t.Fatalf("before the first sync %d %s", code, b)
	}
	// Degraded: Home Assistant away, the node unhealthy, the fabrics unread.
	s = New(fakeBridge{}, fakeNode{err: errors.New("store closed")}, Options{})
	code, b = get(t, s.Handler(), "/api/health", "")
	if code != http.StatusServiceUnavailable || !strings.Contains(string(b), "Home Assistant is not connected") ||
		!strings.Contains(string(b), "listener closed") || !strings.Contains(string(b), "store closed") {
		t.Fatalf("degraded %d %s", code, b)
	}
	// The page and its scripts.
	for path, want := range map[string]string{"/": "go-ha2fabric", "/qr.js": "qrMatrix", "/app.js": "api/health", "/style.css": "--bg"} {
		code, b := get(t, s.Handler(), path, "")
		if code != http.StatusOK || !strings.Contains(string(b), want) {
			t.Errorf("%s: %d", path, code)
		}
	}
}

func TestAllow(t *testing.T) {
	s := New(fakeBridge{}, nil, Options{Allow: []string{"172.30.32.2", "nonsense"}})
	if code, _ := get(t, s.Handler(), "/api/plan", "172.30.32.2:4321"); code != http.StatusOK {
		t.Fatalf("Ingress proxy refused: %d", code)
	}
	if code, _ := get(t, s.Handler(), "/api/plan", "192.168.1.20:4321"); code != http.StatusForbidden {
		t.Fatalf("a LAN client was served: %d", code)
	}
	if code, _ := get(t, s.Handler(), "/api/plan", "garbage"); code != http.StatusForbidden {
		t.Fatalf("an unparsable client was served: %d", code)
	}
}

func TestRun(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	ctx, cancel := context.WithCancel(context.Background())
	s := New(fakeBridge{bridge.Status{HAConnected: true, LastSync: time.Now()}}, nil, Options{Listen: addr})
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		res, err := http.Get("http://" + addr + "/api/health") //nolint:noctx // a test against its own server
		if err == nil {
			_ = res.Body.Close()
			if res.StatusCode != http.StatusOK {
				t.Fatalf("health %d", res.StatusCode)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := New(fakeBridge{}, nil, Options{Listen: "256.0.0.1:1"}).Run(context.Background()); err == nil {
		t.Fatal("a bad address served")
	}
}

// TestQRMatchesReference runs the page's encoder (assets/qr.js) under
// node and compares its matrix, per mask, with the reference matrices of
// python-qrcode in testdata/qr_golden.json. Without node the test skips:
// CI's ubuntu runner has it.
func TestQRMatchesReference(t *testing.T) {
	nodeBin, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	b, err := os.ReadFile(filepath.Join("testdata", "qr_golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Cases map[string]map[string][]string `json:"cases"`
	}
	if err := json.Unmarshal(b, &golden); err != nil {
		t.Fatal(err)
	}
	qr, _ := filepath.Abs(filepath.Join("assets", "qr.js"))
	script := `const {qrMatrix} = require(process.argv[1]); const out = {};
for (let m = 0; m < 8; m++) out[m] = qrMatrix(process.argv[2], m).map(r => r.map(v => v ? 1 : 0).join(""));
out.auto = qrMatrix(process.argv[2]).map(r => r.map(v => v ? 1 : 0).join(""));
console.log(JSON.stringify(out));`
	for text, masks := range golden.Cases {
		out, err := exec.Command(nodeBin, "-e", script, qr, text).Output() //nolint:gosec // the test's own script
		if err != nil {
			t.Fatalf("node: %v", err)
		}
		var got map[string][]string
		if err := json.Unmarshal(out, &got); err != nil {
			t.Fatal(err)
		}
		auto := false
		for mask, want := range masks {
			if strings.Join(got[mask], "\n") != strings.Join(want, "\n") {
				t.Errorf("%q mask %s differs from the reference", text, mask)
			}
			auto = auto || strings.Join(got["auto"], "\n") == strings.Join(want, "\n")
		}
		if !auto {
			t.Errorf("%q: the chosen mask is none of the reference's", text)
		}
	}
}
