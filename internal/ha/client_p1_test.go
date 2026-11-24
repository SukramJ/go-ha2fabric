// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package ha_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"

	"thingfabric.dev/go-ha2fabric/internal/ha"
	"thingfabric.dev/go-ha2fabric/internal/ha/hatest"
)

// scripted is a hostile endpoint that authenticates every connection and
// hands each command to cmd, with a send that may be called from any
// goroutine. auths counts the auth messages it read.
func scripted(t *testing.T, auths *atomic.Int32, cmd func(ctx context.Context, send func(string), id int64, typ string, raw []byte)) string {
	t.Helper()
	return hostile(t, func(ctx context.Context, ws *websocket.Conn) {
		var mu sync.Mutex
		out := func(frame string) {
			mu.Lock()
			defer mu.Unlock()
			send(ctx, ws, frame)
		}
		out(`{"type":"auth_required"}`)
		if _, _, err := ws.Read(ctx); err != nil {
			return
		}
		if auths != nil {
			auths.Add(1)
		}
		out(`{"type":"auth_ok","ha_version":"2026.10.3"}`)
		for {
			_, b, err := ws.Read(ctx)
			if err != nil {
				return
			}
			var c struct {
				ID   int64  `json:"id"`
				Type string `json:"type"`
			}
			if json.Unmarshal(b, &c) != nil {
				return
			}
			cmd(ctx, out, c.ID, c.Type, b)
		}
	})
}

// TestSubscribeAbandonedIsUnsubscribed (HR-P1-ha-62): a subscription
// whose caller gave up before Home Assistant's result is unsubscribed
// once the result arrives, so Home Assistant's teardown runs — the
// connection stays.
func TestSubscribeAbandonedIsUnsubscribed(t *testing.T) {
	var mu sync.Mutex
	var offer int64
	var unsubscribed []int64
	url := scripted(t, nil, func(ctx context.Context, out func(string), id int64, typ string, raw []byte) {
		switch typ {
		case "camera/webrtc/offer":
			mu.Lock()
			offer = id
			mu.Unlock()
			go func() {
				time.Sleep(200 * time.Millisecond)
				out(`{"id":` + itoa(id) + `,"type":"result","success":true,"result":null}`)
				out(`{"id":` + itoa(id) + `,"type":"event","event":{"type":"session","session_id":"S1"}}`)
			}()
		case "unsubscribe_events":
			var u struct {
				Subscription int64 `json:"subscription"`
			}
			_ = json.Unmarshal(raw, &u)
			mu.Lock()
			unsubscribed = append(unsubscribed, u.Subscription)
			mu.Unlock()
			out(`{"id":` + itoa(id) + `,"type":"result","success":true,"result":null}`)
		default:
			out(`{"id":` + itoa(id) + `,"type":"result","success":true,"result":null}`)
		}
	})
	c, r, _ := start(t, ha.Options{URL: url, Token: "t", MinBackoff: time.Hour, Logger: slog.New(slog.DiscardHandler)})
	r.wait(t, "connected")
	octx, ocancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer ocancel()
	if _, err := c.Subscribe(octx, "camera/webrtc/offer", map[string]any{"entity_id": "camera.a", "offer": "v=0"}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Subscribe: %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		mu.Lock()
		got := slices.Clone(unsubscribed)
		id := offer
		mu.Unlock()
		if slices.Contains(got, id) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no unsubscribe_events for the offer %d: %v", id, got)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !c.Connected() {
		t.Fatal("the connection dropped")
	}
}

