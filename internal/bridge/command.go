// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package bridge

import (
	"context"
	"log/slog"
	"maps"
	"time"

	"github.com/SukramJ/go-hamapping"
	"github.com/SukramJ/go-hamqtt/model"

	"github.com/SukramJ/go-ha2fabric/internal/adapt"
	"github.com/SukramJ/go-ha2fabric/internal/ha"
)

// commandTimeout bounds one call_service; Home Assistant answers a
// service call when the service has run (websocket_api/commands.py:283-292
// at 2026.10.3, blocking=True).
const commandTimeout = 30 * time.Second

// rangeHold bounds how long the range a call sent stands in for Home
// Assistant's state ([Bridge.ranges]): long enough for a thermostat that
// reports after a device round trip or a cloud poll, short enough that a
// report that never matches (a rounded or clamped bound) is not held for
// good.
const rangeHold = 30 * time.Second

// sentRange is the heat/cool range a set_temperature carried and when.
type sentRange struct {
	low, high float64
	at        time.Time
}

// onCommand is the plans' OnChange: it runs inline in the controller's
// request, so it only queues (go-hamapping plan.go OnChange).
func (b *Bridge) onCommand(c model.Command) {
	select {
	case b.cmds <- c:
	default:
		b.log.Error("bridge.command_dropped", slog.String("entity", c.Entity.Key()), slog.String("hint", "the command queue is full"))
		b.setStatus(func(s *Status) { s.CommandErrs++; s.LastCommand = "queue full" })
		b.restore(c.Entity.Key())
	}
}

// restore carries Home Assistant's state of id onto its device again: a
// command that did not reach Home Assistant, or failed there, left the
// controller's value on the device (HR-P2-err-63; go-hamapping's OnChange
// runs after the change). A full queue leaves a sync owed instead, which
// applies every state.
func (b *Bridge) restore(id string) {
	b.enqueue(func(ctx context.Context) {
		if g := b.owner[id]; g != nil {
			b.apply(ctx, g, id)
		}
	})
}

// onDiscard is the plans' OnDiscard: a controller's command the row
// answered with the cluster's success and the entity has no action for
// (an Off on a scene's plug, a Toggle of a mute). It is logged, so a
// controller's "done" that changed nothing can be traced (#51); nothing
// is called.
func (b *Bridge) onDiscard(d hamapping.Discard) {
	b.log.Info("bridge.command_discarded", slog.String("entity", d.Entity.Key()), slog.String("cluster", d.Cluster), slog.String("command", d.Command))
	b.setStatus(func(s *Status) { s.Discards++ })
}

// commandLoop turns each queued command into its call_service, in order.
func (b *Bridge) commandLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case c := <-b.cmds:
			b.command(ctx, c)
		}
	}
}

func (b *Bridge) command(ctx context.Context, c model.Command) {
	id := c.Entity.Key()
	b.mu.Lock()
	// A command waits in the queue; the sync before it runs may have
	// taken its entity off the bridge (HR-P1-conc-33).
	if !b.serving[id] {
		b.mu.Unlock()
		b.log.Warn("bridge.command_unserved", slog.String("entity", id), slog.String("hint", "the entity left the bridge while its command was queued"))
		b.setStatus(func(s *Status) { s.CommandErrs++; s.LastCommand = id + ": no longer exposed" })
		return
	}
	cur := b.states[id]
	if c.Method == hamapping.MethodTemperatureLow || c.Method == hamapping.MethodTemperatureHigh {
		cur = b.sentRangeLocked(id, cur)
	}
	b.mu.Unlock()
	sc, err := adapt.Service(c, cur)
	if err != nil {
		b.log.Warn("bridge.command_unmapped", slog.String("entity", id), slog.String("error", err.Error()))
		b.setStatus(func(s *Status) { s.CommandErrs++; s.LastCommand = err.Error() })
		b.restore(id)
		return
	}
	cctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	err = b.up.CallService(cctx, sc)
	if err != nil && ctx.Err() != nil {
		// The worker stops: no failure to count (HR-P2-err-67).
		b.log.Debug("bridge.call_service_cancelled", slog.String("call", sc.String()), slog.String("error", err.Error()))
		return
	}
	b.setStatus(func(s *Status) {
		s.Commands++
		if err != nil {
			s.CommandErrs++
			s.LastCommand = sc.String() + ": " + err.Error()
		}
	})
	if err != nil {
		b.log.Warn("bridge.call_service_failed", slog.String("call", sc.String()), slog.String("error", err.Error()))
		// The controller already saw its change applied; Home Assistant's
		// state is the truth, so it is carried onto the device again.
		b.restore(id)
		return
	}
	b.noteRange(id, sc)
	b.log.Debug("bridge.call_service", slog.String("call", sc.String()))
}

// sentRangeLocked is cur with the range the last set_temperature sent for
// id, while Home Assistant's state has not shown it and rangeHold has not
// passed (HR-P1-adpt-30). go-hamapping gives a controller's range write as
// two commands, one per bound; the second completes from the first's
// bound, not from a state Home Assistant has not reported yet. Called
// with mu held.
func (b *Bridge) sentRangeLocked(id string, cur *ha.State) *ha.State {
	r, ok := b.ranges[id]
	if !ok {
		return cur
	}
	if time.Since(r.at) > rangeHold {
		delete(b.ranges, id)
		return cur
	}
	out := &ha.State{EntityID: id}
	if cur != nil {
		*out = *cur
	}
	out.Attributes = maps.Clone(out.Attributes)
	if out.Attributes == nil {
		out.Attributes = map[string]any{}
	}
	out.Attributes["target_temp_low"], out.Attributes["target_temp_high"] = r.low, r.high
	return out
}

// noteRange keeps the range a successful set_temperature carried.
func (b *Bridge) noteRange(id string, sc ha.ServiceCall) {
	low, okL := sc.Data["target_temp_low"].(float64)
	high, okH := sc.Data["target_temp_high"].(float64)
	if sc.Service != "set_temperature" || !okL || !okH {
		return
	}
	b.mu.Lock()
	b.ranges[id] = sentRange{low: low, high: high, at: time.Now()}
	b.mu.Unlock()
}

// settleRangeLocked drops id's sent range once st shows it: from then on
// Home Assistant's state is the truth again. Called with mu held.
func (b *Bridge) settleRangeLocked(id string, st *ha.State) {
	r, ok := b.ranges[id]
	if !ok || st == nil {
		return
	}
	low, okL := toFloat(st.Attributes["target_temp_low"])
	high, okH := toFloat(st.Attributes["target_temp_high"])
	if okL && okH && low == r.low && high == r.high {
		delete(b.ranges, id)
	}
}

// toFloat is a JSON number of a state attribute.
func toFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case int:
		return float64(t), true
	}
	return 0, false
}

var _ ha.Handler = (*Bridge)(nil)
