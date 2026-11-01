// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package camera_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	gfcamera "github.com/SukramJ/go-fabric/cluster/camera"
	"github.com/SukramJ/go-fabric/cluster/spec"
	csdef "github.com/SukramJ/go-fabric/cluster/spec/cameraavstreammanagement"
	wpdef "github.com/SukramJ/go-fabric/cluster/spec/webrtctransportprovider"
	"github.com/SukramJ/go-fabric/contract"

	"github.com/SukramJ/go-ha2fabric/internal/camera"
	"github.com/SukramJ/go-ha2fabric/internal/ha"
)

const entity = "camera.front_door"

// stream is a fake camera/webrtc/offer subscription.
type stream struct {
	ch    chan json.RawMessage
	mu    sync.Mutex
	err   error
	unsub int
	done  bool
}

func newStream() *stream { return &stream{ch: make(chan json.RawMessage, 16)} }

func (s *stream) Events() <-chan json.RawMessage { return s.ch }

func (s *stream) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

func (s *stream) Unsubscribe(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.unsub++
	if !s.done {
		s.done = true
		close(s.ch)
	}
	return nil
}

func (s *stream) unsubscribed() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.unsub
}

func (s *stream) push(ev map[string]any) {
	b, _ := json.Marshal(ev)
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.done {
		s.ch <- b
	}
}

func (s *stream) drop(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.err = err
	s.done = true
	close(s.ch)
}

// upstream is a fake Home Assistant.
type upstream struct {
	mu         sync.Mutex
	offers     []string
	streams    []*stream
	candidates []string
	offerErr   error
	img        []byte
	ct         string
	imgErr     error
	script     func(*stream)
}

func (u *upstream) Subscribe(_ context.Context, entityID, offer string) (camera.Stream, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.offerErr != nil {
		return nil, u.offerErr
	}
	s := newStream()
	u.offers = append(u.offers, entityID+" "+offer)
	u.streams = append(u.streams, s)
	if u.script != nil {
		u.script(s)
	}
	return s, nil
}

func (u *upstream) Candidate(_ context.Context, entityID, sessionID string, c ha.ICECandidate) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	mid, idx := "nil", "nil"
	if c.SdpMid != nil {
		mid = *c.SdpMid
	}
	if c.SdpMLineIndex != nil {
		idx = fmt.Sprint(*c.SdpMLineIndex)
	}
	u.candidates = append(u.candidates, fmt.Sprintf("%s %s %s %s %s", entityID, sessionID, c.Candidate, mid, idx))
	return nil
}

func (u *upstream) Image(context.Context, string, int, int) (data []byte, contentType string, err error) {
	if u.imgErr != nil {
		return nil, "", u.imgErr
	}
	return u.img, u.ct, nil
}

func (u *upstream) stream(i int) *stream {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.streams[i]
}

func (u *upstream) cands() []string {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]string(nil), u.candidates...)
}

// signaller is a fake *camera.Provider.
type signaller struct {
	mu        sync.Mutex
	calls     []string
	answerErr func(n int) error
	candErr   error
	answers   int
}

func (s *signaller) Answer(_ context.Context, id uint16, sdp string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.answers++
	if s.answerErr != nil {
		if err := s.answerErr(s.answers); err != nil {
			return err
		}
	}
	s.calls = append(s.calls, fmt.Sprintf("answer %d %s", id, sdp))
	return nil
}

func (s *signaller) IceCandidates(_ context.Context, id uint16, cs []gfcamera.ProviderICECandidate) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.candErr != nil {
		return s.candErr
	}
	for _, c := range cs {
		s.calls = append(s.calls, fmt.Sprintf("candidate %d %s mid=%v/%s idx=%v/%d", id, c.Candidate, c.SdpMid.Null, c.SdpMid.Value, c.SdpmLineIndex.Null, c.SdpmLineIndex.Value))
	}
	return nil
}

func (s *signaller) End(_ context.Context, id uint16, reason uint8) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, fmt.Sprintf("end %d %d", id, reason))
	return nil
}

