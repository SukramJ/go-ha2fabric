// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package ha

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Errors of a [Subscription].
var (
	// ErrSubscriptionOverflow ends a subscription whose reader fell more
	// than SubscriptionBuffer events behind: the client's one reader does
	// not wait for it.
	ErrSubscriptionOverflow = errors.New("ha: subscription fell behind")
)

// SubscriptionBuffer is the number of events a [Subscription] holds for
// its reader.
const SubscriptionBuffer = 64

// Subscription is a command that answers its result and then pushes
// events on the same id, such as camera/webrtc/offer (camera/webrtc.py:
// 276-292 at 2026.10.3: the result, then event_message(msg["id"], ...)).
// Its events end when it is unsubscribed or the connection drops; a
// reconnect does not renew it.
type Subscription struct {
	id      int64
	s       *session
	timeout time.Duration

	mu     sync.Mutex
	ch     chan json.RawMessage
	closed bool
	err    error
}

// ID is the command's id, the one Home Assistant's unsubscribe_events
// names.
func (sub *Subscription) ID() int64 { return sub.id }

// Events yields the inner "event" object of each event of the
// subscription (websocket_api/messages.py:107-109 at 2026.10.3). It is
// closed when the subscription ends; [Subscription.Err] then says why.
func (sub *Subscription) Events() <-chan json.RawMessage { return sub.ch }

// Err is nil while the subscription runs and after [Subscription.Unsubscribe];
// otherwise the reason it ended: ErrDisconnected (wrapped) for a dropped
// connection, ErrSubscriptionOverflow for a reader that fell behind.
func (sub *Subscription) Err() error {
	sub.mu.Lock()
	defer sub.mu.Unlock()
	return sub.err
}

// deliver hands one event to the reader without blocking; false when the
// buffer is full.
func (sub *Subscription) deliver(ev json.RawMessage) bool {
	sub.mu.Lock()
	defer sub.mu.Unlock()
	if sub.closed {
		return true
	}
	select {
	case sub.ch <- ev:
		return true
	default:
		return false
	}
}

// close ends the subscription once with err.
func (sub *Subscription) close(err error) {
	sub.mu.Lock()
	defer sub.mu.Unlock()
	if sub.closed {
		return
	}
	sub.closed = true
	sub.err = err
	close(sub.ch)
}

// dropStream removes the subscription of id and ends it with err.
func (s *session) dropStream(id int64, err error) {
	s.mu.Lock()
	sub := s.streams[id]
	delete(s.streams, id)
	s.mu.Unlock()
	if sub != nil {
		sub.close(err)
	}
}

// Subscribe sends one command whose events follow its result on the
// command's id, and returns once the result arrived. A failed result is
// its error and no subscription. payload holds the command's fields
// besides "type" and "id".
func (c *Client) Subscribe(ctx context.Context, msgType string, payload map[string]any) (*Subscription, error) {
	c.mu.Lock()
	s := c.cur
	c.mu.Unlock()
	if s == nil {
		return nil, ErrNotConnected
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.opts.CallTimeout)
		defer cancel()
	}
	cmd := make(map[string]any, len(payload)+1)
	for k, v := range payload {
		cmd[k] = v
	}
	cmd["type"] = msgType
	sub := &Subscription{s: s, timeout: c.opts.CallTimeout, ch: make(chan json.RawMessage, SubscriptionBuffer)}
	if _, err := s.callWith(ctx, cmd, false, sub); err != nil {
		s.dropStream(sub.id, err)
		return nil, fmt.Errorf("ha: %s: %w", msgType, err)
	}
	return sub, nil
}

// Unsubscribe ends the subscription: its channel is closed and Home
// Assistant is sent unsubscribe_events with the subscription's id, which
// runs the command's teardown — for camera/webrtc/offer the provider's
// session close (websocket_api/commands.py:246-263, camera/webrtc.py:
// 276-278, camera/__init__.py:730-733 at 2026.10.3). On a connection that
// already dropped there is nothing to send: Home Assistant ran every
// teardown when it closed (websocket_api/connection.py:258-268).
func (sub *Subscription) Unsubscribe(ctx context.Context) error {
	sub.s.dropStream(sub.id, nil)
	select {
	case <-sub.s.done:
		return nil
	default:
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, sub.timeout)
		defer cancel()
	}
	_, err := sub.s.call(ctx, map[string]any{"type": "unsubscribe_events", "subscription": sub.id}, false)
	if errors.Is(err, ErrDisconnected) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("ha: unsubscribe_events %d: %w", sub.id, err)
	}
	return nil
}
