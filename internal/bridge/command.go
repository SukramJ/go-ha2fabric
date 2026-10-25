// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package bridge

import (
	"context"
	"log/slog"
	"time"

	"github.com/SukramJ/go-hamqtt/model"

	"github.com/SukramJ/go-ha2fabric/internal/adapt"
	"github.com/SukramJ/go-ha2fabric/internal/ha"
)

// commandTimeout bounds one call_service; Home Assistant answers a
// service call when the service has run (websocket_api/commands.py:283-292
// at 2026.10.3, blocking=True).
const commandTimeout = 30 * time.Second

// onCommand is the plans' OnChange: it runs inline in the controller's
// request, so it only queues (go-hamapping plan.go OnChange).
func (b *Bridge) onCommand(c model.Command) {
	select {
	case b.cmds <- c:
	default:
		b.log.Error("bridge.command_dropped", slog.String("entity", c.Entity.Key()), slog.String("hint", "the command queue is full"))
		b.setStatus(func(s *Status) { s.CommandErrs++; s.LastCommand = "queue full" })
	}
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
	cur := b.states[id]
	b.mu.Unlock()
	sc, err := adapt.Service(c, cur)
	if err != nil {
		b.log.Warn("bridge.command_unmapped", slog.String("entity", id), slog.String("error", err.Error()))
		b.setStatus(func(s *Status) { s.CommandErrs++; s.LastCommand = err.Error() })
		return
	}
	cctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	err = b.up.CallService(cctx, sc)
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
		b.enqueue(func(ctx context.Context) {
			if g := b.owner[id]; g != nil {
				b.apply(ctx, g, id)
			}
		})
		return
	}
	b.log.Debug("bridge.call_service", slog.String("call", sc.String()))
}

var _ ha.Handler = (*Bridge)(nil)
