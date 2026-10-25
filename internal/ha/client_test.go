// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package ha_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SukramJ/go-ha2fabric/internal/ha"
	"github.com/SukramJ/go-ha2fabric/internal/ha/hatest"
)

// recorder is a Handler that keeps what it saw.
type recorder struct {
	mu        sync.Mutex
	connected int
	events    []ha.Event
	errs      []error
	ch        chan string
}

func newRecorder() *recorder { return &recorder{ch: make(chan string, 256)} }

func (r *recorder) Connected(string) {
	r.mu.Lock()
	r.connected++
	r.mu.Unlock()
	r.ch <- "connected"
}

func (r *recorder) Event(ev ha.Event) {
	r.mu.Lock()
	r.events = append(r.events, ev)
	r.mu.Unlock()
	r.ch <- "event:" + ev.EventType
}

func (r *recorder) Disconnected(err error) {
	r.mu.Lock()
	r.errs = append(r.errs, err)
	r.mu.Unlock()
	r.ch <- "disconnected"
}

func (r *recorder) wait(t *testing.T, what string) {
	t.Helper()
	timeout := time.After(10 * time.Second)
	for {
		select {
		case got := <-r.ch:
			if got == what {
				return
			}
		case <-timeout:
			t.Fatalf("no %s", what)
		}
	}
}

func start(t *testing.T, opts ha.Options) (*ha.Client, *recorder, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	c := ha.New(opts)
	r := newRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = c.Run(ctx, r)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	return c, r, cancel
}

