// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/SukramJ/go-fabric/node"

	"github.com/SukramJ/go-ha2fabric/internal/config"
	"github.com/SukramJ/go-ha2fabric/internal/ha/hatest"
	"github.com/SukramJ/go-ha2fabric/internal/nodes"
)

// daemon is one run of the daemon in a test, with its web UI's address.
type daemon struct {
	t      *testing.T
	web    string
	token  string
	logs   *safeBuffer
	cancel context.CancelFunc
	done   chan int
}

func startDaemon(t *testing.T, env config.Env, web string) *daemon {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	d := &daemon{t: t, web: web, logs: &safeBuffer{}, cancel: cancel, done: make(chan int, 1)}
	go func() { d.done <- run(ctx, nil, env, io.Discard, d.logs) }()
	t.Cleanup(func() { d.stop() })
	d.eventually("the web UI answers", func() bool {
		code, body := d.call(http.MethodGet, "/api/v1/node/status", "")
		if code != http.StatusOK {
			return false
		}
		var st struct {
			CSRFToken    string `json:"csrf_token"`
			NodeRegistry bool   `json:"node_registry"`
		}
		if json.Unmarshal(body, &st) != nil || !st.NodeRegistry {
			t.Fatalf("status without node_registry: %s", body)
		}
		d.token = st.CSRFToken
		return true
	})
	return d
}

// stop ends the run and wants exit 0; a second call does nothing.
func (d *daemon) stop() {
	if d.cancel == nil {
		return
	}
	d.cancel()
	d.cancel = nil
	if code := <-d.done; code != 0 {
		d.t.Fatalf("exit %d\n%s", code, d.logs.String())
	}
}

