// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package ha_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/SukramJ/go-ha2fabric/internal/ha"
)

// bigState is a state object of about n bytes, its attributes the bulk.
func bigState(id string, n int) string {
	return `{"entity_id":"` + id + `","state":"on","attributes":{"friendly_name":"Big","blob":"` + strings.Repeat("x", n) +
		`"},"last_changed":"2026-11-24T10:00:00Z","last_updated":"2026-11-24T10:00:01Z"}`
}

// TestStateOverTheCap (HR-P2-bound-38): a state over MaxStateBytes keeps
// its id, its state and its times and drops its attributes, saying so in
// Oversized; one under it decodes whole. The same holds inside
// state_changed.
func TestStateOverTheCap(t *testing.T) {
	var st ha.State
	if err := json.Unmarshal([]byte(bigState("sensor.big", ha.MaxStateBytes)), &st); err != nil {
		t.Fatal(err)
	}
	if st.EntityID != "sensor.big" || st.State != "on" || st.LastUpdated != "2026-11-24T10:00:01Z" || st.Attributes != nil || st.Oversized <= ha.MaxStateBytes {
		t.Fatalf("a state over the cap: %q %q %q attributes %d, oversized %d", st.EntityID, st.State, st.LastUpdated, len(st.Attributes), st.Oversized)
	}
	var small ha.State
	if err := json.Unmarshal([]byte(bigState("sensor.small", 100)), &small); err != nil {
		t.Fatal(err)
	}
	if small.Attributes["friendly_name"] != "Big" || small.Oversized != 0 {
		t.Fatalf("a state under the cap: %+v", small)
	}
	var sc ha.StateChanged
	if err := json.Unmarshal([]byte(`{"entity_id":"sensor.big","old_state":null,"new_state":`+bigState("sensor.big", ha.MaxStateBytes)+`}`), &sc); err != nil {
		t.Fatal(err)
	}
	if sc.NewState == nil || sc.NewState.Attributes != nil || sc.NewState.Oversized == 0 {
		t.Fatalf("state_changed over the cap: %+v", sc.NewState)
	}
}

// TestGetStatesLogsTheOversized (HR-P2-bound-38): get_states names a
// state over the cap in the log, with its entity id, once per entity.
func TestGetStatesLogsTheOversized(t *testing.T) {
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
			if cmd.Type == "get_states" {
				send(ctx, ws, `{"id":`+itoa(cmd.ID)+`,"type":"result","success":true,"result":[`+bigState("sensor.big", ha.MaxStateBytes)+`,`+bigState("sensor.small", 10)+`]}`)
			}
		}
	})
	var logs syncBuffer
	c, r, _ := start(t, ha.Options{URL: url, Token: "t", MinBackoff: time.Hour, Logger: slog.New(slog.NewTextHandler(&logs, nil))})
	r.wait(t, "connected")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for range 2 {
		states, err := c.GetStates(ctx)
		if err != nil || len(states) != 2 || states[0].Attributes != nil || states[1].Attributes == nil {
			t.Fatalf("GetStates: %v, %d states", err, len(states))
		}
	}
	if n := strings.Count(logs.String(), "ha.state_oversized"); n != 1 || !strings.Contains(logs.String(), "entity_id=sensor.big") {
		t.Fatalf("%d log lines:\n%s", n, logs.String())
	}
}

// TestReadLimitIsNamed (HR-P2-bound-37): a result over the read limit
// fails its command with an error that says so — not only "connection
// dropped" — and the connection's end carries it too.
func TestReadLimitIsNamed(t *testing.T) {
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
			if cmd.Type == "get_states" {
				send(ctx, ws, `{"id":`+itoa(cmd.ID)+`,"type":"result","success":true,"result":[`+bigState("sensor.big", 8192)+`]}`)
			}
		}
	})
	c, r, _ := start(t, ha.Options{URL: url, Token: "t", ReadLimit: 4096, MinBackoff: time.Hour, Logger: slog.New(slog.DiscardHandler)})
	r.wait(t, "connected")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := c.GetStates(ctx)
	if !errors.Is(err, ha.ErrDisconnected) || !errors.Is(err, ha.ErrReadLimit) || !strings.Contains(err.Error(), "4096") {
		t.Fatalf("GetStates over the read limit: %v", err)
	}
	r.wait(t, "disconnected")
	r.mu.Lock()
	defer r.mu.Unlock()
	if !errors.Is(r.errs[0], ha.ErrReadLimit) {
		t.Fatalf("the connection ended with %v", r.errs[0])
	}
}
