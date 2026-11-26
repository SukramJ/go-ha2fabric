// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"thingfabric.dev/go-fabric/node"

	"thingfabric.dev/go-ha2fabric/internal/config"
	"thingfabric.dev/go-ha2fabric/internal/ha"
	"thingfabric.dev/go-ha2fabric/internal/ha/hatest"
)

// restoreSeams puts the node seams back after a test.
func restoreSeams(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		newNode, factoryReset, stopNode = node.New, (*node.Node).FactoryReset, (*node.Node).Stop
	})
}

// TestEraseResetFailedKeepsNode (HR-P1-err-61, HR-P1-test-70): a reset
// that returns without stopping the node — go-fabric's FactoryReset can
// (node/factory_reset.go:51-55 at v0.26.0) — does not orphan it: the
// daemon stops it before the come-back, so the node comes back on its
// port. The erase answers the reset's error.
func TestEraseResetFailedKeepsNode(t *testing.T) {
	restoreSeams(t)
	env, web := eraseEnv(t)
	d := startDaemon(t, env, web)
	sentinel := errors.New("the sentinel's reset did not stop the node")
	factoryReset = func(*node.Node, context.Context) error { return sentinel }
	d.want(http.MethodPost, "/api/v1/node/maintenance/erase", eraseBody, http.StatusInternalServerError)
	factoryReset = (*node.Node).FactoryReset
	if st := d.status("/api/v1/node/status"); st.warned("node_erase_failed", "") || !st.Enabled {
		t.Fatalf("the node did not come back after a reset that did not stop it: %+v\n%s", st, d.logs.String())
	}
	d.want(http.MethodPost, "/api/v1/node/maintenance/erase", eraseBody, http.StatusNoContent)
	d.stop()
}

// TestEraseResetAndStopFailedKeepsNode (HR-P1-err-61, HR-P1-test-70): a
// node that neither the reset nor the stop stopped is kept, not dropped:
// the next erase resets that node again (go-fabric: "a later
// FactoryReset finishes the reset") and the node comes back.
func TestEraseResetAndStopFailedKeepsNode(t *testing.T) {
	restoreSeams(t)
	env, web := eraseEnv(t)
	d := startDaemon(t, env, web)
	sentinel := errors.New("the sentinel's stop failed")
	factoryReset = func(*node.Node, context.Context) error { return sentinel }
	stopNode = func(*node.Node, context.Context) error { return sentinel }
	d.want(http.MethodPost, "/api/v1/node/maintenance/erase", eraseBody, http.StatusInternalServerError)
	if st := d.status("/api/v1/node/status"); !st.warned("node_erase_failed", "") {
		t.Fatalf("no node_erase_failed while the old node holds its port: %+v", st)
	}
	factoryReset, stopNode = (*node.Node).FactoryReset, (*node.Node).Stop
	d.want(http.MethodPost, "/api/v1/node/maintenance/erase", eraseBody, http.StatusNoContent)
	if st := d.status("/api/v1/node/status"); st.warned("node_erase_failed", "") || !st.Enabled || st.FabricCount != 0 {
		t.Fatalf("the kept node was not reset by the next erase: %+v\n%s", st, d.logs.String())
	}
	d.stop()
}

// TestStatusDuringErase (HR-P1-conc-63): an erase does not hold the
// node's lock across the reset and the come-back, so a status read
// answers while it runs.
func TestStatusDuringErase(t *testing.T) {
	restoreSeams(t)
	env, web := eraseEnv(t)
	d := startDaemon(t, env, web)
	newNode = func(ctx context.Context, c node.Config) (*node.Node, error) {
		time.Sleep(2 * time.Second)
		return node.New(ctx, c)
	}
	erased := make(chan int, 1)
	go func() {
		code, _ := d.call(http.MethodPost, "/api/v1/node/maintenance/erase", eraseBody)
		erased <- code
	}()
	time.Sleep(500 * time.Millisecond)
	began := time.Now()
	code, _ := d.call(http.MethodGet, "/api/v1/node/status", "")
	if took := time.Since(began); code != http.StatusOK || took > time.Second {
		t.Fatalf("a status read during the erase: %d after %v", code, took)
	}
	if code := <-erased; code != http.StatusNoContent {
		t.Fatalf("erase %d", code)
	}
	d.stop()
}