// TestSubscribeAbandonedRefusedSendsNothing (HR-P1-ha-62's negative): a
// late result that is a refusal holds no subscription, so nothing is
// unsubscribed.
func TestSubscribeAbandonedRefusedSendsNothing(t *testing.T) {
	var unsub atomic.Int32
	var answered atomic.Bool
	url := scripted(t, nil, func(ctx context.Context, out func(string), id int64, typ string, raw []byte) {
		switch typ {
		case "camera/webrtc/offer":
			go func() {
				time.Sleep(100 * time.Millisecond)
				out(`{"id":` + itoa(id) + `,"type":"result","success":false,"error":{"code":"home_assistant_error","message":"no"}}`)
				answered.Store(true)
			}()
		case "unsubscribe_events":
			unsub.Add(1)
			out(`{"id":` + itoa(id) + `,"type":"result","success":true,"result":null}`)
		default:
			out(`{"id":` + itoa(id) + `,"type":"result","success":true,"result":null}`)
		}
	})
	c, r, _ := start(t, ha.Options{URL: url, Token: "t", MinBackoff: time.Hour, Logger: slog.New(slog.DiscardHandler)})
	r.wait(t, "connected")
	octx, ocancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer ocancel()
	if _, err := c.Subscribe(octx, "camera/webrtc/offer", map[string]any{"entity_id": "camera.a", "offer": "v=0"}); err == nil {
		t.Fatal("Subscribe succeeded")
	}
	for !answered.Load() {
		time.Sleep(5 * time.Millisecond)
	}
	// A ping round trip orders anything the client sent after the late
	// result before this check.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Call(ctx, map[string]any{"type": "ping"}, nil); err != nil {
		t.Fatal(err)
	}
	if n := unsub.Load(); n != 0 {
		t.Fatalf("unsubscribe_events sent %d times for a refused offer", n)
	}
}

// TestClientBackoffGrowsAfterShortConnection (HR-P1-ha-07, decision 28):
// a connection that authenticates and drops at once does not reset the
// back-off; the delay keeps doubling to MaxBackoff.
func TestClientBackoffGrowsAfterShortConnection(t *testing.T) {
	var auths atomic.Int32
	url := hostile(t, func(ctx context.Context, ws *websocket.Conn) {
		send(ctx, ws, `{"type":"auth_required"}`)
		if _, _, err := ws.Read(ctx); err != nil {
			return
		}
		auths.Add(1)
		send(ctx, ws, `{"type":"auth_ok","ha_version":"2026.10.3"}`)
		_ = ws.Close(websocket.StatusGoingAway, "bye")
	})
	start(t, ha.Options{URL: url, Token: "t", MinBackoff: 20 * time.Millisecond, MaxBackoff: 160 * time.Millisecond, Logger: slog.New(slog.DiscardHandler)})
	time.Sleep(time.Second)
	// Doubling: 20, 40, 80, 160, 160, ... ms is about eight attempts in a
	// second; a reset each time is about fifty.
	if n := auths.Load(); n > 15 {
		t.Fatalf("%d authentications in a second: the back-off was reset after each short connection", n)
	}
}

// TestClientBackoffResetsAfterStableConnection (HR-P1-ha-07's
// negative): a connection that stayed up longer than MaxBackoff resets
// the back-off, so the next attempt follows after MinBackoff.
func TestClientBackoffResetsAfterStableConnection(t *testing.T) {
	srv := hatest.New(t)
	_, r, _ := start(t, ha.Options{URL: srv.URL(), Token: hatest.Token, MinBackoff: 10 * time.Millisecond, MaxBackoff: 50 * time.Millisecond, Logger: slog.New(slog.DiscardHandler)})
	r.wait(t, "connected")
	time.Sleep(100 * time.Millisecond)
	srv.Down()
	r.wait(t, "disconnected")
	srv.Up()
	began := time.Now()
	r.wait(t, "connected")
	if d := time.Since(began); d > time.Second {
		t.Fatalf("reconnected after %v", d)
	}
}

// TestClientAuthRefusedBacksOffLong (HR-P1-ha-08, decision 28): after
// auth_invalid the client waits AuthBackoff, not MaxBackoff, and reports
// the refusal for the status page; an authentication clears it.
func TestClientAuthRefusedBacksOffLong(t *testing.T) {
	srv := hatest.New(t)
	var logs syncBuffer
	c, r, _ := start(t, ha.Options{
		URL: srv.URL(), Token: "wrong", MinBackoff: 10 * time.Millisecond, MaxBackoff: 20 * time.Millisecond,
		AuthBackoff: time.Hour, Logger: slog.New(slog.NewTextHandler(&logs, nil)),
	})
	r.wait(t, "disconnected")
	time.Sleep(300 * time.Millisecond)
	r.mu.Lock()
	attempts := len(r.errs)
	r.mu.Unlock()
	if attempts != 1 {
		t.Fatalf("%d attempts after auth_invalid within 300 ms", attempts)
	}
	if since, refused := c.TokenRefused(); !refused || since.IsZero() {
		t.Fatal("the refusal is not reported")
	}
	if !strings.Contains(logs.String(), "ha.token_refused") {
		t.Fatalf("no ha.token_refused line\n%s", logs.String())
	}
	if strings.Contains(logs.String(), "wrong") {
		t.Fatal("the token reached the log")
	}
}

