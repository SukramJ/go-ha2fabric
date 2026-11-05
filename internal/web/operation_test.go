// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package web

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	fabricui "github.com/SukramJ/go-fabric-ui/fabricui"
)

func do(t *testing.T, h http.Handler, method, path, body string, want int, out any) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != want {
		t.Fatalf("%s %s: %d, want %d: %.300s", method, path, rec.Code, want, rec.Body.String())
	}
	if out != nil {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			t.Fatalf("%s %s: %v: %.300s", method, path, err, rec.Body.String())
		}
	}
}

// TestOperationPage: the Operation routes answer over the daemon's log
// ring — the level at start is LOG_LEVEL's, a level set on the page moves
// the one LevelVar that gates the ring and stderr, the log shows the
// daemon's records, the stream carries them as they arrive, and info
// names the daemon's version.
func TestOperationPage(t *testing.T) {
	lvl := new(slog.LevelVar)
	lvl.Set(slog.LevelInfo)
	var stderr strings.Builder
	log, ring := NewLogger(&stderr, lvl)
	s := New(Options{Logger: log, UI: fabricui.Options{
		Admin:      func(*http.Request) bool { return true },
		Operations: NewOperations(ring, "1.2.3", time.Now().Add(-time.Minute)),
	}})
	h := s.Handler()

	type level struct {
		Level  string   `json:"level"`
		Levels []string `json:"levels"`
	}
	var got level
	do(t, h, http.MethodGet, "/api/v1/operation/log/level", "", http.StatusOK, &got)
	if got.Level != "info" || len(got.Levels) == 0 {
		t.Fatalf("level at start: %+v", got)
	}
	do(t, h, http.MethodPut, "/api/v1/operation/log/level", `{"level":"warn"}`, http.StatusOK, &got)
	if got.Level != "warn" || lvl.Level() != slog.LevelWarn {
		t.Fatalf("after PUT warn: %+v (LevelVar %v)", got, lvl.Level())
	}
	do(t, h, http.MethodPut, "/api/v1/operation/log/level", `{"level":"loud"}`, http.StatusBadRequest, nil)

	log.Info("ha2fabric.test.below") // under the gate: neither ring nor stderr
	log.Warn("ha2fabric.test.marker", slog.Group("peer", slog.Int("node", 7)))
	if strings.Contains(stderr.String(), "ha2fabric.test.below") || !strings.Contains(stderr.String(), "ha2fabric.test.marker") {
		t.Fatalf("stderr does not follow the level: %q", stderr.String())
	}
	var recs struct {
		Records []struct {
			Seq   uint64            `json:"seq"`
			Level string            `json:"level"`
			Msg   string            `json:"msg"`
			Attrs map[string]string `json:"attrs"`
		} `json:"records"`
	}
	do(t, h, http.MethodGet, "/api/v1/operation/log", "", http.StatusOK, &recs)
	var found bool
	for _, r := range recs.Records {
		if r.Msg == "ha2fabric.test.below" {
			t.Fatalf("a record under the gate is in the ring: %+v", r)
		}
		if r.Msg == "ha2fabric.test.marker" {
			found = r.Level == "warn" && r.Attrs["peer.node"] == "7"
		}
	}
	if !found {
		t.Fatalf("the daemon's record is not in the log: %+v", recs.Records)
	}

	var info struct {
		Version string `json:"version"`
		UptimeS int64  `json:"uptime_s"`
	}
	do(t, h, http.MethodGet, "/api/v1/operation/info", "", http.StatusOK, &info)
	if info.Version != "1.2.3" || info.UptimeS < 60 {
		t.Fatalf("info %+v", info)
	}

	srv := httptest.NewServer(h)
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/v1/operation/log/stream", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("stream: %d %q", res.StatusCode, res.Header.Get("Content-Type"))
	}
	lines := bufio.NewReader(res.Body)
	// ": connected" follows the observer's registration.
	if l, err := lines.ReadString('\n'); err != nil || !strings.HasPrefix(l, ": connected") {
		t.Fatalf("stream opened with %q, %v", l, err)
	}
	log.Error("ha2fabric.test.streamed")
	var event string
	for {
		l, err := lines.ReadString('\n')
		if err != nil {
			t.Fatalf("stream ended before the record: %v", err)
		}
		if strings.HasPrefix(l, "event: ") {
			event = strings.TrimSpace(strings.TrimPrefix(l, "event: "))
		}
		if strings.HasPrefix(l, "data: ") && strings.Contains(l, "ha2fabric.test.streamed") {
			if event != fabricui.EventLogRecord || !strings.Contains(l, `"level":"error"`) {
				t.Fatalf("streamed %q as %q", l, event)
			}
			break
		}
	}
}

// TestOperationsAdapter: the port maps the ring one to one and observes
// what arrives.
func TestOperationsAdapter(t *testing.T) {
	lvl := new(slog.LevelVar)
	log, ring := NewLogger(io.Discard, lvl)
	ops := NewOperations(ring, "1.2.3", time.Now().Add(-time.Minute))
	got := make(chan fabricui.LogRecord, 1)
	stop := ops.ObserveLog(func(r fabricui.LogRecord) { got <- r })
	defer stop()
	log.With(slog.String("entity", "light.kitchen")).Info("bridge.synced")
	select {
	case r := <-got:
		if r.Seq != 1 || r.Message != "bridge.synced" || r.Attrs["entity"] != "light.kitchen" || r.Level != slog.LevelInfo {
			t.Fatalf("observed %+v", r)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no record observed")
	}
	if recs := ops.Log(1, 0); len(recs) != 1 || recs[0].Seq != 1 {
		t.Fatalf("log %+v", recs)
	}
	if err := ops.SetLogLevel(slog.LevelError); err != nil || lvl.Level() != slog.LevelError || ops.LogLevel() != slog.LevelError {
		t.Fatalf("SetLogLevel: %v, LevelVar %v", err, lvl.Level())
	}
	if in := ops.Info(); in.Uptime < time.Minute || in.StartedAt.IsZero() || in.Version != "1.2.3" {
		t.Fatalf("info %+v", in)
	}
	if ops.ObserveLog(nil) == nil {
		t.Fatal("ObserveLog(nil) returned no unsubscribe")
	}
}
