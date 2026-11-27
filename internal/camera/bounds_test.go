// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package camera_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"thingfabric.dev/go-fabric/cluster/spec"
	csdef "thingfabric.dev/go-fabric/cluster/spec/cameraavstreammanagement"
	wpdef "thingfabric.dev/go-fabric/cluster/spec/webrtctransportprovider"
	wrdef "thingfabric.dev/go-fabric/cluster/spec/webrtctransportrequestor"
	"thingfabric.dev/go-fabric/contract"

	"thingfabric.dev/go-ha2fabric/internal/camera"
	"thingfabric.dev/go-ha2fabric/internal/ha"
)

// boundedHost is newHost with the idle bounds of opts.
func boundedHost(t *testing.T, up camera.Upstream, opts camera.Options) *camera.Host {
	t.Helper()
	opts.Logger = slog.New(slog.DiscardHandler)
	opts.RetryInterval = time.Millisecond
	h := camera.New(entity, up, opts)
	t.Cleanup(func() { h.Close(context.Background()) })
	return h
}

func exhausted(err error) bool {
	se, ok := errors.AsType[contract.StatusError](err)
	return ok && se.Status == contract.StatusResourceExhausted
}

// fill is the i-th offer of a camera filled to MaxSessions: the first
// MaxSessionsPerFabric of fabric 1, the next of fabric 2.
func fill(i int) wpdef.OfferArgs {
	return offerOf(uint16(i+1), uint8(1+i/camera.MaxSessionsPerFabric))
}

// offerOf is offer(id) from fabric's controller.
func offerOf(id uint16, fabric uint8) wpdef.OfferArgs {
	o := offer(id)
	o.FabricIndex, o.PeerNodeID = fabric, 0x70+uint64(fabric)
	return o
}

// TestFailedReofferEndsTheSession (HR-P1-err-05): a re-offer whose new
// Home Assistant session fails ends the session properly — the
// controller is told, which also takes it out of CurrentSessions.
func TestFailedReofferEndsTheSession(t *testing.T) {
	up := &upstream{}
	h, _ := newHost(t, up)
	sig := &signaller{}
	h.Attach(sig)
	ctx := context.Background()
	if _, err := h.ProvideOffer(ctx, offer(7)); err != nil {
		t.Fatal(err)
	}
	up.mu.Lock()
	up.offerErr = context.DeadlineExceeded
	up.mu.Unlock()
	if _, err := h.ProvideOffer(ctx, offer(7)); err == nil {
		t.Fatal("the failed re-offer succeeded")
	}
	if up.stream(0).unsubscribed() != 1 {
		t.Fatal("the replaced Home Assistant session is open")
	}
	if !slices.Contains(sig.seen(), "end 7 0") {
		t.Fatalf("the controller was not told: %q", sig.seen())
	}
}

// TestFailedFirstOfferSendsNoEnd (HR-P1-err-05's negative): a first
// offer that fails is the command's error only; the kit holds no session
// to end.
func TestFailedFirstOfferSendsNoEnd(t *testing.T) {
	up := &upstream{offerErr: context.DeadlineExceeded}
	h, _ := newHost(t, up)
	sig := &signaller{}
	h.Attach(sig)
	if _, err := h.ProvideOffer(context.Background(), offer(7)); err == nil {
		t.Fatal("the failed offer succeeded")
	}
	if len(sig.seen()) != 0 {
		t.Fatalf("signals %q", sig.seen())
	}
}

