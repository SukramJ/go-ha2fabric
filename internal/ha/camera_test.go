// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package ha_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"log/slog"
	"strings"
	"testing"
	"time"

	"thingfabric.dev/go-ha2fabric/internal/ha"
	"thingfabric.dev/go-ha2fabric/internal/ha/hatest"
)

func testJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := range w {
		img.Set(x, 0, color.RGBA{R: 200, A: 255})
	}
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func nextEvent(t *testing.T, sub *ha.Subscription) ha.WebRTCEvent {
	t.Helper()
	select {
	case raw, ok := <-sub.Events():
		if !ok {
			t.Fatalf("the subscription ended: %v", sub.Err())
		}
		var ev ha.WebRTCEvent
		if err := json.Unmarshal(raw, &ev); err != nil {
			t.Fatal(err)
		}
		return ev
	case <-time.After(5 * time.Second):
		t.Fatal("no event")
	}
	return ha.WebRTCEvent{}
}

func TestClientWebRTCOffer(t *testing.T) {
	srv := hatest.New(t)
	var logs bytes.Buffer
	c, r, _ := start(t, ha.Options{
		URL: srv.URL(), Token: hatest.Token,
		Logger: slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
	})
	r.wait(t, "connected")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sub, err := c.WebRTCOffer(ctx, "camera.front_door", "v=0 offer")
	if err != nil {
		t.Fatal(err)
	}
	session := nextEvent(t, sub)
	if session.Type != ha.WebRTCSession || session.SessionID != "01JBSESSION1" {
		t.Fatalf("first event %+v", session)
	}
	if ev := nextEvent(t, sub); ev.Type != ha.WebRTCAnswer || ev.Answer != hatest.FakeAnswer {
		t.Fatalf("second event %+v", ev)
	}
	ev := nextEvent(t, sub)
	if ev.Type != ha.WebRTCCandidate || ev.Candidate == nil || ev.Candidate.Candidate != hatest.FakeCandidate ||
		ev.Candidate.SdpMid == nil || *ev.Candidate.SdpMid != "0" || ev.Candidate.SdpMLineIndex == nil || *ev.Candidate.SdpMLineIndex != 0 {
		t.Fatalf("third event %+v", ev)
	}
	o := srv.Offers()
	if len(o) != 1 || o[0].EntityID != "camera.front_door" || o[0].SDP != "v=0 offer" || o[0].ID != sub.ID() {
		t.Fatalf("offers %+v, subscription %d", o, sub.ID())
	}

	mid := "0"
	if err := c.WebRTCCandidate(ctx, "camera.front_door", session.SessionID, ha.ICECandidate{Candidate: "candidate:2", SdpMid: &mid}); err != nil {
		t.Fatal(err)
	}
	if cd := srv.Candidates(); len(cd) != 1 || cd[0].SessionID != "01JBSESSION1" || cd[0].Candidate["candidate"] != "candidate:2" ||
		cd[0].Candidate["sdpMid"] != "0" || cd[0].Candidate["sdpMLineIndex"] != nil {
		t.Fatalf("candidates %+v", cd)
	}

	// An event that follows on the id reaches the subscription.
	srv.PushOfferEvent(o[0], hatest.ErrorEvent("go2rtc_webrtc_offer_failed", "gone"))
	if ev := nextEvent(t, sub); ev.Type != ha.WebRTCError || ev.Code != "go2rtc_webrtc_offer_failed" || ev.Message != "gone" {
		t.Fatalf("error event %+v", ev)
	}

	if err := sub.Unsubscribe(ctx); err != nil {
		t.Fatal(err)
	}
	if _, ok := <-sub.Events(); ok || sub.Err() != nil {
		t.Fatalf("after Unsubscribe: open or %v", sub.Err())
	}
	if u := srv.Unsubscribed(); len(u) != 1 || u[0] != sub.ID() {
		t.Fatalf("unsubscribed %v", u)
	}
	// A second unsubscribe is Home Assistant's not_found.
	var herr *ha.Error
	if err := sub.Unsubscribe(ctx); !errors.As(err, &herr) || herr.Code != "not_found" {
		t.Fatalf("second Unsubscribe: %v", err)
	}
	if strings.Contains(logs.String(), hatest.Token) {
		t.Fatal("the token reached the log")
	}
}

func TestClientSubscribeRefused(t *testing.T) {
	srv := hatest.New(t)
	c, r, _ := start(t, ha.Options{URL: srv.URL(), Token: hatest.Token, Logger: slog.New(slog.DiscardHandler)})
	r.wait(t, "connected")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var herr *ha.Error
	if _, err := c.Subscribe(ctx, "camera/nope", nil); !errors.As(err, &herr) || herr.Code != "unknown_command" {
		t.Fatalf("an unknown command: %v", err)
	}
	if _, err := ha.New(ha.Options{URL: srv.URL()}).Subscribe(ctx, "camera/webrtc/offer", nil); !errors.Is(err, ha.ErrNotConnected) {
		t.Fatalf("Subscribe before Run: %v", err)
	}
}

