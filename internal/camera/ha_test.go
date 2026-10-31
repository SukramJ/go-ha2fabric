// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package camera_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/SukramJ/go-ha2fabric/internal/camera"
	"github.com/SukramJ/go-ha2fabric/internal/ha"
	"github.com/SukramJ/go-ha2fabric/internal/ha/hatest"
)

type connected chan struct{}

func (c connected) Connected(string) {
	select {
	case c <- struct{}{}:
	default:
	}
}
func (connected) Event(ha.Event)     {}
func (connected) Disconnected(error) {}

// TestHAUpstream runs the adapter on the client against the fake.
func TestHAUpstream(t *testing.T) {
	srv := hatest.New(t)
	c := ha.New(ha.Options{URL: srv.URL(), Token: hatest.Token, Logger: slog.New(slog.DiscardHandler)})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	up := connected(make(chan struct{}, 1))
	go func() { _ = c.Run(ctx, up) }()
	<-up
	u := camera.HA(c)
	s, err := u.Subscribe(ctx, entity, "v=0")
	if err != nil {
		t.Fatal(err)
	}
	<-s.Events()
	if err := u.Candidate(ctx, entity, "01JBSESSION1", ha.ICECandidate{Candidate: "candidate:x"}); err != nil {
		t.Fatal(err)
	}
	srv.SetImage(entity, jpegOf(t, 8, 8))
	if _, ct, err := u.Image(ctx, entity, 0, 0); err != nil || ct != "image/jpeg" {
		t.Fatal(ct, err)
	}
	if err := s.Unsubscribe(ctx); err != nil || len(srv.Unsubscribed()) != 1 {
		t.Fatal(err)
	}
}
