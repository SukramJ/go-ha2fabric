// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

// Package ha is the daemon's Home Assistant client: the WebSocket API
// (https://developers.home-assistant.io/docs/api/websocket) with its auth
// handshake, the id-tagged command envelope, ping keepalive, event
// subscriptions and a reconnect with back-off. Every payload shape is
// taken from Home Assistant core at the pinned release 2026.10.3; the
// citations are file:line at that tag.
//
// The access token (a long-lived token stand-alone, SUPERVISOR_TOKEN in
// the add-on) is held in memory only: it is sent in the auth message and
// never logged.
package ha

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
)

// Errors of the client.
var (
	// ErrAuthInvalid is Home Assistant's auth_invalid: the token is wrong
	// or revoked.
	ErrAuthInvalid = errors.New("ha: auth_invalid")
	// ErrNotConnected is a command while no connection is authenticated.
	ErrNotConnected = errors.New("ha: not connected")
	// ErrDisconnected is a command whose connection dropped before its
	// result arrived.
	ErrDisconnected = errors.New("ha: connection dropped")
)

// Defaults of [Options].
const (
	DefaultPingInterval = 30 * time.Second
	DefaultCallTimeout  = 30 * time.Second
	DefaultMinBackoff   = time.Second
	DefaultMaxBackoff   = time.Minute
	// DefaultReadLimit bounds one frame. get_states and the registry lists
	// of a large installation run to megabytes.
	DefaultReadLimit = 64 << 20
)

// Options configure a [Client].
type Options struct {
	// URL is the WebSocket endpoint: ws://supervisor/core/websocket in an
	// add-on, ws://<host>:8123/api/websocket stand-alone.
	URL string
	// Token is the access token. It is never logged.
	Token string
	// Events are the event types subscribed after every authentication.
	Events []string
	// PingInterval is the keepalive period; a ping unanswered within
	// CallTimeout drops the connection. 0 is DefaultPingInterval.
	PingInterval time.Duration
	// CallTimeout bounds a command without a deadline of its own. 0 is
	// DefaultCallTimeout.
	CallTimeout time.Duration
	// MinBackoff and MaxBackoff bound the reconnect delay, which doubles
	// per failed attempt. 0 is the default.
	MinBackoff, MaxBackoff time.Duration
	// ReadLimit bounds one frame; 0 is DefaultReadLimit.
	ReadLimit int64
	// Logger is the client's logger; nil is slog.Default().
	Logger *slog.Logger
}

// Handler receives the connection's life and its events. Every method
// runs on the client's goroutines and must not block: hand the work to a
// queue of your own.
type Handler interface {
	// Connected follows an authentication and the subscriptions: the
	// moment to read everything again.
	Connected(haVersion string)
	// Event is one event of a subscription.
	Event(ev Event)
	// Disconnected follows a dropped connection or a failed attempt.
	Disconnected(err error)
}

// Client is a reconnecting WebSocket API client. Its commands are safe for
// concurrent use; they fail with ErrNotConnected while no connection is
// authenticated.
type Client struct {
	opts Options
	log  *slog.Logger

	mu   sync.Mutex
	cur  *session
	conn atomic.Bool
}

// New returns a client; [Client.Run] connects it.
func New(opts Options) *Client {
	if opts.PingInterval <= 0 {
		opts.PingInterval = DefaultPingInterval
	}
	if opts.CallTimeout <= 0 {
		opts.CallTimeout = DefaultCallTimeout
	}
	if opts.MinBackoff <= 0 {
		opts.MinBackoff = DefaultMinBackoff
	}
	if opts.MaxBackoff <= 0 {
		opts.MaxBackoff = DefaultMaxBackoff
	}
	if opts.MaxBackoff < opts.MinBackoff {
		opts.MaxBackoff = opts.MinBackoff
	}
	if opts.ReadLimit <= 0 {
		opts.ReadLimit = DefaultReadLimit
	}
	log := opts.Logger
	if log == nil {
		log = slog.Default()
	}
	return &Client{opts: opts, log: log}
}

// Connected reports whether a connection is authenticated now.
func (c *Client) Connected() bool { return c.conn.Load() }