func TestClientSubscriptionEndsOnReconnect(t *testing.T) {
	srv := hatest.New(t)
	c, r, _ := start(t, ha.Options{URL: srv.URL(), Token: hatest.Token, MinBackoff: 20 * time.Millisecond, MaxBackoff: 50 * time.Millisecond, Logger: slog.New(slog.DiscardHandler)})
	r.wait(t, "connected")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.SetWebRTCScript(func(o hatest.Offer) []map[string]any { return []map[string]any{hatest.SessionEvent(o.SessionID)} })
	sub, err := c.WebRTCOffer(ctx, "camera.front_door", "v=0")
	if err != nil {
		t.Fatal(err)
	}
	nextEvent(t, sub)
	srv.Down()
	select {
	case _, ok := <-sub.Events():
		if ok {
			t.Fatal("an event after the drop")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the subscription outlived its connection")
	}
	if !errors.Is(sub.Err(), ha.ErrDisconnected) {
		t.Fatalf("Err %v", sub.Err())
	}
	if err := sub.Unsubscribe(ctx); err != nil {
		t.Fatalf("Unsubscribe after the drop: %v", err)
	}
	srv.Up()
	r.wait(t, "connected")
	if len(srv.Unsubscribed()) != 0 {
		t.Fatal("an unsubscribe went to the new connection")
	}
}

func TestClientSubscriptionOverflow(t *testing.T) {
	srv := hatest.New(t)
	c, r, _ := start(t, ha.Options{URL: srv.URL(), Token: hatest.Token, Logger: slog.New(slog.DiscardHandler)})
	r.wait(t, "connected")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.SetWebRTCScript(func(o hatest.Offer) []map[string]any {
		evs := make([]map[string]any, ha.SubscriptionBuffer+1)
		for i := range evs {
			evs[i] = hatest.CandidateEvent(hatest.FakeCandidate, "0", 0)
		}
		return evs
	})
	sub, err := c.WebRTCOffer(ctx, "camera.front_door", "v=0")
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.Await(ctx, "overflow", func() bool { return sub.Err() != nil }); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(sub.Err(), ha.ErrSubscriptionOverflow) {
		t.Fatalf("Err %v", sub.Err())
	}
}

func TestHTTPBase(t *testing.T) {
	for in, want := range map[string]string{
		"ws://supervisor/core/websocket":        "http://supervisor/core/api",
		"ws://homeassistant:8123/api/websocket": "http://homeassistant:8123/api",
		"wss://ha.example/api/websocket/":       "https://ha.example/api",
		"ws://[::1]:8123/api/websocket":         "http://[::1]:8123/api",
	} {
		if got, err := ha.HTTPBase(in); err != nil || got != want {
			t.Errorf("HTTPBase(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	// HR-P0-err-03: an unbracketed IPv6 literal and a URL without a host
	// gave a base that url.Parse refuses or that names no host.
	for _, in := range []string{"http://ha/api/websocket", "ws://ha/api", "::", "ws://::/websocket", "ws:///websocket", "ws://:8123/websocket"} {
		if _, err := ha.HTTPBase(in); err == nil {
			t.Errorf("HTTPBase(%q) accepted", in)
		}
	}
}

func TestClientImage(t *testing.T) {
	srv := hatest.New(t)
	var logs bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	c := ha.New(ha.Options{URL: srv.URL(), Token: hatest.Token, Logger: log})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	jp := testJPEG(t, 64, 48)
	srv.SetImage("camera.front_door", jp)

	data, ct, err := c.Image(ctx, "camera.front_door", 640, 480)
	if err != nil || !bytes.Equal(data, jp) || ct != "image/jpeg" {
		t.Fatalf("Image: %d bytes, %q, %v", len(data), ct, err)
	}
	if _, _, err := c.Image(ctx, "camera.front_door", 0, 0); err != nil {
		t.Fatal(err)
	}
	if rq := srv.ImageRequests(); len(rq) != 2 || rq[0].Width != "640" || rq[0].Height != "480" || rq[1].Width != "" {
		t.Fatalf("requests %+v", rq)
	}

	errs := make([]error, 0, 5)
	_, _, err = ha.New(ha.Options{URL: srv.URL(), Token: "wrong-secret"}).Image(ctx, "camera.front_door", 0, 0)
	if !errors.Is(err, ha.ErrUnauthorized) || !strings.Contains(err.Error(), "401") {
		t.Fatalf("a wrong token: %v", err)
	}
	errs = append(errs, err)
	_, _, err = ha.New(ha.Options{URL: srv.URL()}).Image(ctx, "camera.front_door", 0, 0)
	if !errors.Is(err, ha.ErrUnauthorized) || !strings.Contains(err.Error(), "403") {
		t.Fatalf("no token: %v", err)
	}
	srv.SetCameraOff("camera.front_door", true)
	_, _, err = c.Image(ctx, "camera.front_door", 0, 0)
	if !errors.Is(err, ha.ErrCameraOff) {
		t.Fatalf("a camera that is off: %v", err)
	}
	errs = append(errs, err)
	srv.SetCameraOff("camera.front_door", false)
	_, _, err = c.Image(ctx, "camera.missing", 0, 0)
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("an unknown camera: %v", err)
	}
	errs = append(errs, err)
	_, _, err = ha.New(ha.Options{URL: srv.URL(), Token: hatest.Token, ImageLimit: int64(len(jp) - 1)}).Image(ctx, "camera.front_door", 0, 0)
	if !errors.Is(err, ha.ErrImageTooLarge) {
		t.Fatalf("an image over the limit: %v", err)
	}
	errs = append(errs, err)
	_, _, err = ha.New(ha.Options{URL: "ws://127.0.0.1:1/api/websocket", Token: hatest.Token}).Image(ctx, "camera.front_door", 0, 0)
	if err == nil {
		t.Fatal("a refused connection")
	}
	errs = append(errs, err)
	if _, _, err := ha.New(ha.Options{URL: "http://ha/api/websocket"}).Image(ctx, "camera.front_door", 0, 0); err == nil {
		t.Fatal("an http URL")
	}
	for _, err := range errs {
		if strings.Contains(err.Error(), hatest.Token) || strings.Contains(err.Error(), "wrong-secret") {
			t.Fatalf("the token is in the error %q", err)
		}
	}
	if strings.Contains(logs.String(), hatest.Token) {
		t.Fatal("the token reached the log")
	}
}

// TestFakeRefusesAsCore (HR-P3-test-10): the fake refuses the camera
// commands core 2026.10.3 refuses — an unknown camera and one that is off
// (camera/helper.py:14-26), a camera without WebRTC
// (camera/webrtc.py:213-248), a candidate that is not an
// RTCIceCandidateInit (:325-339) — and fires device_registry_updated
// remove for a removed device.
func TestFakeRefusesAsCore(t *testing.T) {
	srv := hatest.New(t)
	c, r, _ := start(t, ha.Options{URL: srv.URL(), Token: hatest.Token, Events: []string{ha.EventDeviceRegistryUpdated}, Logger: slog.New(slog.DiscardHandler)})
	r.wait(t, "connected")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	code := func(err error) string {
		var e *ha.Error
		if errors.As(err, &e) {
			return e.Code
		}
		return fmt.Sprint(err)
	}
	if _, err := c.WebRTCOffer(ctx, "camera.missing", "v=0"); code(err) != "home_assistant_error" {
		t.Errorf("an unknown camera's offer: %v", err)
	}
	srv.SetCameraOff("camera.front_door", true)
	if _, err := c.WebRTCOffer(ctx, "camera.front_door", "v=0"); code(err) != "home_assistant_error" || !strings.Contains(err.Error(), "off") {
		t.Errorf("an offer to a camera that is off: %v", err)
	}
	srv.SetCameraOff("camera.front_door", false)
	srv.SetWebRTC("camera.front_door", false)
	if _, err := c.WebRTCOffer(ctx, "camera.front_door", "v=0"); code(err) != "webrtc_offer_failed" {
		t.Errorf("an offer to a camera without WebRTC: %v", err)
	}
	if err := c.WebRTCCandidate(ctx, "camera.front_door", "s1", ha.ICECandidate{Candidate: "candidate:1 1 UDP 1 192.0.2.1 1 typ host"}); code(err) != "webrtc_candidate_failed" {
		t.Errorf("a candidate to a camera without WebRTC: %v", err)
	}
	srv.SetWebRTC("camera.front_door", true)
	err := c.Call(ctx, map[string]any{"type": "camera/webrtc/candidate", "entity_id": "camera.front_door", "session_id": "s1", "candidate": map[string]any{"sdpMid": "0"}}, nil)
	if code(err) != "invalid_format" {
		t.Errorf("a candidate without its candidate string: %v", err)
	}
	if n := len(srv.Offers()) + len(srv.Candidates()); n != 0 {
		t.Errorf("%d refused commands recorded", n)
	}
	if _, err := c.WebRTCOffer(ctx, "camera.front_door", "v=0"); err != nil {
		t.Fatalf("an offer to a camera with WebRTC: %v", err)
	}

	srv.RemoveDevice("dev-gone-unknown")
	devices, err := c.ListDevices(ctx)
	if err != nil || len(devices) == 0 {
		t.Fatalf("devices: %v", err)
	}
	srv.RemoveDevice(devices[0].ID)
	r.wait(t, "event:"+ha.EventDeviceRegistryUpdated)
	r.mu.Lock()
	ev := r.events[len(r.events)-1]
	r.mu.Unlock()
	if !strings.Contains(string(ev.Data), `"action":"remove"`) || !strings.Contains(string(ev.Data), devices[0].ID) {
		t.Errorf("the remove event: %s", ev.Data)
	}
	if after, _ := c.ListDevices(ctx); len(after) != len(devices)-1 {
		t.Errorf("%d devices after the removal, want %d", len(after), len(devices)-1)
	}
}
