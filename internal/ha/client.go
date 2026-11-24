// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

// Package ha is the daemon's Home Assistant client: the WebSocket API
// (https://developers.home-assistant.io/docs/api/websocket) with its auth
// handshake, the id-tagged command envelope, ping keepalive, event
// subscriptions, commands whose events follow their result
// (camera/webrtc/offer), the camera proxy's still over HTTP, and a
// reconnect with back-off. Every payload shape is
// taken from Home Assistant core at the pinned release 2026.10.3; the
// citations are file:line at that tag.
//
// The access token (a long-lived token stand-alone, SUPERVISOR_TOKEN in
// the add-on) is held in memory only: it is sent in the auth message and
// as the camera proxy's Bearer header, and never logged.
package ha

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
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
	// ErrReadLimit is a message of Home Assistant over the read limit
	// (Options.ReadLimit): the connection ends with it, and a command
	// whose result it was fails with it beside ErrDisconnected.
	ErrReadLimit = errors.New("ha: a message from Home Assistant exceeds the read limit")
)

// Defaults of [Options].
const (
	DefaultPingInterval = 30 * time.Second
	DefaultCallTimeout  = 30 * time.Second
	DefaultMinBackoff   = time.Second
	DefaultMaxBackoff   = time.Minute
	// DefaultAuthBackoff and DefaultAuthMaxBackoff bound the delay after
	// a refused token: minutes, then hourly.
	DefaultAuthBackoff    = 5 * time.Minute
	DefaultAuthMaxBackoff = time.Hour
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
	// AuthBackoff and AuthMaxBackoff bound the delay after Home
	// Assistant refused the token (auth_invalid), which doubles per
	// refusal. 0 is DefaultAuthBackoff and DefaultAuthMaxBackoff.
	AuthBackoff, AuthMaxBackoff time.Duration
	// ReadLimit bounds one frame; 0 is DefaultReadLimit.
	ReadLimit int64
	// ImageLimit bounds a camera still ([Client.Image]); 0 is
	// DefaultImageLimit.
	ImageLimit int64
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

	http *http.Client

	mu   sync.Mutex
	cur  *session
	conn atomic.Bool

	// refused is the time of the first auth_invalid in a row, zero
	// while the token is accepted.
	refusedMu sync.Mutex
	refused   time.Time

	// skipped holds the list commands whose skipped entry was logged
	// (listOf).
	skippedMu sync.Mutex
	skipped   map[string]bool
	// oversized holds the entities whose state over MaxStateBytes was
	// logged (GetStates); guarded by skippedMu.
	oversized map[string]bool
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
	if opts.AuthBackoff <= 0 {
		opts.AuthBackoff = DefaultAuthBackoff
	}
	if opts.AuthMaxBackoff <= 0 {
		opts.AuthMaxBackoff = DefaultAuthMaxBackoff
	}
	if opts.AuthMaxBackoff < opts.AuthBackoff {
		opts.AuthMaxBackoff = opts.AuthBackoff
	}
	if opts.ReadLimit <= 0 {
		opts.ReadLimit = DefaultReadLimit
	}
	if opts.ImageLimit <= 0 {
		opts.ImageLimit = DefaultImageLimit
	}
	log := opts.Logger
	if log == nil {
		log = slog.Default()
	}
	// The REST client is built once, bounded by CallTimeout, and follows
	// no redirect (refuseRESTRedirect). The WebSocket dial takes
	// noRedirect.
	return &Client{opts: opts, log: log, http: &http.Client{Timeout: opts.CallTimeout, CheckRedirect: refuseRESTRedirect}}
}

// errRESTRedirect refuses a redirect of a REST request (the camera
// proxy's still), which carries the token as its Bearer header.
var errRESTRedirect = errors.New("ha: Home Assistant answered with a redirect, which is not followed; set HA_URL to the final address")

// refuseRESTRedirect is the REST client's redirect policy: none followed,
// so the Bearer header goes to the configured endpoint only.
func refuseRESTRedirect(*http.Request, []*http.Request) error { return errRESTRedirect }

