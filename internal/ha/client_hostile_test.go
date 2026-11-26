// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package ha_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"thingfabric.dev/go-ha2fabric/internal/ha"
)

// hostile is a WebSocket endpoint that runs script on each connection,
// in place of Home Assistant; it returns the endpoint's URL.
func hostile(t *testing.T, script func(ctx context.Context, ws *websocket.Conn)) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = ws.CloseNow() }()
		script(r.Context(), ws)
	}))
	t.Cleanup(srv.Close)
	return "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/websocket"
}

func send(ctx context.Context, ws *websocket.Conn, frame string) {
	_ = ws.Write(ctx, websocket.MessageText, []byte(frame))
}

// firstError starts a client on url and returns the error of its first
// failed attempt.
func firstError(t *testing.T, url string) error {
	t.Helper()
	_, r, _ := start(t, ha.Options{URL: url, Token: "t", MinBackoff: time.Hour, Logger: slog.New(slog.DiscardHandler)})
	r.wait(t, "disconnected")
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.errs[0]
}

// TestClientAuthHostile: every way the handshake can go wrong fails the
// attempt with an error that says how, and never connects
// (client.go auth, readOne).
func TestClientAuthHostile(t *testing.T) {
	for name, c := range map[string]struct {
		script func(ctx context.Context, ws *websocket.Conn)
		want   string
		is     error
	}{
		"not auth_required first": {func(ctx context.Context, ws *websocket.Conn) {
			send(ctx, ws, `{"type":"auth_ok"}`)
		}, `expected auth_required, got "auth_ok"`, nil},
		"a frame that does not decode": {func(ctx context.Context, ws *websocket.Conn) {
			send(ctx, ws, `nope`)
		}, "", ha.ErrFrame},
		"closed before auth_required": {func(ctx context.Context, ws *websocket.Conn) {
			_ = ws.Close(websocket.StatusGoingAway, "bye")
		}, "ha: read", nil},
		"neither auth_ok nor auth_invalid": {func(ctx context.Context, ws *websocket.Conn) {
			send(ctx, ws, `{"type":"auth_required"}`)
			_, _, _ = ws.Read(ctx)
			send(ctx, ws, `{"type":"result","id":1}`)
		}, `expected auth_ok, got "result"`, nil},
		"closed before the auth reply": {func(ctx context.Context, ws *websocket.Conn) {
			send(ctx, ws, `{"type":"auth_required"}`)
			_, _, _ = ws.Read(ctx)
			_ = ws.Close(websocket.StatusGoingAway, "bye")
		}, "ha: read", nil},
		"a reply that does not decode": {func(ctx context.Context, ws *websocket.Conn) {
			send(ctx, ws, `{"type":"auth_required"}`)
			_, _, _ = ws.Read(ctx)
			send(ctx, ws, `[]`)
		}, "", ha.ErrFrame},
	} {
		t.Run(name, func(t *testing.T) {
			err := firstError(t, hostile(t, c.script))
			if err == nil || c.want != "" && !strings.Contains(err.Error(), c.want) || c.is != nil && !errors.Is(err, c.is) {
				t.Fatalf("%v", err)
			}
		})
	}
}

// TestClientHostileFrames: on an authenticated connection a frame that
// does not decode and an event whose payload is not an event are logged
// and skipped, and a result that does not decode fails its call — the
// connection stays (client.go read: ha.frame_refused, ha.event_refused;
// Call).
func TestClientHostileFrames(t *testing.T) {
	url := hostile(t, func(ctx context.Context, ws *websocket.Conn) {
		send(ctx, ws, `{"type":"auth_required"}`)
		if _, _, err := ws.Read(ctx); err != nil {
			return
		}
		send(ctx, ws, `{"type":"auth_ok","ha_version":"2026.10.3"}`)
		for {
			_, b, err := ws.Read(ctx)
			if err != nil {
				return
			}
			var cmd struct {
				ID   int64  `json:"id"`
				Type string `json:"type"`
			}
			if json.Unmarshal(b, &cmd) != nil {
				return
			}
			switch cmd.Type {
			case "subscribe_events":
				send(ctx, ws, `{"id":`+itoa(cmd.ID)+`,"type":"result","success":true,"result":null}`)
				send(ctx, ws, `{"broken`)
				send(ctx, ws, `{"id":`+itoa(cmd.ID)+`,"type":"event","event":"not an event"}`)
			case "get_states":
				send(ctx, ws, `{"id":`+itoa(cmd.ID)+`,"type":"result","success":true,"result":"not a list"}`)
			}
		}
	})
	var logs syncBuffer
	c, r, _ := start(t, ha.Options{URL: url, Token: "t", Events: []string{ha.EventStateChanged}, MinBackoff: time.Hour, Logger: slog.New(slog.NewTextHandler(&logs, nil))})
	r.wait(t, "connected")
	deadline := time.Now().Add(10 * time.Second)
	for !strings.Contains(logs.String(), "ha.event_refused") || !strings.Contains(logs.String(), "ha.frame_refused") {
		if time.Now().After(deadline) {
			t.Fatalf("no refusal logged\n%s", logs.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := c.GetStates(ctx); err == nil || !strings.Contains(err.Error(), "get_states result") {
		t.Fatalf("GetStates on a result that does not decode: %v", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.errs) != 0 || len(r.events) != 0 || !c.Connected() {
		t.Fatalf("the connection did not stay: errors %v, events %v", r.errs, r.events)
	}
}

func itoa(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}

// TestClientReplyShape (HR-P3-ha-09): a reply to a command is a result
// with success, as core sends every one (websocket_api/messages.py:63-65
// result_message, :81-104 error_message at 2026.10.3); a reply of another
// type, or a result without success, fails the call with ErrFrame and the
// connection stays.
func TestClientReplyShape(t *testing.T) {
	for name, reply := range map[string]string{
		"a result without success": `{"id":%d,"type":"result","result":{"context":{}}}`,
		"another type":             `{"id":%d,"type":"pong"}`,
	} {
		t.Run(name, func(t *testing.T) {
			url := hostile(t, func(ctx context.Context, ws *websocket.Conn) {
				send(ctx, ws, `{"type":"auth_required"}`)
				if _, _, err := ws.Read(ctx); err != nil {
					return
				}
				send(ctx, ws, `{"type":"auth_ok","ha_version":"2026.10.3"}`)
				for {
					_, b, err := ws.Read(ctx)
					if err != nil {
						return
					}
					var cmd struct {
						ID   int64  `json:"id"`
						Type string `json:"type"`
					}
					if json.Unmarshal(b, &cmd) != nil {
						return
					}
					if cmd.Type == "call_service" {
						send(ctx, ws, fmt.Sprintf(reply, cmd.ID))
						continue
					}
					send(ctx, ws, `{"id":`+itoa(cmd.ID)+`,"type":"result","success":true,"result":null}`)
				}
			})
			c, r, _ := start(t, ha.Options{URL: url, Token: "t", MinBackoff: time.Hour})
			r.wait(t, "connected")
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := c.CallService(ctx, ha.ServiceCall{Domain: "light", Service: "turn_on"}); !errors.Is(err, ha.ErrFrame) {
				t.Fatalf("CallService on %s: %v, want ErrFrame", name, err)
			}
			if !c.Connected() {
				t.Fatal("the connection did not stay")
			}
		})
	}
}