// TestCandidatesAfterSessionAreBounded (HR-P1-bound-17): after Home
// Assistant named its session a call carries at most
// MaxPendingCandidates and a session at most MaxSessionCandidates; beyond
// is refused whole with RESOURCE_EXHAUSTED and nothing is sent.
func TestCandidatesAfterSessionAreBounded(t *testing.T) {
	up := &upstream{script: func(s *stream) { s.push(session("01HA")) }}
	h, _ := newHost(t, up)
	h.Attach(&signaller{})
	ctx := context.Background()
	if _, err := h.ProvideOffer(ctx, offer(1)); err != nil {
		t.Fatal(err)
	}
	cands := func(n int) []wpdef.ICECandidateStruct {
		out := make([]wpdef.ICECandidateStruct, n)
		for i := range out {
			out[i].Candidate, out[i].SdpMid, out[i].SdpmLineIndex = fmt.Sprintf("candidate:%d", i), spec.ValueOf("0"), spec.ValueOf[uint16](0)
		}
		return out
	}
	// The session event arrives on the relay; wait until a candidate
	// passes through unqueued.
	if err := h.ProvideIceCandidates(ctx, 1, cands(1)); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the first candidate", func() bool { return len(up.cands()) == 1 })
	if err := h.ProvideIceCandidates(ctx, 1, cands(camera.MaxPendingCandidates+1)); !errors.Is(err, camera.ErrTooManyCandidates) || !exhausted(err) {
		t.Fatalf("a call beyond the per-call bound: %v", err)
	}
	if n := len(up.cands()); n != 1 {
		t.Fatalf("%d candidates sent", n)
	}
	sent := 1
	for sent+camera.MaxPendingCandidates <= camera.MaxSessionCandidates {
		if err := h.ProvideIceCandidates(ctx, 1, cands(camera.MaxPendingCandidates)); err != nil {
			t.Fatalf("after %d: %v", sent, err)
		}
		sent += camera.MaxPendingCandidates
	}
	rest := camera.MaxSessionCandidates - sent
	if err := h.ProvideIceCandidates(ctx, 1, cands(rest+1)); !errors.Is(err, camera.ErrTooManyCandidates) || !exhausted(err) {
		t.Fatalf("a call beyond the session's bound: %v", err)
	}
	if n := len(up.cands()); n != sent {
		t.Fatalf("%d candidates sent, want %d", n, sent)
	}
}

// blockingImage is an upstream whose still waits for release.
type blockingImage struct {
	upstream
	entered atomic.Int32
	release chan struct{}
	jpeg    []byte
}

func (b *blockingImage) Image(context.Context, string, int, int) (data []byte, contentType string, err error) {
	b.entered.Add(1)
	<-b.release
	return b.jpeg, "image/jpeg", nil
}

// TestSnapshotsShareOneRequest (HR-P1-bound-17): concurrent snapshots of
// one camera share one camera proxy request, and a still younger than
// StillReuse is reused for the same size; another size is a request of
// its own.
func TestSnapshotsShareOneRequest(t *testing.T) {
	up := &blockingImage{release: make(chan struct{}), jpeg: jpegOf(t, 32, 24)}
	h, _ := newHost(t, up)
	ctx := context.Background()
	res := csdef.VideoResolutionStruct{Width: 640, Height: 480}
	var wg sync.WaitGroup
	var bad atomic.Int32
	for range 20 {
		wg.Go(func() {
			got, err := h.CaptureSnapshot(ctx, nil, res)
			if err != nil || !bytes.Equal(got.Data, up.jpeg) {
				bad.Add(1)
			}
		})
	}
	eventually(t, "the request", func() bool { return up.entered.Load() >= 1 })
	time.Sleep(50 * time.Millisecond)
	close(up.release)
	wg.Wait()
	if n := up.entered.Load(); n != 1 || bad.Load() != 0 {
		t.Fatalf("%d requests for 20 concurrent snapshots, %d failed", n, bad.Load())
	}
	if _, err := h.CaptureSnapshot(ctx, nil, res); err != nil || up.entered.Load() != 1 {
		t.Fatalf("a snapshot right after: %v, %d requests", err, up.entered.Load())
	}
	if _, err := h.CaptureSnapshot(ctx, nil, csdef.VideoResolutionStruct{Width: 320, Height: 240}); err != nil || up.entered.Load() != 2 {
		t.Fatalf("another size: %v, %d requests", err, up.entered.Load())
	}
}

// TestSnapshotWaitEndsWithContext (HR-P1-bound-17's negative): a caller
// waiting for the shared request is released by its own context.
func TestSnapshotWaitEndsWithContext(t *testing.T) {
	up := &blockingImage{release: make(chan struct{}), jpeg: jpegOf(t, 32, 24)}
	h, _ := newHost(t, up)
	defer close(up.release)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := h.CaptureSnapshot(ctx, nil, csdef.VideoResolutionStruct{Width: 1, Height: 1}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("a wait past the caller's context: %v", err)
	}
}

// TestEmptyAnswerEndsTheSession (HR-P1-data-20): an answer without an
// SDP is not relayed as an empty one; the session ends as for Home
// Assistant's error.
func TestEmptyAnswerEndsTheSession(t *testing.T) {
	up := &upstream{script: func(s *stream) {
		s.push(session("01HA"))
		s.push(map[string]any{"type": "answer", "answer": ""})
	}}
	h, _ := newHost(t, up)
	sig := &signaller{}
	h.Attach(sig)
	if _, err := h.ProvideOffer(context.Background(), offer(5)); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the end", func() bool { return slices.Contains(sig.seen(), "end 5 0") })
	if slices.ContainsFunc(sig.seen(), func(s string) bool { return len(s) > 6 && s[:6] == "answer" }) {
		t.Fatalf("an empty answer was relayed: %q", sig.seen())
	}
}

