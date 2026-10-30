// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package main

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SukramJ/go-ha2fabric/internal/config"
	"github.com/SukramJ/go-ha2fabric/internal/ha/hatest"
)

func TestVersionHelpAndBadConfig(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run(context.Background(), []string{"--version"}, config.MapEnv(nil), &out, &errb); code != 0 || !strings.HasPrefix(out.String(), "go-ha2fabric ") {
		t.Fatalf("--version %d %q", code, out.String())
	}
	if code := run(context.Background(), []string{"-h"}, config.MapEnv(nil), &out, io.Discard); code != 0 {
		t.Fatalf("-h %d", code)
	}
	errb.Reset()
	if code := run(context.Background(), nil, config.MapEnv(nil), &out, &errb); code != 2 || !strings.Contains(errb.String(), "HA_URL is required") {
		t.Fatalf("no config %d %q", code, errb.String())
	}
}

func TestDumpPlan(t *testing.T) {
	srv := hatest.New(t)
	var out, errb bytes.Buffer
	env := config.MapEnv(map[string]string{"HA_URL": srv.URL(), "HA_TOKEN": hatest.Token, "INCLUDE": "sensor.outdoor"})
	if code := run(context.Background(), []string{"--dump-plan", "--store-path", t.TempDir() + "/x.db"}, env, &out, &errb); code != 0 {
		t.Fatalf("dump-plan %d: %s", code, errb.String())
	}
	for _, want := range []string{"ENTITY", "light.kitchen", "light/extended_color", "mappable", "light.cellar", "disabled in Home Assistant", "included by sensor.outdoor"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("no %q in\n%s", want, out.String())
		}
	}
	if strings.Contains(errb.String(), hatest.Token) {
		t.Fatal("the token reached the log")
	}
	// A wrong token: the dump fails instead of printing nothing.
	env = config.MapEnv(map[string]string{"HA_URL": srv.URL(), "HA_TOKEN": "wrong"})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if code := run(ctx, []string{"--dump-plan", "--store-path", t.TempDir() + "/x.db"}, env, io.Discard, io.Discard); code != 1 {
		t.Fatalf("dump-plan with a wrong token %d", code)
	}
}

func freePort(t *testing.T, network string) string {
	t.Helper()
	if network == "udp" {
		c, err := net.ListenPacket("udp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = c.Close() }()
		return c.LocalAddr().String()
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	return l.Addr().String()
}

// TestRun starts the daemon for real — its own node on a loopback port,
// the fake Home Assistant — reads its health and stops it.
func TestRun(t *testing.T) {
	if runtime.GOOS != "linux" {
		// The daemon's own node advertises over the host's mDNS; the other
		// runners of the OS triplet get the in-process tests only.
		t.Skip("runs the node on the host's network: linux only")
	}
	srv := hatest.New(t)
	webAddr := freePort(t, "tcp")
	env := config.MapEnv(map[string]string{
		"HA_URL": srv.URL(), "HA_TOKEN": hatest.Token, "STORE_PATH": t.TempDir() + "/ha2fabric.db",
		"LISTEN": freePort(t, "udp"), "WEB_LISTEN": webAddr, "LOG_LEVEL": "debug",
	})
	ctx, cancel := context.WithCancel(context.Background())
	var logs safeBuffer
	done := make(chan int, 1)
	go func() { done <- run(ctx, nil, env, io.Discard, &logs) }()
	deadline := time.Now().Add(20 * time.Second)
	for {
		// The selection lists the candidates once Home Assistant is read.
		res, err := http.Get("http://" + webAddr + "/api/v1/node/exposure") //nolint:noctx // the test's own daemon
		if err == nil {
			b, _ := io.ReadAll(res.Body)
			_ = res.Body.Close()
			if res.StatusCode == http.StatusOK && strings.Contains(string(b), "light.kitchen") {
				break
			}
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("the daemon never became healthy: %v\n%s", err, logs.String())
		}
		time.Sleep(50 * time.Millisecond)
	}
	cancel()
	if code := <-done; code != 0 {
		t.Fatalf("exit %d\n%s", code, logs.String())
	}
	for _, want := range []string{"ha2fabric.fabrics", "count=0", "ha2fabric.pairing", "manual_code=", "bridge.synced"} {
		if !strings.Contains(logs.String(), want) {
			t.Errorf("no %q in the log", want)
		}
	}
	if strings.Contains(logs.String(), hatest.Token) {
		t.Fatal("the token reached the log")
	}
}

// safeBuffer is a log sink the daemon's goroutines write concurrently.
type safeBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *safeBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *safeBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}
