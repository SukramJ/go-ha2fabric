// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package camera

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/jpeg"
	"log/slog"
	"testing"

	csdef "thingfabric.dev/go-fabric/cluster/spec/cameraavstreammanagement"
	wpdef "thingfabric.dev/go-fabric/cluster/spec/webrtctransportprovider"

	"github.com/SukramJ/go-ha2fabric/internal/ha"
)

// FuzzCandidateFromHA holds the translation of Home Assistant's candidate
// events (camera/webrtc/offer's "candidate", fromHA) against any event
// JSON: it never panics; an accepted candidate keeps the candidate text,
// a null sdpMid exactly when Home Assistant sent none or "", and the
// sdpMLineIndex sent; and toHA gives back what fromHA took.
func FuzzCandidateFromHA(f *testing.F) {
	for _, s := range []string{
		`{"type":"candidate","candidate":{"candidate":"candidate:1 1 udp 2122260223 10.0.0.2 51000 typ host","sdpMid":"0","sdpMLineIndex":0}}`,
		`{"type":"candidate","candidate":{"candidate":"c","sdpMLineIndex":65535}}`,
		`{"type":"candidate","candidate":{"candidate":"c","sdpMLineIndex":65536}}`,
		`{"type":"candidate","candidate":{"candidate":"c","sdpMid":"","sdpMLineIndex":-1}}`,
		`{"type":"candidate","candidate":{"candidate":""}}`,
		`{"type":"candidate","candidate":null}`, `{"type":"candidate","candidate":{"sdpMLineIndex":1e3}}`,
	} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		var ev ha.WebRTCEvent
		if json.Unmarshal(b, &ev) != nil || ev.Candidate == nil {
			return
		}
		in := *ev.Candidate
		out, ok := fromHA(in)
		if !ok {
			if in.SdpMLineIndex == nil || (*in.SdpMLineIndex >= 0 && *in.SdpMLineIndex <= 0xFFFF) {
				t.Fatalf("refused %+v", in)
			}
			return
		}
		if out.Candidate != in.Candidate || out.SdpMid.Null != (in.SdpMid == nil || *in.SdpMid == "") ||
			(!out.SdpMid.Null && out.SdpMid.Value != *in.SdpMid) || out.SdpmLineIndex.Null != (in.SdpMLineIndex == nil) ||
			(in.SdpMLineIndex != nil && int(out.SdpmLineIndex.Value) != *in.SdpMLineIndex) {
			t.Fatalf("fromHA(%+v) = %+v", in, out)
		}
		back := toHA(out)
		again, ok := fromHA(back)
		if !ok || again != out {
			t.Fatalf("toHA(%+v) = %+v, which fromHA reads as %+v, %v", out, back, again, ok)
		}
	})
}

// FuzzCandidateToHA holds the translation of a controller's candidate
// (ProvideIceCandidates, toHA) against any value: it never panics, and the
// JSON Home Assistant receives decodes to the same candidate.
func FuzzCandidateToHA(f *testing.F) {
	f.Add("candidate:1 1 udp 2122260223 10.0.0.2 51000 typ host", "0", false, uint16(0), false)
	f.Add("", "", true, uint16(65535), true)
	f.Add("\x00\xff", " ", false, uint16(7), false)
	f.Fuzz(func(t *testing.T, cand, mid string, midNull bool, idx uint16, idxNull bool) {
		in := wpdef.ICECandidateStruct{Candidate: cand}
		in.SdpMid.Null, in.SdpMid.Value = midNull, mid
		in.SdpmLineIndex.Null, in.SdpmLineIndex.Value = idxNull, idx
		got := toHA(in)
		b, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		var dec ha.ICECandidate
		if err := json.Unmarshal(b, &dec); err != nil {
			t.Fatalf("%s: %v", b, err)
		}
		if (dec.SdpMid == nil) != midNull || (dec.SdpMLineIndex == nil) != idxNull ||
			(!idxNull && *dec.SdpMLineIndex != int(idx)) {
			t.Fatalf("toHA(%+v) sends %s", in, b)
		}
	})
}

// stillUpstream serves one still.
type stillUpstream struct{ data []byte }

func (stillUpstream) Subscribe(context.Context, string, string) (Stream, error) {
	return nil, errors.New("no stream")
}

func (stillUpstream) Candidate(context.Context, string, string, ha.ICECandidate) error { return nil }

func (u stillUpstream) Image(context.Context, string, int, int) (data []byte, contentType string, err error) {
	return u.data, "image/jpeg", nil
}

// FuzzCaptureSnapshot holds the still's check (CaptureSnapshot) against
// any body Home Assistant's camera proxy returns as image/jpeg: it never
// panics, and an accepted still is a JPEG whose stated size is the one
// its header declares, within 1..65535 on each side.
func FuzzCaptureSnapshot(f *testing.F) {
	for _, wh := range [][2]int{{320, 240}, {1, 1}, {17, 3}} {
		var b bytes.Buffer
		if err := jpeg.Encode(&b, image.NewGray(image.Rect(0, 0, wh[0], wh[1])), nil); err != nil {
			f.Fatal(err)
		}
		f.Add(b.Bytes())
	}
	f.Add([]byte("not a jpeg"))
	f.Add([]byte("\xff\xd8\xff\xc0\x00\x11\x08\xff\xff\xff\xff\x03"))
	f.Add([]byte{})
	h := New("camera.fuzz", nil, Options{Logger: slog.New(slog.DiscardHandler)})
	f.Fuzz(func(t *testing.T, b []byte) {
		h.up = stillUpstream{data: b}
		got, err := h.CaptureSnapshot(context.Background(), nil, csdef.VideoResolutionStruct{Width: 640, Height: 480})
		if err != nil {
			return
		}
		cfg, format, derr := image.DecodeConfig(bytes.NewReader(b))
		if derr != nil || format != "jpeg" || got.ImageCodec != csdef.ImageCodecJpeg || !bytes.Equal(got.Data, b) ||
			int(got.Resolution.Width) != cfg.Width || int(got.Resolution.Height) != cfg.Height || got.Resolution.Width == 0 || got.Resolution.Height == 0 {
			t.Fatalf("accepted %d bytes as %+v", len(b), got.Resolution)
		}
	})
}