// TestHACandidatesAreBounded (HR-P2-bound-40): Home Assistant's side of
// a session is held to the controller's bound — at most
// MaxSessionCandidates candidates reach the controller, and the session
// ends (ICE failed) at the next one; a candidate longer than
// MaxCandidateBytes is refused, the session stays.
func TestHACandidatesAreBounded(t *testing.T) {
	up := &upstream{}
	h := boundedHost(t, up, camera.Options{})
	sig := &signaller{}
	h.Attach(sig)
	if _, err := h.ProvideOffer(context.Background(), offer(5)); err != nil {
		t.Fatal(err)
	}
	st := up.stream(0)
	st.push(session("ha-1"))
	long := "candidate:" + strings.Repeat("A", camera.MaxCandidateBytes)
	st.push(candidate(long, "0", 0))
	for i := range camera.MaxSessionCandidates {
		st.push(candidate(fmt.Sprintf("candidate:%d", i), "0", 0))
	}
	eventually(t, "the session's end", func() bool { return slices.Contains(sig.seen(), "end 5 0") })
	cands := 0
	for _, c := range sig.seen() {
		if strings.HasPrefix(c, "candidate ") {
			cands++
			if strings.Contains(c, long) {
				t.Fatal("a candidate over MaxCandidateBytes reached the controller")
			}
		}
	}
	if cands != camera.MaxSessionCandidates-1 {
		t.Fatalf("%d of Home Assistant's candidates reached the controller, want %d", cands, camera.MaxSessionCandidates-1)
	}
	eventually(t, "the unsubscribe", func() bool { return st.unsubscribed() == 1 })
}

// TestHAErrorTextIsCut (HR-P2-bound-39): Home Assistant's error event
// is logged with its text cut to ha.MaxErrorText.
func TestHAErrorTextIsCut(t *testing.T) {
	long := strings.Repeat("x", 1<<20)
	up := &upstream{script: func(s *stream) {
		s.push(map[string]any{"type": "error", "code": "c" + long, "message": "m" + long})
	}}
	h, logs := newHost(t, up)
	sig := &signaller{}
	h.Attach(sig)
	if _, err := h.ProvideOffer(context.Background(), offer(2)); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the end", func() bool { return len(sig.seen()) == 1 })
	h.Close(context.Background())
	if n := len(logs.String()); n > 4*ha.MaxErrorText+2048 || !strings.Contains(logs.String(), "camera.ha_error") {
		t.Fatalf("the log holds %d bytes", n)
	}
}

// answeringUp is an upstream whose every session answers at once: a
// standing live view, which signals nothing after.
func answeringUp() *upstream {
	return &upstream{script: func(s *stream) {
		s.push(session("ha"))
		s.push(map[string]any{"type": "answer", "answer": "v=0 answer"})
	}}
}

func ends(sig *signaller) []string {
	return slices.DeleteFunc(sig.seen(), func(s string) bool { return !strings.HasPrefix(s, "end ") })
}

// TestCandidateMidIsBounded (HR-P4-bound-21): MaxCandidateBytes bounds a
// candidate with its sdpMid. Home Assistant's candidate with a long
// sdpMid is refused and logged, the session stays; a controller's is
// refused with CONSTRAINT_ERROR before Home Assistant is asked.
func TestCandidateMidIsBounded(t *testing.T) {
	up := &upstream{}
	h := boundedHost(t, up, camera.Options{})
	sig := &signaller{}
	h.Attach(sig)
	ctx := context.Background()
	if _, err := h.ProvideOffer(ctx, offer(5)); err != nil {
		t.Fatal(err)
	}
	st := up.stream(0)
	st.push(session("ha-1"))
	mid := strings.Repeat("M", camera.MaxCandidateBytes)
	st.push(candidate("candidate:long", mid, 0))
	st.push(candidate("candidate:short", "0", 0))
	eventually(t, "the short candidate", func() bool {
		return slices.ContainsFunc(sig.seen(), func(s string) bool { return strings.HasPrefix(s, "candidate 5 candidate:short") })
	})
	for _, s := range sig.seen() {
		if strings.Contains(s, "candidate:long") {
			t.Fatalf("a candidate with a %d-byte sdpMid reached the controller", len(mid))
		}
	}
	var c wpdef.ICECandidateStruct
	c.Candidate, c.SdpMid, c.SdpmLineIndex = "candidate:a", spec.ValueOf(mid), spec.ValueOf[uint16](0)
	err := h.ProvideIceCandidates(ctx, 5, []wpdef.ICECandidateStruct{c})
	if se, ok := errors.AsType[contract.StatusError](err); !ok || se.Status != contract.StatusConstraintError {
		t.Fatalf("a controller's candidate over MaxCandidateBytes: %v", err)
	}
}

