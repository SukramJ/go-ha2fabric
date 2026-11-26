// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	fabricui "thingfabric.dev/go-fabric-ui/fabricui"
)

// supervisorInfoURL is the Supervisor's GET /info, which an add-on's
// token may read without hassio_api (supervisor/api/middleware/
// security.py:107-116, :447-451; the answer's "supervisor" field,
// api/root.py:40-44, at fe91b8db).
const supervisorInfoURL = "http://supervisor/info"

// addonInfoURL is the Supervisor's GET /addons/self/info, which an
// add-on's token may read without hassio_api (the same bypass,
// security.py:107-116 at fe91b8db). Its "ip_address" is the address the
// Ingress proxy connects to (api/ingress.py:125-127, the add-on's
// ip_address; api/apps.py:308): for a host-network add-on the Supervisor
// network's gateway (docker/app.py:188-191), on the host's interface
// "hassio".
const addonInfoURL = "http://supervisor/addons/self/info"

// minSupervisor is the minimum Supervisor version of the admin check,
// which reads the Ingress user's groups (web.UserGroupsHeader): 2026.10.4
// (Supervisor commit 3fbb083b; the header constant is defined in
// supervisor/const.py, :113 at tag 2026.10.4 and :115 at fe91b8db, and
// used by supervisor/api/ingress.py; 2026.10.3 lacks it). Below it the
// add-on's web UI has no administrator.
var minSupervisor = [3]int{2026, 10, 4}

// minSupervisorText is minSupervisor as the messages name it.
const minSupervisorText = "2026.10.4"

// recheckInterval is how often a closed gate reads the Supervisor's
// version again. A Supervisor updates seldom (days apart) and GET /info is
// a cheap local call; ten minutes opens the web UI soon after an update
// without a restart and without a log line or request every few seconds.
const recheckInterval = 10 * time.Minute

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
	gateClosed                   // older, a pre-release of the floor, or a version that does not parse
)

// supervisorGate decides whether the add-on's web UI may have an
// administrator at all: only on a Supervisor of minSupervisor or later.
// Until its version is read, and below it, the UI is the status page
// only, with a warning. run reads the version at start, again every retry
// until it has one, and every recheck after: a Supervisor updated past
// the floor opens the gate, one downgraded below it closes it again
// (HR-P2-cfg-03).
type supervisorGate struct {
	url     string
	token   string
	client  *http.Client
	log     *slog.Logger
	retry   time.Duration
	recheck time.Duration

	state atomic.Int32

	mu      sync.Mutex
	version string
	err     string
	since   time.Time
}

// errSupervisorRedirect refuses a redirect of a request to the Supervisor,
// which carries SUPERVISOR_TOKEN.
var errSupervisorRedirect = errors.New("supervisor: the answer is a redirect, which is not followed")

// supervisorClient is the HTTP client of the Supervisor's API: bounded, no redirect
// followed and no proxy taken from the environment (HTTP_PROXY), so the
// Bearer header goes to the Supervisor and nowhere else.
func supervisorClient() *http.Client {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		base = &http.Transport{}
	}
	tr := base.Clone()
	tr.Proxy = nil
	return &http.Client{
		Timeout: 10 * time.Second, Transport: tr,
		CheckRedirect: func(*http.Request, []*http.Request) error { return errSupervisorRedirect },
	}
}

// newSupervisorGate is the add-on's gate on the Supervisor's API.
func newSupervisorGate(token string, log *slog.Logger) *supervisorGate {
	return &supervisorGate{
		url: supervisorInfoURL, token: token, log: log, retry: 30 * time.Second, recheck: recheckInterval,
		client: supervisorClient(),
		since:  time.Now(),
	}
}

// open reports whether the Supervisor is known to be recent enough.
func (g *supervisorGate) open() bool { return gateState(g.state.Load()) == gateOpen }