// Run connects, authenticates, subscribes and serves the connection until
// ctx ends; a dropped connection or a failed attempt is retried after a
// back-off that doubles up to MaxBackoff and starts again at MinBackoff
// after a connection that authenticated. It returns nil when ctx ends.
func (c *Client) Run(ctx context.Context, h Handler) error {
	backoff := c.opts.MinBackoff
	for {
		authed, err := c.serve(ctx, h)
		if ctx.Err() != nil {
			return nil //nolint:nilerr // the end of ctx is the end of Run, not a failure
		}
		h.Disconnected(err)
		if authed {
			backoff = c.opts.MinBackoff
		}
		c.log.Warn("ha.disconnected", slog.String("error", errString(err)), slog.Duration("retry_in", backoff))
		t := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			t.Stop()
			return nil
		case <-t.C:
		}
		if !authed {
			backoff = min(2*backoff, c.opts.MaxBackoff)
		}
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// session is one authenticated connection.
type session struct {
	ws      *websocket.Conn
	writeMu sync.Mutex
	nextID  int64
	mu      sync.Mutex
	pending map[int64]chan Message
	subs    map[int64]bool
	done    chan struct{}
	err     error
}

// serve runs one connection; authed reports whether it authenticated.
func (c *Client) serve(ctx context.Context, h Handler) (authed bool, err error) {
	dctx, cancel := context.WithTimeout(ctx, c.opts.CallTimeout)
	ws, _, err := websocket.Dial(dctx, c.opts.URL, nil) //nolint:bodyclose // the response body is the connection's
	cancel()
	if err != nil {
		return false, fmt.Errorf("ha: dial: %w", err)
	}
	ws.SetReadLimit(c.opts.ReadLimit)
	defer func() { _ = ws.CloseNow() }()

	version, err := c.auth(ctx, ws)
	if err != nil {
		_ = ws.Close(websocket.StatusPolicyViolation, "auth")
		return false, err
	}
	s := &session{ws: ws, pending: map[int64]chan Message{}, subs: map[int64]bool{}, done: make(chan struct{})}
	rctx, stop := context.WithCancel(ctx)
	defer stop()
	go c.read(rctx, s, h)

	for _, et := range c.opts.Events {
		if err := c.subscribe(ctx, s, et); err != nil {
			s.fail(err)
			return true, err
		}
	}
	c.mu.Lock()
	c.cur = s
	c.mu.Unlock()
	c.conn.Store(true)
	c.log.Info("ha.connected", slog.String("ha_version", version), slog.String("url", c.opts.URL))
	h.Connected(version)
	defer func() {
		c.conn.Store(false)
		c.mu.Lock()
		if c.cur == s {
			c.cur = nil
		}
		c.mu.Unlock()
	}()

	ping := time.NewTicker(c.opts.PingInterval)
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = ws.Close(websocket.StatusNormalClosure, "")
			return true, ctx.Err()
		case <-s.done:
			return true, s.err
		case <-ping.C:
			pctx, cancel := context.WithTimeout(ctx, c.opts.CallTimeout)
			_, err := s.call(pctx, map[string]any{"type": TypePing}, false)
			cancel()
			if err != nil {
				s.fail(fmt.Errorf("ha: ping: %w", err))
				return true, s.err
			}
		}
	}
}

// auth runs the handshake: auth_required, auth, auth_ok or auth_invalid
// (websocket_api/auth.py:25-46, 86-127 at 2026.10.3).
func (c *Client) auth(ctx context.Context, ws *websocket.Conn) (string, error) {
	actx, cancel := context.WithTimeout(ctx, c.opts.CallTimeout)
	defer cancel()
	first, err := readOne(actx, ws)
	if err != nil {
		return "", err
	}
	if first.Type != TypeAuthRequired {
		return "", fmt.Errorf("ha: expected auth_required, got %q", first.Type)
	}
	b, err := json.Marshal(map[string]string{"type": TypeAuth, "access_token": c.opts.Token})
	if err != nil {
		return "", err
	}
	if err := ws.Write(actx, websocket.MessageText, b); err != nil {
		return "", fmt.Errorf("ha: auth: %w", err)
	}
	reply, err := readOne(actx, ws)
	if err != nil {
		return "", err
	}
	switch reply.Type {
	case TypeAuthOK:
		return reply.HAVersion, nil
	case TypeAuthInvalid:
		return "", fmt.Errorf("%w: %s", ErrAuthInvalid, reply.Message)
	default:
		return "", fmt.Errorf("ha: expected auth_ok, got %q", reply.Type)
	}
}

