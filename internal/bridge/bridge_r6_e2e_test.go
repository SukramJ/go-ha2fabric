// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package bridge_test

import (
	"bytes"
	"image"
	"image/jpeg"
	"testing"

	"github.com/SukramJ/go-fabric/cluster/spec"
	csdef "github.com/SukramJ/go-fabric/cluster/spec/cameraavstreammanagement"
	wpdef "github.com/SukramJ/go-fabric/cluster/spec/webrtctransportprovider"
	"github.com/SukramJ/go-fabric/controller/client"
	"github.com/SukramJ/go-fabric/fabrictest"

	"github.com/SukramJ/go-ha2fabric/internal/bridge"
	"github.com/SukramJ/go-ha2fabric/internal/expose"
	"github.com/SukramJ/go-ha2fabric/internal/ha/hatest"
)

const cameraDeviceType = 0x0142 // go-fabric device/camera_gen.go:20-24 at v0.15.5

func (e *env) verdict(entityID string) bridge.Verdict {
	e.t.Helper()
	plan := e.br.Plan()
	for i := range plan {
		if plan[i].EntityID == entityID {
			return plan[i]
		}
	}
	e.t.Fatalf("no verdict for %s", entityID)
	return bridge.Verdict{}
}

func (e *env) sessions(ep uint16) []wpdef.WebRTCSessionStruct {
	e.t.Helper()
	vals, err := fabrictest.ReadCluster(e.ctx, e.pair, ep, wpdef.ClusterID)
	if err != nil {
		e.t.Fatal(err)
	}
	st, _ := wpdef.StateFrom(vals)
	return st.CurrentSessions
}