func TestClientReadsTheRegistries(t *testing.T) {
	srv := hatest.New(t)
	var logs bytes.Buffer
	c, r, _ := start(t, ha.Options{
		URL: srv.URL(), Token: hatest.Token, Events: []string{ha.EventStateChanged, ha.EventEntityRegistryUpdated},
		Logger: slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
	})
	r.wait(t, "connected")
	if !c.Connected() {
		t.Fatal("not connected")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ents, err := c.ListEntities(ctx)
	if err != nil || len(ents) < 10 {
		t.Fatalf("entities %d %v", len(ents), err)
	}
	var cellar ha.EntityEntry
	for _, e := range ents {
		if e.EntityID == "light.cellar" {
			cellar = e
		}
	}
	if cellar.DisabledBy != "user" || cellar.Domain() != "light" || cellar.Platform != "hue" || cellar.DeviceID != "dev-hue-2" {
		t.Fatalf("cellar %+v", cellar)
	}
	devs, err := c.ListDevices(ctx)
	if err != nil || len(devs) == 0 || devs[0].Manufacturer == "" || len(devs[0].Identifiers[0]) != 2 {
		t.Fatalf("devices %+v %v", devs, err)
	}
	areas, err := c.ListAreas(ctx)
	if err != nil || len(areas) != 2 || areas[0].Name != "Kitchen" {
		t.Fatalf("areas %+v %v", areas, err)
	}
	states, err := c.GetStates(ctx)
	if err != nil || len(states) == 0 || states[0].Attributes["supported_color_modes"] == nil {
		t.Fatalf("states %v", err)
	}
	cfg, err := c.GetConfig(ctx)
	if err != nil || cfg.TemperatureUnit() != "°C" || cfg.Version != "2026.10.3" {
		t.Fatalf("config %+v %v", cfg, err)
	}
	ex, err := c.ListExposed(ctx)
	if err != nil || !ex.Entities["light.kitchen"][ha.AssistantConversation] || ex.Entities["sensor.processor_use"] != nil {
		t.Fatalf("exposed %+v %v", ex, err)
	}
	if err := c.Call(ctx, map[string]any{"type": "no/such/command"}, nil); err == nil || !strings.Contains(err.Error(), "unknown_command") {
		t.Fatalf("unknown command: %v", err)
	}

	// Events of the subscriptions reach the handler.
	srv.SetState("light.kitchen", "off", nil)
	r.wait(t, "event:"+ha.EventStateChanged)
	r.mu.Lock()
	ev := r.events[len(r.events)-1]
	r.mu.Unlock()
	var sc ha.StateChanged
	if err := json.Unmarshal(ev.Data, &sc); err != nil || sc.EntityID != "light.kitchen" || sc.NewState.State != "off" || sc.OldState.State != "on" {
		t.Fatalf("state_changed %+v %v", sc, err)
	}
	srv.UpdateEntity("light.kitchen", map[string]any{"disabled_by": "user"})
	r.wait(t, "event:"+ha.EventEntityRegistryUpdated)

	// call_service with target.entity_id.
	if err := c.CallService(ctx, ha.ServiceCall{Domain: "light", Service: "turn_on", EntityID: "light.kitchen", Data: map[string]any{"brightness": 128}}); err != nil {
		t.Fatal(err)
	}
	call, err := srv.WaitCall(ctx, func(sc ha.ServiceCall) bool { return sc.Service == "turn_on" })
	if err != nil || call.EntityID != "light.kitchen" || call.Data["brightness"] != float64(128) {
		t.Fatalf("call %+v %v", call, err)
	}
	srv.Fail = func(ha.ServiceCall) *ha.Error { return &ha.Error{Code: "not_found", Message: "Service not found."} }
	var herr *ha.Error
	if err := c.CallService(ctx, ha.ServiceCall{Domain: "light", Service: "nope"}); !errors.As(err, &herr) || herr.Code != "not_found" {
		t.Fatalf("failed call: %v", err)
	}
	if strings.Contains(logs.String(), hatest.Token) {
		t.Fatal("the token reached the log")
	}
}

func TestClientReconnects(t *testing.T) {
	srv := hatest.New(t)
	c, r, _ := start(t, ha.Options{URL: srv.URL(), Token: hatest.Token, MinBackoff: 20 * time.Millisecond, MaxBackoff: 50 * time.Millisecond, Logger: slog.New(slog.DiscardHandler)})
	r.wait(t, "connected")
	srv.Down()
	r.wait(t, "disconnected")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Call(ctx, map[string]any{"type": "get_states"}, nil); !errors.Is(err, ha.ErrNotConnected) && !errors.Is(err, ha.ErrDisconnected) {
		t.Fatalf("call while down: %v", err)
	}
	r.wait(t, "disconnected") // a refused attempt
	srv.Up()
	r.wait(t, "connected")
	if srv.Authenticated() != 2 {
		t.Fatalf("authenticated %d times", srv.Authenticated())
	}
}

func TestClientAuthInvalid(t *testing.T) {
	srv := hatest.New(t)
	c, r, _ := start(t, ha.Options{URL: srv.URL(), Token: "wrong", MinBackoff: 10 * time.Millisecond, MaxBackoff: 20 * time.Millisecond, Logger: slog.New(slog.DiscardHandler)})
	r.wait(t, "disconnected")
	r.mu.Lock()
	err := r.errs[0]
	r.mu.Unlock()
	if !errors.Is(err, ha.ErrAuthInvalid) || c.Connected() {
		t.Fatalf("auth with a wrong token: %v", err)
	}
	if strings.Contains(err.Error(), "wrong") {
		t.Fatal("the token is in the error")
	}
}

func TestClientPing(t *testing.T) {
	srv := hatest.New(t)
	_, r, _ := start(t, ha.Options{URL: srv.URL(), Token: hatest.Token, PingInterval: 10 * time.Millisecond, Logger: slog.New(slog.DiscardHandler)})
	r.wait(t, "connected")
	time.Sleep(100 * time.Millisecond)
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.errs) != 0 {
		t.Fatalf("the pinged connection dropped: %v", r.errs)
	}
}

func TestClientNotConnected(t *testing.T) {
	c := ha.New(ha.Options{URL: "ws://127.0.0.1:1/api/websocket"})
	if _, err := c.GetStates(context.Background()); !errors.Is(err, ha.ErrNotConnected) {
		t.Fatalf("GetStates before Run: %v", err)
	}
}
