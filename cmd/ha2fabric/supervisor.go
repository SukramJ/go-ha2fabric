// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	fabricui "github.com/SukramJ/go-fabric-ui/fabricui"
)

// supervisorInfoURL is the Supervisor's GET /info, which an add-on's
// token may read without hassio_api (supervisor/api/middleware/
// security.py:107-116, :447-451; the answer's "supervisor" field,
// api/root.py:40-44, at fe91b8db).
const supervisorInfoURL = "http://supervisor/info"

// minSupervisor is the first Supervisor version that sets the Ingress
// user's groups (web.UserGroupsHeader) and drops a value the browser
// sent: 2026.10.4 (Supervisor commit 3fbb083b; the header constant is in
// supervisor/api/ingress.py from that tag on). Before it the header
// cannot tell an administrator.
var minSupervisor = [3]int{2026, 10, 4}

// minSupervisorText is minSupervisor as the messages name it.
const minSupervisorText = "2026.10.4"

// Warning codes of the gate.
const (
	warningSupervisorTooOld  = "supervisor_too_old"
	warningSupervisorUnknown = "supervisor_unknown"
)

// gateState is what the gate knows about the Supervisor.
type gateState int32

const (
	gateUnknown gateState = iota // not read yet: no administrator
	gateOpen                     // minSupervisor or later
	gateClosed                   // older, or a version that does not parse
)

// supervisorGate decides whether the add-on's web UI may have an
// administrator at all: only on a Supervisor of minSupervisor or later.
// Until its version is read, and below it, the UI is the status page
// only, with a warning. run reads the version at start and again every
// retry until it has one.
type supervisorGate struct {
	url    string
	token  string
	client *http.Client
	log    *slog.Logger
	retry  time.Duration

	state atomic.Int32

	mu      sync.Mutex
	version string
	err     string
	since   time.Time
}

// newSupervisorGate is the add-on's gate on the Supervisor's API.
func newSupervisorGate(token string, log *slog.Logger) *supervisorGate {
	return &supervisorGate{
		url: supervisorInfoURL, token: token, log: log, retry: 30 * time.Second,
		client: &http.Client{Timeout: 10 * time.Second},
		since:  time.Now(),
	}
}

// open reports whether the Supervisor is known to be recent enough.
func (g *supervisorGate) open() bool { return gateState(g.state.Load()) == gateOpen }

// run reads the Supervisor's version until it has one or ctx ends.
func (g *supervisorGate) run(ctx context.Context) {
	for attempt := 0; ; attempt++ {
		v, err := g.read(ctx)
		if err == nil {
			g.mu.Lock()
			g.version, g.err, g.since = v, "", time.Now()
			g.mu.Unlock()
			if atLeast(v, minSupervisor) {
				g.state.Store(int32(gateOpen))
				g.log.Info("ha2fabric.supervisor", slog.String("version", v), slog.Bool("admin", true))
			} else {
				g.state.Store(int32(gateClosed))
				g.log.Warn("ha2fabric.supervisor", slog.String("version", v), slog.Bool("admin", false),
					slog.String("hint", "the web UI's administrator pages need Supervisor "+minSupervisorText+" or later; until the Supervisor is updated the web UI shows the status page only"))
			}
			return
		}
		if ctx.Err() != nil {
			return
		}
		g.mu.Lock()
		g.err = err.Error()
		g.mu.Unlock()
		level := slog.LevelDebug
		if attempt == 0 {
			level = slog.LevelWarn
		}
		g.log.Log(ctx, level, "ha2fabric.supervisor_unreadable", slog.String("error", err.Error()), slog.Duration("retry_in", g.retry),
			slog.String("hint", "the web UI shows the status page only until the Supervisor's version is read"))
		t := time.NewTimer(g.retry)
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
		}
	}
}

// read is GET /info's supervisor version.
func (g *supervisorGate) read(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.url, http.NoBody)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+g.token)
	res, err := g.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("supervisor: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("supervisor: GET /info answered %d", res.StatusCode)
	}
	var body struct {
		Result string `json:"result"`
		Data   struct {
			Supervisor string `json:"supervisor"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&body); err != nil {
		return "", fmt.Errorf("supervisor: GET /info: %w", err)
	}
	if body.Result != "ok" || body.Data.Supervisor == "" {
		return "", fmt.Errorf("supervisor: GET /info without a version (result %q)", body.Result)
	}
	return body.Data.Supervisor, nil
}

// Warnings are the gate's while it does not open.
func (g *supervisorGate) Warnings() []fabricui.Warning {
	g.mu.Lock()
	defer g.mu.Unlock()
	switch gateState(g.state.Load()) {
	case gateClosed:
		return []fabricui.Warning{{
			Code:    warningSupervisorTooOld,
			Message: "the Supervisor is " + g.version + "; the web UI's administrator pages need Supervisor " + minSupervisorText + " or later, which tells the add-on whether the Home Assistant user is an administrator: update the Supervisor, then restart the add-on. Until then the web UI shows the status page only",
			Since:   g.since,
		}}
	case gateUnknown:
		msg := "the Supervisor's version has not been read yet"
		if g.err != "" {
			msg += " (" + g.err + ")"
		}
		return []fabricui.Warning{{
			Code:    warningSupervisorUnknown,
			Message: msg + "; the web UI's administrator pages need Supervisor " + minSupervisorText + " or later, and until the version is read the web UI shows the status page only",
			Since:   g.since,
		}}
	case gateOpen:
	}
	return nil
}

// atLeast reports whether the version v (year.month.patch, a suffix such
// as ".dev1201" or "b1" after the numbers ignored) is floor or later; a
// version that does not parse is not.
func atLeast(v string, floor [3]int) bool {
	parts := strings.SplitN(v, ".", 4)
	if len(parts) < 3 {
		return false
	}
	var got [3]int
	for i := range 3 {
		p := parts[i]
		end := 0
		for end < len(p) && p[end] >= '0' && p[end] <= '9' {
			end++
		}
		if end == 0 || i < 2 && end != len(p) {
			return false
		}
		n, err := strconv.Atoi(p[:end])
		if err != nil {
			return false
		}
		got[i] = n
	}
	for i := range 3 {
		if got[i] != floor[i] {
			return got[i] > floor[i]
		}
	}
	return true
}