// TestCameraEndToEnd drives a camera through a commissioned controller:
// the Camera endpoint, a stream allocation, ProvideOffer to
// camera/webrtc/offer, the controller's candidates to
// camera/webrtc/candidate with Home Assistant's session id, EndSession to
// unsubscribe_events, and CaptureSnapshot from the camera proxy. The
// controller of fabrictest has no WebRtcTransportRequestor, so the
// camera's Answer and IceCandidates reaching a requestor are held by
// internal/camera's tests and by go-fabric's
// node/webrtc_provider_peers_test.go; here a scripted answer shows the
// signal goes out and, refused, ends the session.
func TestCameraEndToEnd(t *testing.T) {
	e := newEnv(t, expose.Rule{Assist: true}, func(o *bridge.Options) { o.Camera = true })
	e.commission()
	cam := e.mustEndpoint("camera.front_door")
	if v := e.verdict("camera.front_door"); v.DeviceType != cameraDeviceType || v.Row != "camera/camera" {
		t.Fatalf("camera verdict %+v", v)
	}
	if v := e.verdict("camera.doorbell_still"); v.Endpoint != 0 || v.State != "unmappable" {
		t.Fatalf("a camera without STREAM %+v", v)
	}

	invoke := func(cluster, cmd uint32, fields spec.Encodable) fabrictest.Result {
		t.Helper()
		res, err := fabrictest.Invoke(e.ctx, e.pair, client.CommandAt(cam, cluster, cmd), fields)
		if err != nil {
			t.Fatalf("command 0x%02x of cluster 0x%04x: %v", cmd, cluster, err)
		}
		return res
	}
	res := invoke(csdef.ClusterID, csdef.CmdVideoStreamAllocate, csdef.VideoStreamAllocateRequest{
		StreamUsage: csdef.StreamUsageLiveView, VideoCodec: csdef.VideoCodecH264, MinFrameRate: 15, MaxFrameRate: 30,
		MinResolution: csdef.VideoResolutionStruct{Width: 640, Height: 480}, MaxResolution: csdef.VideoResolutionStruct{Width: 1920, Height: 1080},
		MinBitRate: 10000, MaxBitRate: 2000000, KeyFrameInterval: 4000,
	})
	var video csdef.VideoStreamAllocateResponse
	if err := video.DecodeTLV(res.Fields); err != nil || video.VideoStreamId != 1 {
		t.Fatalf("VideoStreamAllocate %+v %v", video, err)
	}

	e.ha.SetWebRTCScript(func(o hatest.Offer) []map[string]any { return []map[string]any{hatest.SessionEvent(o.SessionID)} })
	usage := wpdef.StreamUsageLiveView
	origin := uint16(1)
	offer := "v=0\r\no=- 1 1 IN IP4 192.0.2.1\r\ns=controller\r\n"
	res = invoke(wpdef.ClusterID, wpdef.CmdProvideOffer, wpdef.ProvideOfferRequest{
		WebRtcSessionId: spec.NullOf[uint16](), Sdp: offer, StreamUsage: &usage, OriginatingEndpointId: &origin,
		VideoStreams: &[]uint16{video.VideoStreamId},
	})
	var po wpdef.ProvideOfferResponse
	if err := po.DecodeTLV(res.Fields); err != nil {
		t.Fatal(err)
	}
	if o := e.ha.Offers(); len(o) != 1 || o[0].EntityID != "camera.front_door" || o[0].SDP != offer {
		t.Fatalf("offers %+v", o)
	}
	if s := e.sessions(cam); len(s) != 1 || s[0].Id != po.WebRtcSessionId || s[0].PeerEndpointId != origin {
		t.Fatalf("CurrentSessions %+v", s)
	}

	res = invoke(wpdef.ClusterID, wpdef.CmdProvideIceCandidates, wpdef.ProvideIceCandidatesRequest{
		WebRtcSessionId: po.WebRtcSessionId,
		IceCandidates:   []wpdef.ICECandidateStruct{{Candidate: "candidate:9 1 UDP 1 192.0.2.1 5000 typ host", SdpMid: spec.ValueOf("0"), SdpmLineIndex: spec.ValueOf[uint16](0)}},
	})
	_ = res
	if err := e.ha.Await(e.ctx, "the controller's candidate", func() bool { return len(e.ha.Candidates()) == 1 }); err != nil {
		t.Fatal(err)
	}
	if c := e.ha.Candidates()[0]; c.SessionID != e.ha.Offers()[0].SessionID || c.Candidate["candidate"] != "candidate:9 1 UDP 1 192.0.2.1 5000 typ host" || c.Candidate["sdpMid"] != "0" {
		t.Fatalf("candidate %+v", c)
	}

	invoke(wpdef.ClusterID, wpdef.CmdEndSession, wpdef.EndSessionRequest{WebRtcSessionId: po.WebRtcSessionId, Reason: 2})
	if u := e.ha.Unsubscribed(); len(u) != 1 || u[0] != e.ha.Offers()[0].ID {
		t.Fatalf("unsubscribed %v", u)
	}
	if s := e.sessions(cam); len(s) != 0 {
		t.Fatalf("CurrentSessions after EndSession %+v", s)
	}

	// Home Assistant's answer goes to the controller, whose node serves no
	// requestor: the refused signal ends the session.
	e.ha.SetWebRTCScript(nil)
	res = invoke(wpdef.ClusterID, wpdef.CmdProvideOffer, wpdef.ProvideOfferRequest{
		WebRtcSessionId: spec.NullOf[uint16](), Sdp: offer, StreamUsage: &usage, OriginatingEndpointId: &origin,
		VideoStreams: &[]uint16{video.VideoStreamId},
	})
	if err := po.DecodeTLV(res.Fields); err != nil {
		t.Fatal(err)
	}
	e.eventually("the refused answer ends the session", func() bool {
		return len(e.sessions(cam)) == 0 && len(e.ha.Unsubscribed()) == 2
	})

	res = invoke(csdef.ClusterID, csdef.CmdSnapshotStreamAllocate, csdef.SnapshotStreamAllocateRequest{
		ImageCodec: csdef.ImageCodecJpeg, MaxFrameRate: 30, Quality: 90,
		MinResolution: csdef.VideoResolutionStruct{Width: 320, Height: 240}, MaxResolution: csdef.VideoResolutionStruct{Width: 1920, Height: 1080},
	})
	var snap csdef.SnapshotStreamAllocateResponse
	if err := snap.DecodeTLV(res.Fields); err != nil || snap.SnapshotStreamId != 1 {
		t.Fatalf("SnapshotStreamAllocate %+v %v", snap, err)
	}
	var jp bytes.Buffer
	if err := jpeg.Encode(&jp, image.NewGray(image.Rect(0, 0, 160, 120)), nil); err != nil {
		t.Fatal(err)
	}
	e.ha.SetImage("camera.front_door", jp.Bytes())
	res = invoke(csdef.ClusterID, csdef.CmdCaptureSnapshot, csdef.CaptureSnapshotRequest{
		SnapshotStreamId: spec.ValueOf(snap.SnapshotStreamId), RequestedResolution: csdef.VideoResolutionStruct{Width: 640, Height: 480},
	})
	var shot csdef.CaptureSnapshotResponse
	if err := shot.DecodeTLV(res.Fields); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(shot.Data, jp.Bytes()) || shot.ImageCodec != csdef.ImageCodecJpeg || shot.Resolution.Width != 160 || shot.Resolution.Height != 120 {
		t.Fatalf("snapshot %d bytes %v %+v", len(shot.Data), shot.ImageCodec, shot.Resolution)
	}
	if r := e.ha.ImageRequests(); len(r) != 1 || r[0].Width != "640" || r[0].Height != "480" {
		t.Fatalf("camera proxy requests %+v", r)
	}
}

// TestCameraOffEndToEnd: with the option off (the default) no camera
// endpoint exists, and the verdict says why.
func TestCameraOffEndToEnd(t *testing.T) {
	e := newEnv(t, expose.Rule{Assist: true})
	for _, id := range []string{"camera.front_door", "camera.doorbell_still"} {
		if v := e.verdict(id); v.Endpoint != 0 || v.State != "unmappable" || v.Row != "" {
			t.Fatalf("%s with the camera off %+v", id, v)
		}
	}
}