// errRedirect refuses a redirect of the WebSocket handshake.
var errRedirect = errors.New("ha: the WebSocket endpoint answered with a redirect, which is not followed; set HA_URL to the final address")

// noRedirect is the WebSocket dial's HTTP client: the default transport,
// and no redirect followed — the token goes in the first message of the
// connection, so it would go to wherever a redirect chain ends.
var noRedirect = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return errRedirect }}

// TokenRefused reports whether Home Assistant's last answer to the
// token was auth_invalid, and since when it refuses it.
func (c *Client) TokenRefused() (since time.Time, refused bool) {
	c.refusedMu.Lock()
	defer c.refusedMu.Unlock()
	return c.refused, !c.refused.IsZero()
}

// setRefused records an auth_invalid (true) or an authentication (false).
func (c *Client) setRefused(refused bool) {
	c.refusedMu.Lock()
	defer c.refusedMu.Unlock()
	switch {
	case !refused:
		c.refused = time.Time{}
	case c.refused.IsZero():
		c.refused = time.Now()
	}
}

// Connected reports whether a connection is authenticated now.
func (c *Client) Connected() bool { return c.conn.Load() }

// Run connects, authenticates, subscribes and serves the connection until
// ctx ends; a dropped connection or a failed attempt is retried after a
// back-off that doubles up to MaxBackoff and starts again at MinBackoff
// only after a connection that stayed up longer than MaxBackoff, so one
// that authenticates and drops at once is not retried at MinBackoff for
// ever. A refused token (auth_invalid) waits AuthBackoff, doubling to
// AuthMaxBackoff, and is reported by [Client.TokenRefused] until an
// authentication. Run never gives up: the token of the add-on can become
// valid again. It returns nil when ctx ends.
func (c *Client) Run(ctx context.Context, h Handler) error {
	backoff := c.opts.MinBackoff
	authBackoff := c.opts.AuthBackoff
	for {
		up, authed, err := c.serve(ctx, h)
		if ctx.Err() != nil {
			return nil //nolint:nilerr // the end of ctx is the end of Run, not a failure
		}
		h.Disconnected(err)
		if authed {
			authBackoff = c.opts.AuthBackoff
		}
		if up > c.opts.MaxBackoff {
			backoff = c.opts.MinBackoff
		}
		delay := backoff
		if errors.Is(err, ErrAuthInvalid) {
			c.setRefused(true)
			delay = authBackoff
			authBackoff = min(2*authBackoff, c.opts.AuthMaxBackoff)
			c.log.Warn("ha.token_refused", slog.Duration("retry_in", delay),
				slog.String("hint", "Home Assistant refused the access token (auth_invalid); stand-alone, set a valid long-lived token in HA_TOKEN and restart"))
		} else {
			c.log.Warn("ha.disconnected", slog.String("error", errString(err)), slog.Duration("retry_in", delay))
			backoff = min(2*backoff, c.opts.MaxBackoff)
		}
		t := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			t.Stop()
			return nil
		case <-t.C:
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
	streams map[int64]*Subscription
	done    chan struct{}
	err     error
	// timeout bounds a command's write (CallTimeout).
	timeout time.Duration
}

// serve runs one connection; authed reports whether it authenticated, up
// how long it was authenticated.
func (c *Client) serve(ctx context.Context, h Handler) (up time.Duration, authed bool, err error) {
	dctx, cancel := context.WithTimeout(ctx, c.opts.CallTimeout)
	ws, _, err := websocket.Dial(dctx, c.opts.URL, &websocket.DialOptions{HTTPClient: noRedirect}) //nolint:bodyclose // the response body is the connection's
	cancel()
	if err != nil {
		return 0, false, fmt.Errorf("ha: dial: %w", scrubURL(err))
	}
	ws.SetReadLimit(c.opts.ReadLimit)
	defer func() { _ = ws.CloseNow() }()

	version, err := c.auth(ctx, ws)
	if err != nil {
		_ = ws.Close(websocket.StatusPolicyViolation, "auth")
		return 0, false, err
	}
	// The refusal ends with this authentication, not with the end of the
	// connection it opens (HR-P1-ha-92).
	c.setRefused(false)
	began := time.Now()
	defer func() { up = time.Since(began) }()
	s := &session{ws: ws, pending: map[int64]chan Message{}, subs: map[int64]bool{}, streams: map[int64]*Subscription{}, done: make(chan struct{}), timeout: c.opts.CallTimeout}
	rctx, stop := context.WithCancel(ctx)
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		c.read(rctx, s, h)
	}()
	// The reader is joined before serve returns, so no Event follows the
	// Disconnected that Run sends next (the Handler's order).
	defer func() {
		stop()
		_ = ws.CloseNow()
		<-readDone
	}()

	for _, et := range c.opts.Events {
		if err := c.subscribe(ctx, s, et); err != nil {
			s.fail(err)
			return 0, true, err
		}
	}
	c.mu.Lock()
	c.cur = s
	c.mu.Unlock()
	c.conn.Store(true)
	c.log.Info("ha.connected", slog.String("ha_version", version), slog.String("url", LogURL(c.opts.URL)))
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
			return 0, true, ctx.Err()
		case <-s.done:
			return 0, true, s.err
		case <-ping.C:
			pctx, cancel := context.WithTimeout(ctx, c.opts.CallTimeout)
			_, err := s.call(pctx, map[string]any{"type": TypePing}, false)
			cancel()
			if err != nil {
				s.fail(fmt.Errorf("ha: ping: %w", err))
				return 0, true, s.err
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
			if errors.Is(err, websocket.ErrMessageTooBig) {
				// Named, so the sync's error says why (HR-P2-bound-37).
				err = fmt.Errorf("%w of %d bytes: %w", ErrReadLimit, c.opts.ReadLimit, err)
			}
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
				stream := s.streams[m.ID]
				s.mu.Unlock()
				if stream != nil {
					if !stream.deliver(m.Event) {
						c.log.Warn("ha.subscription_overflow", slog.Int64("id", m.ID))
						s.dropStream(m.ID, ErrSubscriptionOverflow)
					}
					continue
				}
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

// dropped is the error of a command whose connection ended with cause:
// ErrDisconnected, carrying the cause when there is one (a message over
// the read limit, say).
func dropped(cause error) error {
	if cause == nil {
		return ErrDisconnected
	}
	return fmt.Errorf("%w: %w", ErrDisconnected, cause)
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
	for id, sub := range s.streams {
		delete(s.streams, id)
		sub.close(fmt.Errorf("%w: %w", ErrDisconnected, err))
	}
	_ = s.ws.CloseNow()
}

// call sends one command with the next id and waits for its answer. Ids
// increase per connection, as Home Assistant requires; the write lock
// keeps the send order the id order. A subscription's id is registered
// before the command is sent, so no event that follows its result is
// dropped.
func (s *session) call(ctx context.Context, cmd map[string]any, sub bool) (Message, error) {
	return s.callWith(ctx, cmd, sub, nil)
}

// callWith is call that registers stream, when not nil, as the receiver
// of the events of the command's id — before the command is sent.
//
// ctx bounds the wait for the result, not the write: coder/websocket
// closes the connection when a write's context ends mid-frame, and the
// connection is every node's. The write runs on ctx's values bounded by
// the session's timeout; a ctx that has ended sends nothing.
//
// A stream whose ctx ends after the command was written may still be
// taken by Home Assistant, which registers the subscription's teardown
// before it sends the result (camera/webrtc.py:275-280 at 2026.10.3) and
// runs it only on unsubscribe_events or when the connection closes
// (websocket_api/connection.py:258-268): the late result is awaited and
// a successful one unsubscribed (unsubscribeLate).
func (s *session) callWith(ctx context.Context, cmd map[string]any, sub bool, stream *Subscription) (Message, error) {
	if err := ctx.Err(); err != nil {
		return Message{}, err
	}
	ch := make(chan Message, 1)
	s.writeMu.Lock()
	s.nextID++
	id := s.nextID
	cmd["id"] = id
	s.mu.Lock()
	select {
	case <-s.done:
		err := s.err
		s.mu.Unlock()
		s.writeMu.Unlock()
		return Message{}, dropped(err)
	default:
	}
	s.pending[id] = ch
	if sub {
		s.subs[id] = true
	}
	if stream != nil {
		stream.id = id
		s.streams[id] = stream
	}
	s.mu.Unlock()
	b, err := json.Marshal(cmd)
	if err == nil {
		wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.timeout)
		err = s.ws.Write(wctx, websocket.MessageText, b)
		cancel()
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
			s.mu.Lock()
			err := s.err
			s.mu.Unlock()
			return Message{}, dropped(err)
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
		if stream != nil {
			go s.unsubscribeLate(context.WithoutCancel(ctx), id, ch)
			return Message{}, ctx.Err()
		}
		s.mu.Lock()
		delete(s.pending, id)
		s.mu.Unlock()
		return Message{}, ctx.Err()
	}
}

// unsubscribeLate waits for the result of a subscription whose caller
// gave up and, when Home Assistant took the command, sends
// unsubscribe_events for it, so its teardown runs (for camera/webrtc/offer
// the provider's session close). A refusal holds nothing; a dropped
// connection closes ch and ran every teardown already. Best effort, on
// ctx without the caller's end.
func (s *session) unsubscribeLate(ctx context.Context, id int64, ch chan Message) {
	m, ok := <-ch
	if !ok || m.Success != nil && !*m.Success {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	_, _ = s.call(ctx, map[string]any{"type": "unsubscribe_events", "subscription": id}, false)
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
// Entries that do not decode come back as a [*SkippedError] with their
// entity ids, beside the entries that did; one whose entity id cannot be
// read fails the list.
func (c *Client) ListEntities(ctx context.Context) ([]EntityEntry, error) {
	return listOf[EntityEntry](ctx, c, "config/entity_registry/list", idOf("entity_id", true))
}

// ListDevices is config/device_registry/list
// (homeassistant/components/config/device_registry.py:75-101 at 2026.10.3).
// Entries that do not decode come back as a [*SkippedError] with their
// ids, beside the entries that did; one whose id cannot be read fails the
// list.
func (c *Client) ListDevices(ctx context.Context) ([]DeviceEntry, error) {
	return listOf[DeviceEntry](ctx, c, "config/device_registry/list", idOf("id", true))
}

// ListAreas is config/area_registry/list
// (homeassistant/components/config/area_registry.py:23-37 at 2026.10.3).
// An entry that does not decode is left out (an area is only a device's
// location name).
func (c *Client) ListAreas(ctx context.Context) ([]Area, error) {
	return listOf[Area](ctx, c, "config/area_registry/list", nil)
}

// ListLabels is config/label_registry/list, which needs no
// administrator (homeassistant/components/config/label_registry.py:53-67
// at 2026.10.3). Entries that do not decode come back as a
// [*SkippedError] with their label ids where readable, beside the entries
// that did.
func (c *Client) ListLabels(ctx context.Context) ([]Label, error) {
	return listOf[Label](ctx, c, "config/label_registry/list", idOf("label_id", false))
}

// GetStates is get_states: every state the user may read
// (websocket_api/commands.py:369-385 at 2026.10.3). A state that does not
// decode is left out: its entity has no state for the sync and stays out
// until its next state_changed. A state over MaxStateBytes comes without
// its attributes (State.Oversized) and is logged with its entity id, at
// Warn the first time for each entity (HR-P2-bound-38).
func (c *Client) GetStates(ctx context.Context) ([]State, error) {
	states, err := listOf[State](ctx, c, "get_states", nil)
	for i := range states {
		st := &states[i]
		if st.Oversized == 0 {
			continue
		}
		c.skippedMu.Lock()
		logged := c.oversized[st.EntityID]
		if c.oversized == nil {
			c.oversized = map[string]bool{}
		}
		c.oversized[st.EntityID] = true
		c.skippedMu.Unlock()
		level := slog.LevelWarn
		if logged {
			level = slog.LevelDebug
		}
		c.log.Log(ctx, level, "ha.state_oversized", slog.String("entity_id", st.EntityID), slog.Int("bytes", st.Oversized),
			slog.Int("max", MaxStateBytes), slog.String("hint", "the entity's attributes were dropped"))
	}
	return states, err
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

// SkippedError is a registry list some of whose entries did not decode;
// the list method returns it beside the entries that did. A sync judges
// the entities it concerns rather than reading them without their entry
// (HR-P1-data-90).
type SkippedError struct {
	// Command is the list command.
	Command string
	// IDs are the skipped entries' ids; a label's is empty when it could
	// not be read either.
	IDs []string
	// Err is the first entry's decoding error.
	Err error
}

func (e *SkippedError) Error() string {
	return fmt.Sprintf("%s: %d entries did not decode: %v", e.Command, len(e.IDs), e.Err)
}

func (e *SkippedError) Unwrap() error { return e.Err }

// idOf reads an entry's id from field for a skipped entry; strict fails
// the list when it cannot be read, else the id is empty.
func idOf(field string, strict bool) func(json.RawMessage) (string, error) {
	return func(r json.RawMessage) (string, error) {
		var m map[string]json.RawMessage
		var id string
		err := json.Unmarshal(r, &m)
		if err == nil {
			err = json.Unmarshal(m[field], &id)
		}
		if (err != nil || id == "") && strict {
			return "", fmt.Errorf("an entry without a readable %s", field)
		}
		return id, nil
	}
}

// listOf is a list command whose entries decode one by one: an entry
// that does not decode (a registry's free-text field that an integration
// set to a number, say — the registries validate few field types, see
// DeviceEntry's Tuple) is logged once per command rather than failing the
// list and with it every sync. Without id it is left out; with id, its id
// is read and returned in a [*SkippedError] beside the entries that
// decoded, and an id that cannot be read fails the list.
func listOf[T any](ctx context.Context, c *Client, msgType string, id func(json.RawMessage) (string, error)) ([]T, error) {
	var raw []json.RawMessage
	if err := c.Call(ctx, map[string]any{"type": msgType}, &raw); err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}
	out := make([]T, 0, len(raw))
	skipped := 0
	var first error
	var ids []string
	for _, r := range raw {
		var v T
		if err := json.Unmarshal(r, &v); err != nil {
			skipped++
			if first == nil {
				first = err
			}
			if id != nil {
				sid, ierr := id(r)
				if ierr != nil {
					return nil, fmt.Errorf("%w (%w)", ierr, err)
				}
				ids = append(ids, sid)
			}
			continue
		}
		out = append(out, v)
	}
	if skipped > 0 {
		c.skippedMu.Lock()
		logged := c.skipped[msgType]
		if c.skipped == nil {
			c.skipped = map[string]bool{}
		}
		c.skipped[msgType] = true
		c.skippedMu.Unlock()
		level := slog.LevelWarn
		if logged {
			level = slog.LevelDebug
		}
		c.log.Log(ctx, level, "ha.entry_skipped", slog.String("command", msgType), slog.Int("skipped", skipped),
			slog.Int("of", len(raw)), slog.String("error", first.Error()))
	}
	if ids != nil {
		return out, &SkippedError{Command: msgType, IDs: ids, Err: first}
	}
	return out, nil
}

// LogURL is raw as the log shows it: the scheme, the host and the path,
// without a user, a query or a fragment, which may carry a proxy's
// credential. A URL that does not parse is not shown.
func LogURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "(unparsable)"
	}
	v := url.URL{Scheme: u.Scheme, Host: u.Host, Path: u.Path}
	return v.String()
}

// scrubURL replaces the URL a net/http error names with its [LogURL], in
// the error and in the messages wrapped around it.
func scrubURL(err error) error {
	ue, ok := errors.AsType[*url.Error](err)
	if !ok {
		return err
	}
	raw := ue.URL
	if raw == "" {
		return err
	}
	ue.URL = LogURL(raw)
	return scrubbed{msg: strings.ReplaceAll(err.Error(), raw, ue.URL), err: err}
}

// scrubbed is an error with its message rewritten; it unwraps to the
// original.
type scrubbed struct {
	msg string
	err error
}

func (e scrubbed) Error() string { return e.msg }
func (e scrubbed) Unwrap() error { return e.err }