// run reads the Supervisor's version until ctx ends: every retry while
// it cannot be read, and every recheck once it has been read, so a
// Supervisor updated meanwhile opens the gate and one downgraded closes
// it, each without a restart. A read that fails after the gate opened
// leaves it open (the Supervisor restarting is no downgrade).
func (g *supervisorGate) run(ctx context.Context) {
	for attempt := 0; ; attempt++ {
		wait := g.retry
		v, err := g.read(ctx)
		switch {
		case err == nil && atLeast(v, minSupervisor):
			g.mu.Lock()
			changed := gateState(g.state.Load()) != gateOpen || g.version != v
			if changed {
				g.since = time.Now()
			}
			g.version, g.err = v, ""
			g.mu.Unlock()
			g.state.Store(int32(gateOpen))
			if changed {
				g.log.Info("ha2fabric.supervisor", slog.String("version", v), slog.Bool("admin", true))
			}
			wait = g.recheck
		case err == nil:
			g.mu.Lock()
			changed := gateState(g.state.Load()) != gateClosed || g.version != v
			if changed {
				g.since = time.Now()
			}
			g.version, g.err = v, ""
			g.mu.Unlock()
			g.state.Store(int32(gateClosed))
			if changed {
				g.log.Warn("ha2fabric.supervisor", slog.String("version", v), slog.Bool("admin", false), slog.Duration("recheck_in", g.recheck),
					slog.String("hint", "the web UI's administrator pages need Supervisor "+minSupervisorText+" or later; until the Supervisor is updated the web UI shows the status page only"))
			}
			wait = g.recheck
		case ctx.Err() != nil:
			return
		default:
			read := gateState(g.state.Load()) != gateUnknown
			g.mu.Lock()
			g.err = err.Error()
			g.mu.Unlock()
			level := slog.LevelDebug
			if attempt == 0 {
				level = slog.LevelWarn
			}
			if read {
				wait = g.recheck
			}
			g.log.Log(ctx, level, "ha2fabric.supervisor_unreadable", slog.String("error", err.Error()), slog.Duration("retry_in", wait),
				slog.String("hint", "the web UI shows the status page only until the Supervisor's version is read"))
		}
		t := time.NewTimer(wait)
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
	var data struct {
		Supervisor string `json:"supervisor"`
	}
	result, err := supervisorGet(ctx, g.client, g.url, g.token, &data)
	if err != nil {
		return "", err
	}
	if result != "ok" || data.Supervisor == "" {
		return "", fmt.Errorf("supervisor: GET %s without a version (result %q)", urlPath(g.url), result)
	}
	return data.Supervisor, nil
}

// supervisorGet reads one of the Supervisor's API answers: its "result"
// and its "data" into data.
func supervisorGet(ctx context.Context, client *http.Client, target, token string, data any) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, http.NoBody)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("supervisor: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("supervisor: GET %s answered %d", urlPath(target), res.StatusCode)
	}
	body := struct {
		Result string `json:"result"`
		Data   any    `json:"data"`
	}{Data: data}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&body); err != nil {
		return "", fmt.Errorf("supervisor: GET %s: %w", urlPath(target), err)
	}
	return body.Result, nil
}

// urlPath is the path of a Supervisor URL, as the messages name it.
func urlPath(u string) string {
	if p, err := url.Parse(u); err == nil && p.Path != "" {
		return p.Path
	}
	return u
}

