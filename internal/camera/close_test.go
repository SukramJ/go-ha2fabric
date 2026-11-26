// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package camera_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	gfcamera "thingfabric.dev/go-fabric/cluster/camera"

	"thingfabric.dev/go-ha2fabric/internal/camera"
)

// goneController is a signaller whose controller went away: every
// signal waits until its context ends, as go-fabric's Provider.Invoke to
// a peer that no longer answers.
type goneController struct{}

func (goneController) Answer(ctx context.Context, _ uint16, _ string) error {
	<-ctx.Done()
	return ctx.Err()
}

func (goneController) IceCandidates(ctx context.Context, _ uint16, _ []gfcamera.ProviderICECandidate) error {
	<-ctx.Done()
	return ctx.Err()
}

func (goneController) End(ctx context.Context, _ uint16, _ uint8) error {
	<-ctx.Done()
	return ctx.Err()
}

// fullHost is a host holding MaxSessions sessions of a controller that
// went away.
func fullHost(t *testing.T, timeout time.Duration) *camera.Host {
	t.Helper()
	h := camera.New(entity, &upstream{}, camera.Options{
		Logger: slog.New(slog.DiscardHandler), SignalTimeout: timeout, RetryInterval: time.Millisecond, EvictIdle: time.Hour,
	})
	h.Attach(goneController{})
	for i := range camera.MaxSessions {
		if _, err := h.ProvideOffer(context.Background(), fill(i)); err != nil {
			t.Fatal(err)
		}
	}
	return h
}

// TestCloseHonoursItsDeadline (HR-P2-conc-60): Close of a host whose
// sessions' controller is gone returns by its ctx's deadline, not after
// one SignalTimeout per session in turn.
func TestCloseHonoursItsDeadline(t *testing.T) {
	h := fullHost(t, 2*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	began := time.Now()
	h.Close(ctx)
	if took := time.Since(began); took > time.Second {
		t.Fatalf("Close of %d sessions took %v with a ctx of 100ms", camera.MaxSessions, took)
	}
}

// TestCloseEndsSessionsInParallel (HR-P2-conc-60): with an ended ctx
// (the worker's at a stop) the signals still go out, together, so Close
// takes one SignalTimeout, not one per session.
func TestCloseEndsSessionsInParallel(t *testing.T) {
	const timeout = 300 * time.Millisecond
	h := fullHost(t, timeout)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	began := time.Now()
	h.Close(ctx)
	if took := time.Since(began); took > 3*timeout {
		t.Fatalf("Close of %d sessions took %v, SignalTimeout %v: the sessions end in turn", camera.MaxSessions, took, timeout)
	}
}