// TestHostDeclaresTheKitsBounds (#83): the host hands go-hamapping's
// camera row the kit's bounds, MaxSessions and MaxSessionsPerFabric, and
// its eviction: Options.EvictIdle, DefaultEvictIdle by default, across
// fabrics (decision 80).
func TestHostDeclaresTheKitsBounds(t *testing.T) {
	h := boundedHost(t, &upstream{}, camera.Options{})
	if total, perFabric := h.WebRTCSessionBounds(); total != 8 || perFabric != 4 {
		t.Fatalf("bounds %d, %d", total, perFabric)
	}
	if idle, across := h.WebRTCSessionEviction(); idle != camera.DefaultEvictIdle || !across {
		t.Fatalf("eviction %v, %v", idle, across)
	}
	h = boundedHost(t, &upstream{}, camera.Options{EvictIdle: 5 * time.Second})
	if idle, _ := h.WebRTCSessionEviction(); idle != 5*time.Second {
		t.Fatalf("Options.EvictIdle 5s handed on as %v", idle)
	}
}

// TestEvictedSessionEndsQuietly (#83): the kit evicts a session through
// EndSession with OutOfResources and tells its controller itself; the
// host closes Home Assistant's session, and its relay sends no End of
// its own.
func TestEvictedSessionEndsQuietly(t *testing.T) {
	up := answeringUp()
	h := boundedHost(t, up, camera.Options{})
	sig := &signaller{}
	h.Attach(sig)
	ctx := context.Background()
	if _, err := h.ProvideOffer(ctx, offer(1)); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the answer", func() bool { return slices.Contains(sig.seen(), "answer 1 v=0 answer") })
	if err := h.EndSession(ctx, 1, uint8(wrdef.WebRTCEndReasonOutOfResources)); err != nil {
		t.Fatal(err)
	}
	if up.stream(0).unsubscribed() != 1 {
		t.Fatal("Home Assistant's session is open")
	}
	time.Sleep(50 * time.Millisecond)
	if e := ends(sig); len(e) != 0 {
		t.Fatalf("the relay sent an End of its own: %q", e)
	}
}

// TestFailedReofferTellsWhomItCan: a re-offer whose new Home Assistant
// session fails closes the old one also when no signaller is attached
// yet, and when the controller's End fails, which is logged only.
func TestFailedReofferTellsWhomItCan(t *testing.T) {
	ctx := context.Background()
	for _, sig := range []*signaller{nil, {endErr: errors.New("controller gone")}} {
		up := &upstream{}
		h := boundedHost(t, up, camera.Options{})
		if sig != nil {
			h.Attach(sig)
		}
		if _, err := h.ProvideOffer(ctx, offer(1)); err != nil {
			t.Fatal(err)
		}
		up.mu.Lock()
		up.offerErr = errors.New("webrtc_offer_failed")
		up.mu.Unlock()
		if _, err := h.ProvideOffer(ctx, offer(1)); err == nil {
			t.Fatal("a failed re-offer answered nil")
		}
		if up.stream(0).unsubscribed() != 1 {
			t.Fatal("the old Home Assistant session is open")
		}
		if sig != nil && !slices.Contains(sig.seen(), fmt.Sprintf("end 1 %d", camera.ReasonICEFailed)) {
			t.Fatalf("signals %q", sig.seen())
		}
	}
}

// TestHACandidateWithoutMidIsRelayed: MaxCandidateBytes counts the line
// alone of Home Assistant's candidate without an sdpMid, which goes to
// the controller with a null sdpMid.
func TestHACandidateWithoutMidIsRelayed(t *testing.T) {
	up := &upstream{script: func(s *stream) {
		s.push(session("ha"))
		s.push(map[string]any{"type": "candidate", "candidate": map[string]any{"candidate": "candidate:nomid", "sdpMLineIndex": 0}})
	}}
	h := boundedHost(t, up, camera.Options{})
	sig := &signaller{}
	h.Attach(sig)
	if _, err := h.ProvideOffer(context.Background(), offer(1)); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the relayed candidate", func() bool {
		return slices.Contains(sig.seen(), "candidate 1 candidate:nomid mid=true/ idx=false/0")
	})
}
