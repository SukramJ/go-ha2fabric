// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package hatest_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"thingfabric.dev/go-ha2fabric/internal/ha"
	"thingfabric.dev/go-ha2fabric/internal/ha/hatest"
)

type up struct {
	once sync.Once
	ch   chan struct{}
}

func (u *up) Connected(string)   { u.once.Do(func() { close(u.ch) }) }
func (u *up) Event(ha.Event)     {}
func (u *up) Disconnected(error) {}

// TestAnswersAreCopies (HR-P4-test-61): the fake answers a copy of its
// state, made under its lock, so a test's writes while a client reads
// are no data race (go test -race).
func TestAnswersAreCopies(t *testing.T) {
	srv := hatest.New(t)
	c := ha.New(ha.Options{URL: srv.URL(), Token: hatest.Token})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h := &up{ch: make(chan struct{})}
	go func() { _ = c.Run(ctx, h) }()
	select {
	case <-h.ch:
	case <-time.After(10 * time.Second):
		t.Fatal("no connection")
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range 200 {
			srv.SetExposed("light.kitchen", i%2 == 0)
			srv.RenameEntity("light.kitchen", "light.kitchen")
		}
	}()
	for range 200 {
		if _, err := c.ListExposed(ctx); err != nil {
			t.Fatal(err)
		}
		if _, err := c.GetConfig(ctx); err != nil {
			t.Fatal(err)
		}
	}
	<-done
}