func (d *daemon) call(method, path, body string) (code int, resp []byte) {
	d.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, "http://"+d.web+path, strings.NewReader(body))
	if err != nil {
		d.t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if d.token != "" {
		req.Header.Set("X-Fabric-UI-Token", d.token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil
	}
	defer func() { _ = res.Body.Close() }()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, b
}

func (d *daemon) want(method, path, body string, code int) []byte {
	d.t.Helper()
	got, b := d.call(method, path, body)
	if got != code {
		d.t.Fatalf("%s %s: %d, want %d: %s\n%s", method, path, got, code, b, d.logs.String())
	}
	return b
}

func (d *daemon) eventually(what string, cond func() bool) {
	d.t.Helper()
	// 90 s as TestRun's: the slowest runner's node start (main_test.go).
	deadline := time.Now().Add(90 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			d.t.Fatalf("never: %s\n%s", what, d.logs.String())
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// nodeList is GET /api/v1/nodes.
type nodeList struct {
	Nodes []struct {
		Key      string `json:"key"`
		Label    string `json:"label"`
		Enabled  bool   `json:"enabled"`
		Managed  bool   `json:"managed"`
		Exposure bool   `json:"exposure"`
	} `json:"nodes"`
}

func (d *daemon) nodes() nodeList {
	d.t.Helper()
	var l nodeList
	if err := json.Unmarshal(d.want(http.MethodGet, "/api/v1/nodes", "", http.StatusOK), &l); err != nil {
		d.t.Fatal(err)
	}
	return l
}

// TestNodeRegistry runs the daemon for real and drives the registry
// through the web UI: create a node (it starts, reads Home Assistant
// through the shared connection and takes a selection of its own), list
// it with its configuration block, rename it, restart the daemon with it
// in the registry, delete it — its files go — and meet the limit.
func TestNodeRegistry(t *testing.T) {
	srv := hatest.New(t)
	dir := t.TempDir()
	web := freePort(t, "tcp")
	listen := freePort(t, "udp")
	env := config.MapEnv(map[string]string{
		"HA_URL": srv.URL(), "HA_TOKEN": hatest.Token, "STORE_PATH": filepath.Join(dir, "ha2fabric.db"),
		"LISTEN": listen, "WEB_LISTEN": web, "WEB_ADMIN": "true", "NODE_LIMIT": "1", "LOG_LEVEL": "debug",
	})
	d := startDaemon(t, env, web)

	// The configured node's block names its source and never the passcode.
	status := d.want(http.MethodGet, "/api/v1/node/status", "", http.StatusOK)
	if !bytes.Contains(status, []byte(`"source":"configuration"`)) || !bytes.Contains(status, []byte(`"source_note":"the flags and environment`)) || bytes.Contains(status, []byte("20202021")) {
		t.Fatalf("configured node's status %s", status)
	}

	created := d.want(http.MethodPost, "/api/v1/nodes", `{"label":"Alexa"}`, http.StatusCreated)
	if !bytes.Contains(created, []byte(`"key":"alexa"`)) {
		t.Fatalf("created %s", created)
	}
	// NODE_LIMIT=1: the second is refused.
	if b := d.want(http.MethodPost, "/api/v1/nodes", `{"label":"Google"}`, http.StatusConflict); !bytes.Contains(b, []byte("node_limit")) {
		t.Fatalf("limit %s", b)
	}
	l := d.nodes()
	if len(l.Nodes) != 2 || l.Nodes[1].Key != "alexa" || !l.Nodes[1].Managed || !l.Nodes[1].Enabled || !l.Nodes[1].Exposure {
		t.Fatalf("nodes %+v", l)
	}

	reg := readRegistry(t, dir)
	if len(reg) != 1 || reg[0].Key != "alexa" {
		t.Fatalf("nodes.json %+v", reg)
	}
	_, lp, _ := net.SplitHostPort(listen)
	base, _ := strconv.Atoi(lp)
	_, ap, _ := net.SplitHostPort(reg[0].Listen)
	if p, _ := strconv.Atoi(ap); p <= base {
		t.Fatalf("the node's port %s is not above the configured %d", reg[0].Listen, base)
	}
	var st struct {
		Config struct {
			Source        string `json:"source"`
			Label         string `json:"label"`
			Listen        string `json:"listen"`
			Discriminator int    `json:"discriminator"`
			Store         string `json:"store"`
		} `json:"config"`
	}
	raw := d.want(http.MethodGet, "/api/v1/nodes/alexa/status", "", http.StatusOK)
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatal(err)
	}
	c := st.Config
	if c.Source != "registry" || c.Label != "Alexa" || c.Listen != reg[0].Listen || c.Discriminator != int(reg[0].Discriminator) || c.Store != "ha2fabric-alexa.db" {
		t.Fatalf("registry node's config %s", raw)
	}
	if bytes.Contains(raw, []byte(strconv.Itoa(int(reg[0].Passcode)))) {
		t.Fatal("the status carries the passcode")
	}

	// The node joined the running connection: its selection lists Home
	// Assistant's candidates, and none is exposed until picked.
	d.eventually("the node's selection lists light.kitchen", func() bool {
		_, b := d.call(http.MethodGet, "/api/v1/nodes/alexa/exposure", "")
		return bytes.Contains(b, []byte("light.kitchen"))
	})
	d.want(http.MethodPut, "/api/v1/nodes/alexa/exposure", `{"items":[{"key":"light.kitchen","enabled":true}]}`, http.StatusOK)
	if _, err := os.Stat(filepath.Join(dir, "selection-alexa.json")); err != nil {
		t.Fatalf("the node's selection: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "selection.json")); err == nil {
		t.Fatal("the node's choice reached the configured node's selection")
	}

	d.want(http.MethodPut, "/api/v1/nodes/alexa/label", `{"label":"Echo"}`, http.StatusNoContent)
	if l := d.nodes(); l.Nodes[1].Label != "Echo" {
		t.Fatalf("renamed %+v", l)
	}
	d.want(http.MethodDelete, "/api/v1/nodes/default", "", http.StatusConflict)
	d.stop()

	// A restart starts the registry's node with its label and selection.
	d = startDaemon(t, env, web)
	if l := d.nodes(); len(l.Nodes) != 2 || l.Nodes[1].Key != "alexa" || l.Nodes[1].Label != "Echo" || !l.Nodes[1].Enabled {
		t.Fatalf("after the restart %+v\n%s", l, d.logs.String())
	}
	if !strings.Contains(d.logs.String(), "node=alexa") {
		t.Fatal("no log line of the registry node")
	}

	d.want(http.MethodDelete, "/api/v1/nodes/alexa", "", http.StatusNoContent)
	if l := d.nodes(); len(l.Nodes) != 1 {
		t.Fatalf("after the delete %+v", l)
	}
	for _, f := range []string{"ha2fabric-alexa.db", "selection-alexa.json"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			t.Errorf("%s survived the delete", f)
		}
	}
	if reg := readRegistry(t, dir); len(reg) != 0 {
		t.Fatalf("nodes.json after the delete %+v", reg)
	}
	d.want(http.MethodDelete, "/api/v1/nodes/alexa", "", http.StatusNotFound)
	// The limit counts the registry: room again.
	d.want(http.MethodPost, "/api/v1/nodes", `{"label":"Google"}`, http.StatusCreated)
	d.stop()
	if !strings.Contains(d.logs.String(), "ha2fabric.node_deleted") {
		t.Fatal("no node_deleted line")
	}
}

// nodeStatus is the part of GET /api/v1/node/status the erase tests read.
type nodeStatus struct {
	Enabled     bool `json:"enabled"`
	Erase       bool `json:"erase"`
	FabricCount int  `json:"fabric_count"`
	Warnings    []struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"warnings"`
}

func (d *daemon) status(path string) nodeStatus {
	d.t.Helper()
	var st nodeStatus
	if err := json.Unmarshal(d.want(http.MethodGet, path, "", http.StatusOK), &st); err != nil {
		d.t.Fatal(err)
	}
	return st
}

// warned reports whether st holds code with prefix at the start of its
// message.
func (st *nodeStatus) warned(code, prefix string) bool {
	for _, w := range st.Warnings {
		if w.Code == code && strings.HasPrefix(w.Message, prefix) {
			return true
		}
	}
	return false
}

// endpointIDs are the bridged endpoints of GET <path>/endpoints.
func (d *daemon) endpointIDs(path string) []int {
	d.t.Helper()
	var l struct {
		Endpoints []struct {
			EndpointID int `json:"endpoint_id"`
		} `json:"endpoints"`
	}
	code, b := d.call(http.MethodGet, path+"/endpoints", "")
	if code != http.StatusOK || json.Unmarshal(b, &l) != nil {
		return nil
	}
	ids := make([]int, 0, len(l.Endpoints))
	for _, e := range l.Endpoints {
		ids = append(ids, e.EndpointID)
	}
	return ids
}

func eraseEnv(t *testing.T) (env config.Env, web string) {
	t.Helper()
	srv := hatest.New(t)
	dir := t.TempDir()
	web = freePort(t, "tcp")
	env = config.MapEnv(map[string]string{
		"HA_URL": srv.URL(), "HA_TOKEN": hatest.Token, "STORE_PATH": filepath.Join(dir, "ha2fabric.db"),
		"LISTEN": freePort(t, "udp"), "WEB_LISTEN": web, "WEB_ADMIN": "true", "NODE_LIMIT": "1", "LOG_LEVEL": "debug",
	})
	return env, web
}

const eraseBody = `{"confirm":"erase-node"}`

// TestNodeErase erases the configured node and a registry node through
// the web UI (go-fabric-ui contract 0.7.0, brief E1): the node comes back
// on its erased store, the status reads the new one through the switch,
// the fixtures' endpoints come back, and Delete works after an erase.
func TestNodeErase(t *testing.T) {
	env, web := eraseEnv(t)
	d := startDaemon(t, env, web)
	var before []int
	d.eventually("the configured node serves the fixtures", func() bool {
		before = d.endpointIDs("/api/v1/node")
		return len(before) > 0
	})
	if !d.status("/api/v1/node/status").Erase {
		t.Fatal("the status does not offer Erase")
	}

	d.want(http.MethodPost, "/api/v1/node/maintenance/erase", "", http.StatusBadRequest)
	d.want(http.MethodPost, "/api/v1/node/maintenance/erase", eraseBody, http.StatusNoContent)
	d.eventually("the erased node answers", func() bool {
		code, b := d.call(http.MethodGet, "/api/v1/node/status", "")
		var st nodeStatus
		return code == http.StatusOK && json.Unmarshal(b, &st) == nil && st.Enabled && st.Erase && st.FabricCount == 0
	})
	var after []int
	d.eventually("the fixtures' endpoints come back", func() bool {
		after = d.endpointIDs("/api/v1/node")
		return len(after) == len(before)
	})
	// The measurement of brief E1 §4.1 on a store file: reported, not a gate.
	t.Logf("configured node's endpoints across the erase: before %v, after %v", before, after)
	logs := d.logs.String()
	if !strings.Contains(logs, "ha2fabric.node_erased") || strings.Count(logs, "msg=ha2fabric.pairing ") < 2 {
		t.Fatalf("no node_erased line or no second pairing line\n%s", logs)
	}
	if strings.Contains(logs, hatest.Token) {
		t.Fatal("the log carries the token")
	}

	d.want(http.MethodPost, "/api/v1/nodes", `{"label":"Alexa"}`, http.StatusCreated)
	d.want(http.MethodPost, "/api/v1/nodes/alexa/maintenance/erase", eraseBody, http.StatusNoContent)
	d.eventually("the erased registry node answers", func() bool {
		code, b := d.call(http.MethodGet, "/api/v1/nodes/alexa/status", "")
		var st nodeStatus
		return code == http.StatusOK && json.Unmarshal(b, &st) == nil && st.Enabled && st.FabricCount == 0
	})
	if !strings.Contains(d.logs.String(), "node=alexa") || strings.Count(d.logs.String(), "ha2fabric.node_erased") < 2 {
		t.Fatalf("no node_erased line of the registry node\n%s", d.logs.String())
	}
	d.want(http.MethodDelete, "/api/v1/nodes/alexa", "", http.StatusNoContent)
	d.stop()
}

// TestNodeEraseFailed is a come-back that fails (brief E1 §3, decided
// 2026-11-08): the erase answers 500, the daemon keeps running, every
// status page shows node_erase_failed, and the next Erase starts the node
// again. A registry node in that state is listed not enabled and can be
// deleted.
func TestNodeEraseFailed(t *testing.T) {
	env, web := eraseEnv(t)
	d := startDaemon(t, env, web)
	sentinel := errors.New("the sentinel's come-back fails")
	t.Cleanup(func() { newNode = node.New })
	fail := func() { newNode = func(context.Context, node.Config) (*node.Node, error) { return nil, sentinel } }

	fail()
	d.want(http.MethodPost, "/api/v1/node/maintenance/erase", eraseBody, http.StatusInternalServerError)
	// The switch keeps the stopped node (go-fabric-ui fabricui/nodeswitch.go
	// at v0.7.1: Set panics on nil), so the status is the stopped node's.
	st := d.status("/api/v1/node/status")
	if !st.warned("node_erase_failed", "the node did not come back after the erase: node: "+sentinel.Error()) {
		t.Fatalf("no node_erase_failed: %+v", st)
	}
	if !st.Enabled {
		t.Fatalf("a switch on the stopped node lists the node not enabled: %+v", st)
	}
	code, b := d.call(http.MethodGet, "/api/v1/node/fabrics", "")
	t.Logf("GET /node/fabrics after a failed come-back: %d %s", code, b)
	if code != http.StatusInternalServerError {
		t.Fatalf("fabrics of the stopped node: %d %s", code, b)
	}
	if logs := d.logs.String(); !strings.Contains(logs, "ha2fabric.node_lost") || !strings.Contains(logs, sentinel.Error()) {
		t.Fatalf("no node_lost line\n%s", logs)
	}
	select {
	case c := <-d.done:
		t.Fatalf("the daemon exited %d", c)
	default:
	}

	newNode = node.New
	d.want(http.MethodPost, "/api/v1/node/maintenance/erase", eraseBody, http.StatusNoContent)
	d.eventually("the retried node answers", func() bool {
		code, b := d.call(http.MethodGet, "/api/v1/node/status", "")
		var st nodeStatus
		return code == http.StatusOK && json.Unmarshal(b, &st) == nil && st.Enabled && st.FabricCount == 0 && !st.warned("node_erase_failed", "")
	})
	d.eventually("the retried node serves the fixtures", func() bool { return len(d.endpointIDs("/api/v1/node")) > 0 })

	d.want(http.MethodPost, "/api/v1/nodes", `{"label":"Alexa"}`, http.StatusCreated)
	fail()
	d.want(http.MethodPost, "/api/v1/nodes/alexa/maintenance/erase", eraseBody, http.StatusInternalServerError)
	newNode = node.New
	if l := d.nodes(); len(l.Nodes) != 2 || l.Nodes[1].Key != "alexa" || l.Nodes[1].Enabled {
		t.Fatalf("nodes after the registry node's failed erase %+v", l)
	}
	for _, path := range []string{"/api/v1/node/status", "/api/v1/nodes/alexa/status"} {
		if st := d.status(path); !st.warned("node_erase_failed", "node alexa: the node did not come back after the erase") {
			t.Fatalf("%s without the registry node's node_erase_failed: %+v", path, st)
		}
	}
	d.want(http.MethodPost, "/api/v1/nodes/alexa/maintenance/erase", eraseBody, http.StatusServiceUnavailable)
	d.want(http.MethodDelete, "/api/v1/nodes/alexa", "", http.StatusNoContent)
	if st := d.status("/api/v1/node/status"); st.warned("node_erase_failed", "") {
		t.Fatalf("the deleted node's warning stays: %+v", st)
	}
	d.stop()
}

func readRegistry(t *testing.T, dir string) []nodes.Node {
	t.Helper()
	r, err := nodes.Open(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	return r.Nodes()
}