func (s *signaller) seen() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.calls...)
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("no %s", what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func newHost(t *testing.T, up camera.Upstream) (*camera.Host, *bytes.Buffer) {
	t.Helper()
	var logs bytes.Buffer
	h := camera.New(entity, up, camera.Options{
		Logger:        slog.New(slog.NewTextHandler(&lockedWriter{w: &logs}, &slog.HandlerOptions{Level: slog.LevelDebug})),
		RetryInterval: time.Millisecond,
	})
	t.Cleanup(func() { h.Close(context.Background()) })
	return h, &logs
}

type lockedWriter struct {
	mu sync.Mutex
	w  *bytes.Buffer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

func offer(id uint16) wpdef.OfferArgs {
	t := true
	return wpdef.OfferArgs{
		SessionID: id, StreamUsage: wpdef.StreamUsageLiveView, VideoStreams: &[]uint16{3, 4}, AudioStreams: &[]uint16{},
		PeerNodeID: 0x77, FabricIndex: 1, OriginatingEndpointID: 2, Sdp: "v=0 offer", MetadataEnabled: &t,
	}
}

func session(id string) map[string]any { return map[string]any{"type": "session", "session_id": id} }

func candidate(c string, mid, idx any) map[string]any {
	return map[string]any{"type": "candidate", "candidate": map[string]any{"candidate": c, "sdpMid": mid, "sdpMLineIndex": idx}}
}

func TestOfferAnswerCandidates(t *testing.T) {
	up := &upstream{script: func(s *stream) {
		s.push(session("01HA"))
		s.push(map[string]any{"type": "answer", "answer": "v=0 answer"})
		s.push(candidate("candidate:1", "0", 0))
		s.push(candidate("candidate:2", "", nil))
		s.push(candidate("", nil, nil)) // end of candidates: not relayed
		s.push(map[string]any{"type": "later"})
	}}
	h, _ := newHost(t, up)
	sig := &signaller{}
	h.Attach(sig)
	got, err := h.ProvideOffer(context.Background(), offer(5))
	if err != nil {
		t.Fatal(err)
	}
	if got.Id != 5 || got.PeerNodeId != 0x77 || got.PeerEndpointId != 2 || got.FabricIndex != 1 || got.StreamUsage != wpdef.StreamUsageLiveView ||
		!got.MetadataEnabled || got.VideoStreams == nil || len(*got.VideoStreams) != 1 || (*got.VideoStreams)[0] != 3 ||
		got.AudioStreams == nil || (*got.AudioStreams)[0] != camera.NominalStreamID {
		t.Fatalf("session %+v", got)
	}
	want := []string{
		"answer 5 v=0 answer",
		"candidate 5 candidate:1 mid=false/0 idx=false/0",
		"candidate 5 candidate:2 mid=true/ idx=true/0",
	}
	eventually(t, "signals", func() bool { return len(sig.seen()) == len(want) })
	if fmt.Sprint(sig.seen()) != fmt.Sprint(want) {
		t.Fatalf("signals %q", sig.seen())
	}
	if up.offers[0] != entity+" v=0 offer" {
		t.Fatalf("offers %q", up.offers)
	}
	if h.Entity() != entity {
		t.Fatal(h.Entity())
	}
}

func TestCandidateBeforeSession(t *testing.T) {
	up := &upstream{}
	h, _ := newHost(t, up)
	h.Attach(&signaller{})
	ctx := context.Background()
	if _, err := h.ProvideOffer(ctx, offer(1)); err != nil {
		t.Fatal(err)
	}
	var c1, c2 wpdef.ICECandidateStruct
	c1.Candidate, c1.SdpMid, c1.SdpmLineIndex = "candidate:a", spec.ValueOf("0"), spec.ValueOf[uint16](1)
	c2.Candidate, c2.SdpMid, c2.SdpmLineIndex = "candidate:b", spec.NullOf[string](), spec.NullOf[uint16]()
	if err := h.ProvideIceCandidates(ctx, 1, []wpdef.ICECandidateStruct{c1}); err != nil {
		t.Fatal(err)
	}
	if len(up.cands()) != 0 {
		t.Fatal("a candidate before Home Assistant named its session")
	}
	up.stream(0).push(session("01HA"))
	eventually(t, "the queued candidate", func() bool { return len(up.cands()) == 1 })
	if err := h.ProvideIceCandidates(ctx, 1, []wpdef.ICECandidateStruct{c2}); err != nil {
		t.Fatal(err)
	}
	want := []string{entity + " 01HA candidate:a 0 1", entity + " 01HA candidate:b nil nil"}
	if fmt.Sprint(up.cands()) != fmt.Sprint(want) {
		t.Fatalf("candidates %q", up.cands())
	}
	if err := h.ProvideIceCandidates(ctx, 9, []wpdef.ICECandidateStruct{c1}); !errors.Is(err, gfcamera.ErrUnknownSession) {
		t.Fatalf("an unknown session: %v", err)
	}
}

func TestHAErrorEndsTheSession(t *testing.T) {
	up := &upstream{script: func(s *stream) {
		s.push(session("01HA"))
		s.push(map[string]any{"type": "error", "code": "go2rtc_webrtc_offer_failed", "message": "no stream source"})
	}}
	h, logs := newHost(t, up)
	sig := &signaller{}
	h.Attach(sig)
	if _, err := h.ProvideOffer(context.Background(), offer(2)); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the end", func() bool { return len(sig.seen()) == 1 })
	if sig.seen()[0] != fmt.Sprintf("end 2 %d", camera.ReasonICEFailed) || up.stream(0).unsubscribed() != 1 {
		t.Fatalf("signals %q, unsubscribed %d", sig.seen(), up.stream(0).unsubscribed())
	}
	if !strings.Contains(logs.String(), "go2rtc_webrtc_offer_failed") {
		t.Fatal("the error is not logged")
	}
}

func TestEndSessionUnsubscribes(t *testing.T) {
	up := &upstream{}
	h, _ := newHost(t, up)
	sig := &signaller{}
	h.Attach(sig)
	ctx := context.Background()
	if _, err := h.ProvideOffer(ctx, offer(3)); err != nil {
		t.Fatal(err)
	}
	if err := h.EndSession(ctx, 3, 2); err != nil {
		t.Fatal(err)
	}
	if up.stream(0).unsubscribed() != 1 {
		t.Fatal("not unsubscribed")
	}
	if err := h.EndSession(ctx, 3, 2); err != nil {
		t.Fatal(err)
	}
	if len(sig.seen()) != 0 {
		t.Fatalf("a signal for a session the controller ended: %q", sig.seen())
	}
}

func TestAnswerRetriesUnknownSession(t *testing.T) {
	up := &upstream{script: func(s *stream) { s.push(map[string]any{"type": "answer", "answer": "a"}) }}
	h, _ := newHost(t, up)
	sig := &signaller{answerErr: func(n int) error {
		if n < 4 {
			return fmt.Errorf("%w: 4", gfcamera.ErrUnknownSession)
		}
		return nil
	}}
	h.Attach(sig)
	if _, err := h.ProvideOffer(context.Background(), offer(4)); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the answer", func() bool { return len(sig.seen()) == 1 })
	if sig.seen()[0] != "answer 4 a" {
		t.Fatalf("signals %q", sig.seen())
	}

	// Ten refusals end the session.
	sig2 := &signaller{answerErr: func(int) error { return gfcamera.ErrUnknownSession }}
	up2 := &upstream{script: up.script}
	h2, logs := newHost(t, up2)
	h2.Attach(sig2)
	if _, err := h2.ProvideOffer(context.Background(), offer(4)); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the end", func() bool { return len(sig2.seen()) == 1 })
	sig2.mu.Lock()
	n := sig2.answers
	sig2.mu.Unlock()
	if n != 10 || sig2.seen()[0] != "end 4 0" || !strings.Contains(logs.String(), "camera.signal_failed") {
		t.Fatalf("%d answers, signals %q", n, sig2.seen())
	}
}

func TestPayloadTooLargeEndsTheSession(t *testing.T) {
	up := &upstream{script: func(s *stream) { s.push(map[string]any{"type": "answer", "answer": strings.Repeat("a", 1500)}) }}
	h, logs := newHost(t, up)
	sig := &signaller{answerErr: func(int) error {
		return fmt.Errorf("camera: WebRTC session 6: %w: command 0x01 of 1530 bytes, the session carries 1178", gfcamera.ErrPayloadTooLarge)
	}}
	h.Attach(sig)
	if _, err := h.ProvideOffer(context.Background(), offer(6)); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the end", func() bool { return len(sig.seen()) == 1 })
	if sig.seen()[0] != "end 6 0" || up.stream(0).unsubscribed() != 1 {
		t.Fatalf("signals %q", sig.seen())
	}
	if !strings.Contains(logs.String(), "camera.signal_too_large") || !strings.Contains(logs.String(), "over UDP") {
		t.Fatalf("log %s", logs.String())
	}
	sig.answerErr = nil
}

func TestCandidateSignalFails(t *testing.T) {
	up := &upstream{script: func(s *stream) {
		s.push(candidate("candidate:1", "0", -1)) // refused: not a uint16
		s.push(candidate("candidate:2", "0", 0))
	}}
	h, logs := newHost(t, up)
	sig := &signaller{candErr: errors.New("matter status 0x8B")}
	h.Attach(sig)
	if _, err := h.ProvideOffer(context.Background(), offer(7)); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the end", func() bool { return len(sig.seen()) == 1 })
	if sig.seen()[0] != "end 7 0" || !strings.Contains(logs.String(), "camera.candidate_refused") || !strings.Contains(logs.String(), "command=IceCandidates") {
		t.Fatalf("signals %q, log %s", sig.seen(), logs.String())
	}
}

func TestSignalWaitsForAttach(t *testing.T) {
	up := &upstream{script: func(s *stream) { s.push(map[string]any{"type": "answer", "answer": "a"}) }}
	var logs bytes.Buffer
	h := camera.New(entity, up, camera.Options{Logger: slog.New(slog.NewTextHandler(&lockedWriter{w: &logs}, nil)), RetryInterval: 20 * time.Millisecond})
	defer h.Close(context.Background())
	if _, err := h.ProvideOffer(context.Background(), offer(8)); err != nil {
		t.Fatal(err)
	}
	sig := &signaller{}
	time.Sleep(10 * time.Millisecond)
	h.Attach(sig)
	eventually(t, "the answer", func() bool { return len(sig.seen()) == 1 })

	// Without Attach the session ends after the bound.
	up2 := &upstream{script: up.script}
	h2, logs2 := newHost(t, up2)
	if _, err := h2.ProvideOffer(context.Background(), offer(8)); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the drop", func() bool { return up2.stream(0).unsubscribed() == 1 })
	if !strings.Contains(logs2.String(), "no WebRtcTransportProvider attached") {
		t.Fatalf("log %s", logs2.String())
	}
}

func TestDisconnectEndsTheSession(t *testing.T) {
	up := &upstream{}
	h, _ := newHost(t, up)
	sig := &signaller{}
	h.Attach(sig)
	if _, err := h.ProvideOffer(context.Background(), offer(9)); err != nil {
		t.Fatal(err)
	}
	up.stream(0).drop(ha.ErrDisconnected)
	eventually(t, "the end", func() bool { return len(sig.seen()) == 1 })
	if sig.seen()[0] != fmt.Sprintf("end 9 %d", camera.ReasonUnknown) {
		t.Fatalf("signals %q", sig.seen())
	}
}

func TestReofferAndReattach(t *testing.T) {
	up := &upstream{}
	h, _ := newHost(t, up)
	sig := &signaller{}
	h.Attach(sig)
	ctx := context.Background()
	if _, err := h.ProvideOffer(ctx, offer(1)); err != nil {
		t.Fatal(err)
	}
	if _, err := h.ProvideOffer(ctx, offer(1)); err != nil {
		t.Fatal(err)
	}
	if up.stream(0).unsubscribed() != 1 || up.stream(1).unsubscribed() != 0 {
		t.Fatal("a re-offer keeps the first Home Assistant session")
	}
	h.Attach(sig) // the same provider: nothing changes
	if up.stream(1).unsubscribed() != 0 {
		t.Fatal("re-attaching the same provider dropped the session")
	}
	h.Attach(&signaller{})
	if up.stream(1).unsubscribed() != 1 {
		t.Fatal("a new provider kept the old one's session")
	}
	if len(sig.seen()) != 0 {
		t.Fatalf("signals %q", sig.seen())
	}
}

func TestOfferRefused(t *testing.T) {
	up := &upstream{offerErr: &ha.Error{Code: "webrtc_offer_failed", Message: "Camera does not support WebRTC"}}
	h, _ := newHost(t, up)
	if _, err := h.ProvideOffer(context.Background(), offer(1)); err == nil {
		t.Fatal("a refused offer")
	}
}

func TestClose(t *testing.T) {
	up := &upstream{}
	h := camera.New(entity, up, camera.Options{Logger: slog.New(slog.DiscardHandler)})
	sig := &signaller{}
	h.Attach(sig)
	if _, err := h.ProvideOffer(context.Background(), offer(1)); err != nil {
		t.Fatal(err)
	}
	if _, err := h.ProvideOffer(context.Background(), offer(2)); err != nil {
		t.Fatal(err)
	}
	h.Close(context.Background())
	h.Close(context.Background())
	if len(sig.seen()) != 2 || !strings.HasSuffix(sig.seen()[0], " 12") || up.stream(0).unsubscribed() != 1 || up.stream(1).unsubscribed() != 1 {
		t.Fatalf("signals %q", sig.seen())
	}
	if _, err := h.ProvideOffer(context.Background(), offer(3)); !errors.Is(err, camera.ErrClosed) {
		t.Fatalf("an offer after Close: %v", err)
	}
	// A host never attached closes without signals.
	camera.New(entity, up, camera.Options{}).Close(context.Background())
}

func TestRefusedCommandsAndNominalStreams(t *testing.T) {
	h, _ := newHost(t, &upstream{})
	ctx := context.Background()
	_, _, err := h.SolicitOffer(ctx, offer(1))
	var se contract.StatusError
	if !errors.As(err, &se) || se.Status != 0xCB || !strings.Contains(err.Error(), "offer only") {
		t.Fatalf("SolicitOffer: %v", err)
	}
	if err := h.ProvideAnswer(ctx, 1, "v=0"); !errors.Is(err, camera.ErrProvideAnswer) {
		t.Fatalf("ProvideAnswer: %v", err)
	}
	if id, err := h.VideoStreamAllocate(ctx, csdef.VideoStreamStruct{}); id != camera.NominalStreamID || err != nil {
		t.Fatal(id, err)
	}
	if id, err := h.AudioStreamAllocate(ctx, csdef.AudioStreamStruct{}); id != camera.NominalStreamID || err != nil {
		t.Fatal(id, err)
	}
	if id, err := h.SnapshotStreamAllocate(ctx, csdef.SnapshotStreamStruct{}); id != camera.NominalStreamID || err != nil {
		t.Fatal(id, err)
	}
	h.VideoStreamAllocated(ctx, csdef.VideoStreamStruct{}, csdef.StreamNewAllocation)
	for _, fn := range []func(uint16) error{
		func(id uint16) error { return h.VideoStreamModify(ctx, id, nil, nil) },
		func(id uint16) error { return h.VideoStreamDeallocate(ctx, id) },
		func(id uint16) error { return h.AudioStreamDeallocate(ctx, id) },
		func(id uint16) error { return h.SnapshotStreamModify(ctx, id, nil, nil) },
		func(id uint16) error { return h.SnapshotStreamDeallocate(ctx, id) },
	} {
		if err := fn(camera.NominalStreamID); err != nil {
			t.Fatal(err)
		}
		if err := fn(2); !errors.Is(err, camera.ErrUnknownStream) {
			t.Fatalf("stream 2: %v", err)
		}
	}
}

func jpegOf(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, image.NewGray(image.Rect(0, 0, w, h)), nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestCaptureSnapshot(t *testing.T) {
	jp := jpegOf(t, 320, 240)
	up := &upstream{img: jp, ct: "image/jpeg"}
	h, _ := newHost(t, up)
	ctx := context.Background()
	got, err := h.CaptureSnapshot(ctx, nil, csdef.VideoResolutionStruct{Width: 640, Height: 480})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Data, jp) || got.ImageCodec != csdef.ImageCodecJpeg || got.Resolution.Width != 320 || got.Resolution.Height != 240 {
		t.Fatalf("snapshot %d bytes %v %+v", len(got.Data), got.ImageCodec, got.Resolution)
	}
	up.imgErr = fmt.Errorf("%w: %s", ha.ErrCameraOff, entity)
	if _, err := h.CaptureSnapshot(ctx, nil, csdef.VideoResolutionStruct{Width: 1, Height: 1}); !errors.Is(err, ha.ErrCameraOff) {
		t.Fatalf("a camera that is off: %v", err)
	}
	up.imgErr = nil
	up.ct = "image/png"
	if _, err := h.CaptureSnapshot(ctx, nil, csdef.VideoResolutionStruct{Width: 1, Height: 1}); !errors.Is(err, camera.ErrNotJPEG) {
		t.Fatalf("a PNG: %v", err)
	}
	up.ct, up.img = "image/jpeg", []byte("not a jpeg")
	if _, err := h.CaptureSnapshot(ctx, nil, csdef.VideoResolutionStruct{Width: 1, Height: 1}); !errors.Is(err, camera.ErrNotJPEG) {
		t.Fatalf("garbage: %v", err)
	}
}