// Warnings are the gate's while it does not open.
func (g *supervisorGate) Warnings() []fabricui.Warning {
	g.mu.Lock()
	defer g.mu.Unlock()
	switch gateState(g.state.Load()) {
	case gateClosed:
		if _, _, ok := parseVersion(g.version); !ok {
			return []fabricui.Warning{{
				Code:    warningSupervisorUnknown,
				Message: fmt.Sprintf("the Supervisor's version %q does not parse; the web UI's administrator pages need Supervisor %s or later, so until a version is read that does, the web UI shows the status page only", g.version, minSupervisorText),
				Since:   g.since,
			}}
		}
		return []fabricui.Warning{{
			Code:    warningSupervisorTooOld,
			Message: "the Supervisor is " + g.version + "; the web UI's administrator pages need Supervisor " + minSupervisorText + " or later, which tells the add-on whether the Home Assistant user is an administrator. Until the Supervisor is updated the web UI shows the status page only; the add-on reads the version again every few minutes",
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

// parseVersion reads year.month.patch from v; suffix reports anything
// after the third number (".dev1201", "b1", ".post1"); ok is false when v
// does not start with three numbers.
func parseVersion(v string) (nums [3]int, suffix, ok bool) {
	parts := strings.SplitN(v, ".", 4)
	if len(parts) < 3 {
		return nums, false, false
	}
	for i := range 3 {
		p := parts[i]
		end := 0
		for end < len(p) && p[end] >= '0' && p[end] <= '9' {
			end++
		}
		if end == 0 || i < 2 && end != len(p) {
			return nums, false, false
		}
		n, err := strconv.Atoi(p[:end])
		if err != nil {
			return nums, false, false
		}
		nums[i] = n
		if i == 2 && end != len(p) {
			suffix = true
		}
	}
	return nums, suffix || len(parts) == 4, true
}

// atLeast reports whether the version v is floor or later. A version with
// a suffix after its numbers is admitted only when the numbers are above
// floor: a development or pre-release build numbered for floor itself
// may predate what floor brings. A version that does not parse is not.
func atLeast(v string, floor [3]int) bool {
	got, suffix, ok := parseVersion(v)
	if !ok {
		return false
	}
	for i := range 3 {
		if got[i] != floor[i] {
			return got[i] > floor[i]
		}
	}
	return !suffix
}

// Reading the add-on's address at start: a few tries, so a Supervisor
// still answering its own start does not leave the wide bind for the
// whole run.
const (
	addressTries = 3
	addressPause = time.Second
)

// addonAddress is the address the Supervisor gives this add-on
// (GET /addons/self/info's ip_address): where its Ingress proxy connects.
// An unspecified address (the Supervisor's "no address") is none.
func addonAddress(ctx context.Context, client *http.Client, target, token string) (netip.Addr, error) {
	var data struct {
		IPAddress string `json:"ip_address"`
	}
	result, err := supervisorGet(ctx, client, target, token, &data)
	if err != nil {
		return netip.Addr{}, err
	}
	if result != "ok" {
		return netip.Addr{}, fmt.Errorf("supervisor: GET %s answered result %q", urlPath(target), result)
	}
	a, err := netip.ParseAddr(data.IPAddress)
	if err != nil || a.IsUnspecified() {
		return netip.Addr{}, fmt.Errorf("supervisor: GET %s without an address (ip_address %q)", urlPath(target), data.IPAddress)
	}
	return a.Unmap(), nil
}

// addonWebListen is the web UI's address in the add-on (decision 54 of
// the 2026-11-23 review): an unspecified host in WEB_LISTEN (the add-on's
// 0.0.0.0:8486) becomes the address the Supervisor gives the add-on, so
// the listener is not on the LAN; the port stays. Not found after
// addressTries, the bind stays as given, with one warning. A host set
// explicitly stays; the client filter (WEB_ALLOW) holds either way.
func addonWebListen(ctx context.Context, listen string, read func(context.Context) (netip.Addr, error), pause time.Duration, log *slog.Logger) string {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return listen
	}
	if host != "" {
		if ip, err := netip.ParseAddr(host); err != nil || !ip.IsUnspecified() {
			return listen
		}
	}
	for try := 1; ; try++ {
		a, err := read(ctx)
		if err == nil {
			addr := net.JoinHostPort(a.String(), port)
			log.Info("ha2fabric.web_listen", slog.String("listen", addr), slog.String("from", "the Supervisor's address of the add-on"))
			return addr
		}
		if try == addressTries || ctx.Err() != nil {
			log.Warn("ha2fabric.web_listen", slog.String("listen", listen), slog.String("error", err.Error()),
				slog.String("hint", "the add-on's address was not read from the Supervisor, so the web UI listens on every interface; it still serves the Ingress proxy only"))
			return listen
		}
		t := time.NewTimer(pause)
		select {
		case <-ctx.Done():
			t.Stop()
		case <-t.C:
		}
	}
}