// TestStopWithinBudget (HR-P1-err-64, decision 29): a stop while an erase
// is inside its reset ends within the stop budget — the erase ends with
// the daemon, and the nodes stop under one deadline.
func TestStopWithinBudget(t *testing.T) {
	restoreSeams(t)
	stopBudget = 3 * time.Second
	t.Cleanup(func() { stopBudget = defaultStopBudget })
	env, web := eraseEnv(t)
	d := startDaemon(t, env, web)
	release := make(chan struct{})
	timer := time.AfterFunc(12*time.Second, func() { close(release) })
	t.Cleanup(func() { timer.Stop() })
	factoryReset = func(n *node.Node, ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-release:
		}
		return errors.New("released")
	}
	go func() { _, _ = d.call(http.MethodPost, "/api/v1/node/maintenance/erase", eraseBody) }()
	time.Sleep(500 * time.Millisecond)
	began := time.Now()
	d.cancel()
	d.cancel = nil
	select {
	case code := <-d.done:
		if took := time.Since(began); took > stopBudget+2*time.Second {
			t.Fatalf("the stop took %v, budget %v", took, stopBudget)
		}
		if code != 0 {
			t.Fatalf("exit %d", code)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("no exit")
	}
}

// TestStopLetsEraseFinish (HR-P1-err-94, decision 29): a stop that
// arrives while an erase resets the node does not cancel the reset at
// once — one cut between "fabrics removed" and "store reset" would leave
// the erase half done — but lets it finish inside the stop budget; the
// daemon then exits 0 without bringing the node back.
func TestStopLetsEraseFinish(t *testing.T) {
	restoreSeams(t)
	env, web := eraseEnv(t)
	d := startDaemon(t, env, web)
	entered, got := make(chan struct{}), make(chan error, 1)
	factoryReset = func(n *node.Node, ctx context.Context) error {
		close(entered)
		select {
		case <-ctx.Done():
			got <- ctx.Err()
			return ctx.Err()
		case <-time.After(time.Second):
			got <- nil
		}
		return n.FactoryReset(ctx)
	}
	go func() { _, _ = d.call(http.MethodPost, "/api/v1/node/maintenance/erase", eraseBody) }()
	<-entered
	d.cancel()
	d.cancel = nil
	if err := <-got; err != nil {
		t.Fatalf("the stop cancelled the reset at once: %v", err)
	}
	select {
	case code := <-d.done:
		if code != 0 {
			t.Fatalf("exit %d", code)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("no exit")
	}
	if strings.Contains(d.logs.String(), "ha2fabric.node_lost") {
		t.Fatalf("the stopping daemon tried to bring the node back:\n%s", d.logs.String())
	}
}

// TestSignalDuringStart (HR-P1-err-72): a stop that arrives before the
// daemon has started is an ordinary stop: exit 0.
func TestSignalDuringStart(t *testing.T) {
	srv := hatest.New(t)
	env := config.MapEnv(map[string]string{
		"HA_URL": srv.URL(), "HA_TOKEN": hatest.Token, "STORE_PATH": filepath.Join(t.TempDir(), "ha2fabric.db"),
		"LISTEN": freePort(t, "udp"), "WEB_LISTEN": "",
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var logs strings.Builder
	if code := run(ctx, nil, env, io.Discard, &logs); code != 0 {
		t.Fatalf("exit %d\n%s", code, logs.String())
	}
}

// TestEraseAfterStopRefused (HR-P1-err-101): an erase that takes the
// node's lock after the daemon's stop began but before close is refused,
// so no reset starts outside the stop budget.
func TestEraseAfterStopRefused(t *testing.T) {
	base, cancel := context.WithCancel(context.Background())
	cancel()
	restoreSeams(t)
	reset := false
	factoryReset = func(*node.Node, context.Context) error { reset = true; return nil }
	// A node kept for the next erase: without the refusal the erase would
	// reach it (and its reset) although the daemon already stops.
	s := &served{base: base, stale: &node.Node{}, log: slog.New(slog.DiscardHandler)}
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("the erase went on and panicked: %v", r)
			}
		}()
		err = s.erase(context.Background())
	}()
	if !errors.Is(err, errStopping) || reset {
		t.Fatalf("erase after the stop: err=%v reset=%v, want errStopping and no reset", err, reset)
	}
}

// TestCloseBoundedByBridge (HR-P2-conc-60): a bridge worker that does not
// end — a camera's sessions held by controllers that went away — does not
// hold the node's close past the stop's context.
func TestCloseBoundedByBridge(t *testing.T) {
	restoreSeams(t)
	stopNode = func(*node.Node, context.Context) error { return nil }
	s := &served{base: context.Background(), fan: &ha.Fanout{}, n: &node.Node{}, cancel: func() {}, done: make(chan struct{}), log: slog.New(slog.DiscardHandler)}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	errc := make(chan error, 1)
	go func() { errc <- s.close(ctx) }()
	select {
	case <-errc:
	case <-time.After(5 * time.Second):
		t.Fatal("close waited for the bridge's worker past the stop's context")
	}
}
