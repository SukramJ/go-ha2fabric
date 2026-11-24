// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"thingfabric.dev/go-fabric/node"

	"thingfabric.dev/go-ha2fabric/internal/config"
	"thingfabric.dev/go-ha2fabric/internal/ha/hatest"
)

// TestRunWebListenFails: a web listener that cannot bind ends the daemon
// with exit 1 and the ha2fabric.web line, after its node stopped
// (main.go run, webErr).
func TestRunWebListenFails(t *testing.T) {
	taken, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = taken.Close() }()
	srv := hatest.New(t)
	env := config.MapEnv(map[string]string{
		"HA_URL": srv.URL(), "HA_TOKEN": hatest.Token, "STORE_PATH": filepath.Join(t.TempDir(), "ha2fabric.db"),
		"LISTEN": freePort(t, "udp"), "WEB_LISTEN": taken.Addr().String(),
	})
	var logs safeBuffer
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if code := run(ctx, nil, env, io.Discard, &logs); code != 1 {
		t.Fatalf("exit %d\n%s", code, logs.String())
	}
	if out := logs.String(); !strings.Contains(out, "msg=ha2fabric.web ") || !strings.Contains(out, "ha2fabric.stopping") {
		t.Fatalf("no ha2fabric.web line\n%s", out)
	}
}

// TestCreateRollback: a registry node that does not start, or whose
// record cannot be written, is rolled back — the create answers an error,
// no record and no file of the node stay, and the daemon runs on
// (nodes.go fleet.Create).
func TestCreateRollback(t *testing.T) {
	env, web := eraseEnv(t)
	d := startDaemon(t, env, web)
	dir := filepath.Dir(func() string { v, _ := env("STORE_PATH"); return v }())
	leftovers := func() []string {
		m, _ := filepath.Glob(filepath.Join(dir, "*alexa*"))
		return m
	}

	t.Cleanup(func() { newNode = node.New })
	newNode = func(context.Context, node.Config) (*node.Node, error) {
		return nil, errors.New("the sentinel's start fails")
	}
	d.want(http.MethodPost, "/api/v1/nodes", `{"label":"Alexa"}`, http.StatusInternalServerError)
	newNode = node.New
	if l := d.nodes(); len(l.Nodes) != 1 || len(leftovers()) != 0 {
		t.Fatalf("after a failed start: nodes %+v, files %v", l, leftovers())
	}

	// nodes.json as a directory: the node starts, its record cannot be
	// written.
	if err := os.Mkdir(filepath.Join(dir, "nodes.json"), 0o700); err != nil {
		t.Fatal(err)
	}
	d.want(http.MethodPost, "/api/v1/nodes", `{"label":"Alexa"}`, http.StatusInternalServerError)
	if l := d.nodes(); len(l.Nodes) != 1 || len(leftovers()) != 0 {
		t.Fatalf("after a failed write: nodes %+v, files %v", l, leftovers())
	}
	if err := os.Remove(filepath.Join(dir, "nodes.json")); err != nil {
		t.Fatal(err)
	}
	d.want(http.MethodPost, "/api/v1/nodes", `{"label":"Alexa"}`, http.StatusCreated)
	d.stop()
}