// TestClientAuthRefusedClears (HR-P1-ha-08's negative): a client that
// authenticates reports no refusal.
func TestClientAuthRefusedClears(t *testing.T) {
	srv := hatest.New(t)
	c, r, _ := start(t, ha.Options{URL: srv.URL(), Token: hatest.Token, Logger: slog.New(slog.DiscardHandler)})
	r.wait(t, "connected")
	if _, refused := c.TokenRefused(); refused {
		t.Fatal("refused after an authentication")
	}
}

// TestClientAuthRefusedThenAccepted (HR-P1-ha-92): a refusal ends with
// the next authentication, not with the end of that connection.
func TestClientAuthRefusedThenAccepted(t *testing.T) {
	var n atomic.Int32
	url := hostile(t, func(ctx context.Context, ws *websocket.Conn) {
		send(ctx, ws, `{"type":"auth_required"}`)
		if _, _, err := ws.Read(ctx); err != nil {
			return
		}
		if n.Add(1) == 1 {
			send(ctx, ws, `{"type":"auth_invalid","message":"no"}`)
			return
		}
		send(ctx, ws, `{"type":"auth_ok","ha_version":"2026.10.3"}`)
		<-ctx.Done()
	})
	c, r, _ := start(t, ha.Options{URL: url, Token: "t", AuthBackoff: 50 * time.Millisecond, Logger: slog.New(slog.DiscardHandler)})
	r.wait(t, "connected")
	if _, refused := c.TokenRefused(); refused {
		t.Fatal("authenticated, the refusal still reported")
	}
}

// TestListSkipsEntryThatDoesNotDecode (HR-P1-data-09, -data-90): one
// registry entry with a wrong-typed field costs that entry, not the list,
// and the list names it (ha.SkippedError); the skip is logged once.
func TestListSkipsEntryThatDoesNotDecode(t *testing.T) {
	srv := hatest.New(t)
	var logs syncBuffer
	c, r, _ := start(t, ha.Options{URL: srv.URL(), Token: hatest.Token, Logger: slog.New(slog.NewTextHandler(&logs, nil))})
	r.wait(t, "connected")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	devs, err := c.ListDevices(ctx)
	if err != nil || len(devs) < 2 {
		t.Fatalf("devices %d %v", len(devs), err)
	}
	ents, err := c.ListEntities(ctx)
	if err != nil || len(ents) < 2 {
		t.Fatalf("entities %d %v", len(ents), err)
	}
	srv.UpdateDevice(devs[0].ID, map[string]any{"name": 7})
	srv.UpdateEntity(ents[0].EntityID, map[string]any{"original_name": 5})
	skippedIs := func(err error, id string) bool {
		se, ok := errors.AsType[*ha.SkippedError](err)
		return ok && slices.Equal(se.IDs, []string{id})
	}
	for range 2 {
		got, err := c.ListDevices(ctx)
		if !skippedIs(err, devs[0].ID) || len(got) != len(devs)-1 || slices.ContainsFunc(got, func(d ha.DeviceEntry) bool { return d.ID == devs[0].ID }) {
			t.Fatalf("devices after a bad entry: %d of %d, %v", len(got), len(devs), err)
		}
		gotE, err := c.ListEntities(ctx)
		if !skippedIs(err, ents[0].EntityID) || len(gotE) != len(ents)-1 {
			t.Fatalf("entities after a bad entry: %d of %d, %v", len(gotE), len(ents), err)
		}
	}
	if n := strings.Count(logs.String(), "ha.entry_skipped"); n != 2 {
		t.Fatalf("ha.entry_skipped logged %d times, want once per list\n%s", n, logs.String())
	}
}