func readOne(ctx context.Context, ws *websocket.Conn) (Message, error) {
	_, b, err := ws.Read(ctx)
	if err != nil {
		return Message{}, fmt.Errorf("ha: read: %w", err)
	}
	msgs, err := Decode(b)
	if err != nil {
		return Message{}, err
	}
	return msgs[0], nil
}

// read is the connection's one reader: results go to their caller, events
// to the handler.
func (c *Client) read(ctx context.Context, s *session, h Handler) {
	for {
		_, b, err := s.ws.Read(ctx)
		if err != nil {
			s.fail(fmt.Errorf("ha: read: %w", err))
			return
		}
		msgs, err := Decode(b)
		if err != nil {
			c.log.Warn("ha.frame_refused", slog.String("error", err.Error()), slog.Int("bytes", len(b)))
			continue
		}
		for _, m := range msgs {
			switch m.Type {
			case TypeEvent:
				s.mu.Lock()
				ours := s.subs[m.ID]
				s.mu.Unlock()
				if !ours {
					continue
				}
				var ev Event
				if err := json.Unmarshal(m.Event, &ev); err != nil {
					c.log.Warn("ha.event_refused", slog.String("error", err.Error()))
					continue
				}
				h.Event(ev)
			default:
				s.mu.Lock()
				ch := s.pending[m.ID]
				delete(s.pending, m.ID)
				s.mu.Unlock()
				if ch != nil {
					ch <- m
				}
			}
		}
	}
}

// fail ends the session once and fails every pending command.
func (s *session) fail(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case <-s.done:
		return
	default:
	}
	s.err = err
	close(s.done)
	for id, ch := range s.pending {
		delete(s.pending, id)
		close(ch)
	}
	_ = s.ws.CloseNow()
}

// call sends one command with the next id and waits for its answer. Ids
// increase per connection, as Home Assistant requires; the write lock
// keeps the send order the id order. A subscription's id is registered
// before the command is sent, so no event that follows its result is
// dropped.
func (s *session) call(ctx context.Context, cmd map[string]any, sub bool) (Message, error) {
	ch := make(chan Message, 1)
	s.writeMu.Lock()
	s.nextID++
	id := s.nextID
	cmd["id"] = id
	s.mu.Lock()
	select {
	case <-s.done:
		s.mu.Unlock()
		s.writeMu.Unlock()
		return Message{}, ErrDisconnected
	default:
	}
	s.pending[id] = ch
	if sub {
		s.subs[id] = true
	}
	s.mu.Unlock()
	b, err := json.Marshal(cmd)
	if err == nil {
		err = s.ws.Write(ctx, websocket.MessageText, b)
	}
	s.writeMu.Unlock()
	if err != nil {
		s.mu.Lock()
		delete(s.pending, id)
		s.mu.Unlock()
		return Message{}, fmt.Errorf("ha: write: %w", err)
	}
	select {
	case m, ok := <-ch:
		if !ok {
			return Message{}, ErrDisconnected
		}
		if m.Type == TypePong {
			return m, nil
		}
		if m.Success != nil && !*m.Success {
			if m.Error != nil {
				return m, m.Error
			}
			return m, errors.New("ha: command failed")
		}
		return m, nil
	case <-ctx.Done():
		s.mu.Lock()
		delete(s.pending, id)
		s.mu.Unlock()
		return Message{}, ctx.Err()
	}
}

// subscribe is subscribe_events for one event type
// (websocket_api/commands.py:180-219 at 2026.10.3; an event outside
// SUBSCRIBE_ALLOWLIST needs an administrator, :194).
func (c *Client) subscribe(ctx context.Context, s *session, eventType string) error {
	sctx, cancel := context.WithTimeout(ctx, c.opts.CallTimeout)
	defer cancel()
	if _, err := s.call(sctx, map[string]any{"type": "subscribe_events", "event_type": eventType}, true); err != nil {
		return fmt.Errorf("ha: subscribe_events %s: %w", eventType, err)
	}
	return nil
}

// Call sends one command (its "type" and fields; the id is the client's)
// on the current connection and decodes its result into out (nil
// discards it).
func (c *Client) Call(ctx context.Context, cmd map[string]any, out any) error {
	c.mu.Lock()
	s := c.cur
	c.mu.Unlock()
	if s == nil {
		return ErrNotConnected
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.opts.CallTimeout)
		defer cancel()
	}
	m, err := s.call(ctx, cmd, false)
	if err != nil {
		return err
	}
	if out == nil || len(m.Result) == 0 {
		return nil
	}
	if err := json.Unmarshal(m.Result, out); err != nil {
		return fmt.Errorf("ha: %v result: %w", cmd["type"], err)
	}
	return nil
}

// ListEntities is config/entity_registry/list
// (homeassistant/components/config/entity_registry.py:39-64 at 2026.10.3).
func (c *Client) ListEntities(ctx context.Context) ([]EntityEntry, error) {
	var out []EntityEntry
	err := c.Call(ctx, map[string]any{"type": "config/entity_registry/list"}, &out)
	return out, err
}

// ListDevices is config/device_registry/list
// (homeassistant/components/config/device_registry.py:75-101 at 2026.10.3).
func (c *Client) ListDevices(ctx context.Context) ([]DeviceEntry, error) {
	var out []DeviceEntry
	err := c.Call(ctx, map[string]any{"type": "config/device_registry/list"}, &out)
	return out, err
}

// ListAreas is config/area_registry/list
// (homeassistant/components/config/area_registry.py:23-37 at 2026.10.3).
func (c *Client) ListAreas(ctx context.Context) ([]Area, error) {
	var out []Area
	err := c.Call(ctx, map[string]any{"type": "config/area_registry/list"}, &out)
	return out, err
}

// ListLabels is config/label_registry/list, which needs no
// administrator (homeassistant/components/config/label_registry.py:53-67
// at 2026.10.3).
func (c *Client) ListLabels(ctx context.Context) ([]Label, error) {
	var out []Label
	err := c.Call(ctx, map[string]any{"type": "config/label_registry/list"}, &out)
	return out, err
}

// GetStates is get_states: every state the user may read
// (websocket_api/commands.py:369-385 at 2026.10.3).
func (c *Client) GetStates(ctx context.Context) ([]State, error) {
	var out []State
	err := c.Call(ctx, map[string]any{"type": "get_states"}, &out)
	return out, err
}

// GetConfig is get_config (websocket_api/commands.py:658-666 at
// 2026.10.3).
func (c *Client) GetConfig(ctx context.Context) (Config, error) {
	var out Config
	err := c.Call(ctx, map[string]any{"type": "get_config"}, &out)
	return out, err
}

// ListExposed is homeassistant/expose_entity/list, an administrator's
// command (homeassistant/components/homeassistant/exposed_entities.py:
// 414-442 at 2026.10.3).
func (c *Client) ListExposed(ctx context.Context) (Exposed, error) {
	var out Exposed
	err := c.Call(ctx, map[string]any{"type": "homeassistant/expose_entity/list"}, &out)
	return out, err
}

// CallService is call_service with target.entity_id; Home Assistant runs
// the service blocking and answers when it has run
// (websocket_api/commands.py:266-295 at 2026.10.3).
func (c *Client) CallService(ctx context.Context, sc ServiceCall) error {
	cmd := map[string]any{"type": "call_service", "domain": sc.Domain, "service": sc.Service}
	if sc.EntityID != "" {
		cmd["target"] = map[string]any{"entity_id": sc.EntityID}
	}
	if len(sc.Data) > 0 {
		cmd["service_data"] = sc.Data
	}
	return c.Call(ctx, cmd, nil)
}