// TestCallContextKeepsConnection (HR-P1-conc-22): a caller's context
// that ends while its command is being written does not close the one
// connection every node shares.
func TestCallContextKeepsConnection(t *testing.T) {
	release := make(chan struct{})
	url := hostile(t, func(ctx context.Context, ws *websocket.Conn) {
		ws.SetReadLimit(-1)
		send(ctx, ws, `{"type":"auth_required"}`)
		if _, _, err := ws.Read(ctx); err != nil {
			return
		}
		send(ctx, ws, `{"type":"auth_ok","ha_version":"2026.10.3"}`)
		<-release // stop reading until the test lets go
		for {
			_, b, err := ws.Read(ctx)
			if err != nil {
				return
			}
			var c struct {
				ID int64 `json:"id"`
			}
			_ = json.Unmarshal(b, &c)
			send(ctx, ws, `{"id":`+itoa(c.ID)+`,"type":"result","success":true,"result":null}`)
		}
	})
	c, r, _ := start(t, ha.Options{URL: url, Token: "t", MinBackoff: time.Hour, PingInterval: time.Hour, CallTimeout: 10 * time.Second, Logger: slog.New(slog.DiscardHandler)})
	r.wait(t, "connected")
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	go func() {
		time.Sleep(1200 * time.Millisecond)
		close(release)
	}()
	// Larger than the socket buffers, so the write is in progress when
	// the context ends.
	began := time.Now()
	err := c.Call(ctx, map[string]any{"type": "get_states", "pad": strings.Repeat("x", 24<<20)}, nil)
	if time.Since(began) < time.Second {
		t.Logf("the write ended before the server read again: %v", err)
	} else {
		t.Logf("call: %v after %v", err, time.Since(began))
	}
	time.Sleep(200 * time.Millisecond)
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.errs) != 0 || !c.Connected() {
		t.Fatalf("the connection dropped: %v", r.errs)
	}
}

// orderRecorder is a Handler that logs the order of its calls; Event
// blocks on the first event for a while.
type orderRecorder struct {
	mu    sync.Mutex
	seen  []string
	first sync.Once
	down  chan struct{}
}

func (o *orderRecorder) add(s string) {
	o.mu.Lock()
	o.seen = append(o.seen, s)
	o.mu.Unlock()
}
func (o *orderRecorder) Connected(string) { o.add("connected") }
func (o *orderRecorder) Event(ha.Event) {
	o.first.Do(func() { time.Sleep(300 * time.Millisecond) })
	o.add("event")
}

func (o *orderRecorder) Disconnected(error) {
	o.add("disconnected")
	select {
	case o.down <- struct{}{}:
	default:
	}
}

// TestNoEventAfterDisconnected (HR-P1-conc-71): the connection's reader
// is joined before Disconnected, so the Handler's order holds even when
// the connection ends while the reader is inside Event.
func TestNoEventAfterDisconnected(t *testing.T) {
	url := scripted(t, nil, func(ctx context.Context, out func(string), id int64, typ string, raw []byte) {
		if typ != "subscribe_events" {
			return // the ping is never answered
		}
		out(`{"id":` + itoa(id) + `,"type":"result","success":true,"result":null}`)
		ev := `{"id":` + itoa(id) + `,"type":"event","event":{"event_type":"state_changed","data":{}}}`
		out(`[` + ev + `,` + ev + `]`)
	})
	o := &orderRecorder{down: make(chan struct{}, 1)}
	c := ha.New(ha.Options{URL: url, Token: "t", Events: []string{ha.EventStateChanged}, PingInterval: 10 * time.Millisecond, CallTimeout: 50 * time.Millisecond, MinBackoff: time.Hour, Logger: slog.New(slog.DiscardHandler)})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = c.Run(ctx, o)
	}()
	select {
	case <-o.down:
	case <-time.After(10 * time.Second):
		t.Fatal("no disconnected")
	}
	time.Sleep(500 * time.Millisecond)
	cancel()
	<-done
	o.mu.Lock()
	defer o.mu.Unlock()
	if i := slices.Index(o.seen, "disconnected"); i < 0 || slices.Contains(o.seen[i:], "event") {
		t.Fatalf("an event followed disconnected: %v", o.seen)
	}
}
