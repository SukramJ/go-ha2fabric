// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

// Package camera is the host of one Home Assistant camera on go-fabric's
// camera kit: the ports of the Camera device type's
// WebRtcTransportProvider and CameraAvStreamManagement servers, which
// go-hamapping's camera row declares as the consumer's (`ports: host`).
//
// The host relays signalling only. Home Assistant's WebRTC provider —
// go2rtc, or a camera integration with native WebRTC — produces the
// stream and runs the peer connection with the controller; the host
// carries the controller's offer and candidates to camera/webrtc/offer and
// camera/webrtc/candidate, and Home Assistant's answer and candidates back
// to the controller's WebRtcTransportRequestor through the kit's
// *camera.Provider signals (go-fabric docs/guides/camera.md:84-87 at
// v0.15.5). It never touches the media: the video, audio and snapshot
// streams it allocates are one nominal stream each, and a snapshot is
// Home Assistant's camera proxy still.
//
// It is the one package that imports go-fabric's cluster/camera — the
// signals are methods of *camera.Provider — and go-fabric's bridge for the
// one error a signal over UDP returns, bridge.ErrPayloadTooLarge
// (docs/guides/camera.md:121-128).
package camera

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // image.DecodeConfig reads the still's size
	"log/slog"
	"math"
	"sync"
	"time"

	gfbridge "github.com/SukramJ/go-fabric/bridge"
	gfcamera "github.com/SukramJ/go-fabric/cluster/camera"
	csdef "github.com/SukramJ/go-fabric/cluster/spec/cameraavstreammanagement"
	wpdef "github.com/SukramJ/go-fabric/cluster/spec/webrtctransportprovider"
	"github.com/SukramJ/go-fabric/contract"

	"github.com/SukramJ/go-ha2fabric/internal/ha"
)

// End reasons, WebRTCEndReasonEnum (connectedhomeip
// data_model/1.7/globals/Enums.xml:225-265 at 1dc3b69ed5e; go-fabric
// carries the field as a uint8 and checks only the highest value,
// cluster/camera/provider.go:587-588 at v0.15.5).
const (
	// ReasonICEFailed (0, "No media connection could be established to
	// the other party", Enums.xml:226) ends a session Home Assistant
	// reported an error for, or whose signal did not reach the
	// controller.
	ReasonICEFailed uint8 = 0
	// ReasonUnknown (12, "Unknown or unspecified reason", Enums.xml:262)
	// ends the sessions of a camera that went away: the entity left the
	// bridge, or the connection to Home Assistant dropped. The enum has
	// no value closer to it.
	ReasonUnknown uint8 = 12
)

// NominalStreamID is the id of the host's one video, audio and snapshot
// stream.
const NominalStreamID uint16 = 1

// statusInvalidInState is the IM status INVALID_IN_STATE (0xCB, Matter
// Core §8.10.1; go-fabric im/status.go:94 at v0.15.5).
const statusInvalidInState uint8 = 0xCB

// Errors of the host.
var (
	// ErrSolicitOffer refuses SolicitOffer: Home Assistant's WebRTC API
	// takes the client's offer only (camera/webrtc.py:252-258 at
	// 2026.10.3, "offer" required).
	ErrSolicitOffer = errors.New("camera: Home Assistant takes the controller's offer only (ProvideOffer)")
	// ErrProvideAnswer refuses ProvideAnswer: without a solicited offer
	// there is no answer to take.
	ErrProvideAnswer = errors.New("camera: no solicited offer awaits an answer")
	// ErrUnknownStream is a stream id other than NominalStreamID.
	ErrUnknownStream = errors.New("camera: no stream of the id")
	// ErrNotJPEG is a still whose content type is not image/jpeg.
	ErrNotJPEG = errors.New("camera: the still is not a JPEG")
	// ErrClosed is an offer to a host that was closed.
	ErrClosed = errors.New("camera: the host is closed")
)

// Stream is one camera/webrtc/offer subscription (*ha.Subscription).
type Stream interface {
	Events() <-chan json.RawMessage
	Err() error
	Unsubscribe(ctx context.Context) error
}

// Upstream is what the host asks of Home Assistant.
type Upstream interface {
	// Subscribe is camera/webrtc/offer.
	Subscribe(ctx context.Context, entityID, offer string) (Stream, error)
	// Candidate is camera/webrtc/candidate.
	Candidate(ctx context.Context, entityID, sessionID string, candidate ha.ICECandidate) error
	// Image is the camera proxy's still.
	Image(ctx context.Context, entityID string, width, height int) (data []byte, contentType string, err error)
}

// HA is the Upstream of a Home Assistant client.
func HA(c *ha.Client) Upstream { return client{c} }

type client struct{ c *ha.Client }

func (u client) Subscribe(ctx context.Context, entityID, offer string) (Stream, error) {
	return u.c.WebRTCOffer(ctx, entityID, offer)
}

func (u client) Candidate(ctx context.Context, entityID, sessionID string, candidate ha.ICECandidate) error {
	return u.c.WebRTCCandidate(ctx, entityID, sessionID, candidate)
}

func (u client) Image(ctx context.Context, entityID string, width, height int) (data []byte, contentType string, err error) {
	return u.c.Image(ctx, entityID, width, height)
}

// Signaller is the controller side of a session: *camera.Provider's
// signals (cluster/camera/provider_peer.go:84-120 at v0.15.5).
type Signaller interface {
	Answer(ctx context.Context, sessionID uint16, sdp string) error
	IceCandidates(ctx context.Context, sessionID uint16, candidates []gfcamera.ProviderICECandidate) error
	End(ctx context.Context, sessionID uint16, reason uint8) error
}

var _ Signaller = (*gfcamera.Provider)(nil)

// Options configure a [Host].
type Options struct {
	// Logger is the host's logger; nil is slog.Default().
	Logger *slog.Logger
	// Retries and RetryInterval bound the wait for a session the kit has
	// not stored yet — it stores the session after ProvideOffer returned
	// (cluster/camera/provider.go:522-526 at v0.15.5), and Home Assistant
	// may answer before — and for the signaller before [Host.Attach].
	// 0 is 10 and 50 ms.
	Retries       int
	RetryInterval time.Duration
	// SignalTimeout bounds one signal and one Home Assistant command; 0
	// is 10 s.
	SignalTimeout time.Duration
}

// Host serves one camera entity's ports. Its methods are safe for
// concurrent use.
type Host struct {
	entity string
	up     Upstream
	opts   Options
	log    *slog.Logger

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	ready  chan struct{}

	mu       sync.Mutex
	sig      Signaller
	sessions map[uint16]*session
	closed   bool
}

// session is one WebRTC session: the controller's id and Home Assistant's.
type session struct {
	id     uint16
	stream Stream
	// haID is Home Assistant's session id, from its session event;
	// pending holds the controller's candidates until it arrived.
	haID    string
	pending []ha.ICECandidate
}

// New returns the host of an entity.
func New(entityID string, up Upstream, opts Options) *Host {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.Retries <= 0 {
		opts.Retries = 10
	}
	if opts.RetryInterval <= 0 {
		opts.RetryInterval = 50 * time.Millisecond
	}
	if opts.SignalTimeout <= 0 {
		opts.SignalTimeout = 10 * time.Second
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Host{
		entity: entityID, up: up, opts: opts,
		log: opts.Logger.With(slog.String("entity_id", entityID)),
		ctx: ctx, cancel: cancel, ready: make(chan struct{}),
		sessions: map[uint16]*session{},
	}
}

// Entity is the host's entity id.
func (h *Host) Entity() string { return h.entity }

// Attach sets the signaller, the *camera.Provider of the device the plan
// built. A different signaller — a rebuilt device — drops the sessions of
// the previous one: the controller held them with that provider.
func (h *Host) Attach(s Signaller) {
	h.mu.Lock()
	prev := h.sig
	h.sig = s
	var stale []*session
	if prev != nil && prev != s {
		for id, ss := range h.sessions {
			delete(h.sessions, id)
			stale = append(stale, ss)
		}
	}
	select {
	case <-h.ready:
	default:
		close(h.ready)
	}
	h.mu.Unlock()
	for _, ss := range stale {
		h.unsubscribe(h.ctx, ss)
	}
}

// Close ends every session with ReasonUnknown and stops the host.
func (h *Host) Close() {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	h.closed = true
	sig := h.sig
	all := make([]*session, 0, len(h.sessions))
	for id, ss := range h.sessions {
		delete(h.sessions, id)
		all = append(all, ss)
	}
	h.mu.Unlock()
	for _, ss := range all {
		h.unsubscribe(h.ctx, ss)
		if sig != nil {
			ctx, cancel := context.WithTimeout(context.Background(), h.opts.SignalTimeout)
			if err := sig.End(ctx, ss.id, ReasonUnknown); err != nil {
				h.log.Debug("camera.end", slog.Int("session", int(ss.id)), slog.String("error", err.Error()))
			}
			cancel()
		}
	}
	h.cancel()
	h.wg.Wait()
}

// SolicitOffer implements wpdef.ProviderPort: not served, since Home
// Assistant's API takes the client's offer only. The error carries
// INVALID_IN_STATE; the kit's server answers FAILURE for it today, its
// portError keeping only an im.StatusCodeError (cluster/camera/chime.go:
// 293-299 at v0.15.5).
func (h *Host) SolicitOffer(context.Context, wpdef.OfferArgs) (wpdef.WebRTCSessionStruct, bool, error) {
	return wpdef.WebRTCSessionStruct{}, false, contract.StatusError{Status: statusInvalidInState, Message: ErrSolicitOffer.Error()}
}

// ProvideAnswer implements wpdef.ProviderPort: never expected.
func (h *Host) ProvideAnswer(context.Context, uint16, string) error { return ErrProvideAnswer }

// ProvideOffer implements wpdef.ProviderPort: the controller's offer goes
// to camera/webrtc/offer, and the session the kit stores is returned. A
// re-offer of a session the host holds replaces its Home Assistant
// session.
func (h *Host) ProvideOffer(ctx context.Context, args wpdef.OfferArgs) (wpdef.WebRTCSessionStruct, error) {
	h.mu.Lock()
	closed := h.closed
	old := h.sessions[args.SessionID]
	delete(h.sessions, args.SessionID)
	h.mu.Unlock()
	if closed {
		return wpdef.WebRTCSessionStruct{}, ErrClosed
	}
	if old != nil {
		h.unsubscribe(ctx, old)
	}
	stream, err := h.up.Subscribe(ctx, h.entity, args.Sdp)
	if err != nil {
		h.log.Warn("camera.offer_failed", slog.Int("session", int(args.SessionID)), slog.String("error", err.Error()))
		return wpdef.WebRTCSessionStruct{}, err
	}
	ss := &session{id: args.SessionID, stream: stream}
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		h.unsubscribe(ctx, ss)
		return wpdef.WebRTCSessionStruct{}, ErrClosed
	}
	h.sessions[ss.id] = ss
	h.wg.Add(1)
	h.mu.Unlock()
	go h.relay(h.ctx, ss) //nolint:contextcheck // the relay outlives the controller's request; the host's context bounds it

	out := wpdef.WebRTCSessionStruct{
		Id: args.SessionID, PeerNodeId: args.PeerNodeID, PeerEndpointId: args.OriginatingEndpointID,
		StreamUsage: args.StreamUsage, VideoStreams: streams(args.VideoStreams), AudioStreams: streams(args.AudioStreams),
		FabricIndex: args.FabricIndex,
	}
	if args.MetadataEnabled != nil {
		out.MetadataEnabled = *args.MetadataEnabled
	}
	h.log.Info("camera.session", slog.Int("session", int(ss.id)), slog.Uint64("peer_node", args.PeerNodeID))
	return out, nil
}

// streams is the session's stream list: the first stream of the
// validated list, the nominal stream when the list is empty, none when
// the offer named no stream of the kind.
func streams(l *[]uint16) *[]uint16 {
	if l == nil {
		return nil
	}
	if len(*l) > 0 {
		return &[]uint16{(*l)[0]}
	}
	return &[]uint16{NominalStreamID}
}

// ProvideIceCandidates implements wpdef.ProviderPort: each candidate goes
// to camera/webrtc/candidate with Home Assistant's session id; before
// Home Assistant named it, the candidates wait.
func (h *Host) ProvideIceCandidates(ctx context.Context, sessionID uint16, candidates []wpdef.ICECandidateStruct) error {
	h.mu.Lock()
	ss := h.sessions[sessionID]
	if ss == nil {
		h.mu.Unlock()
		return fmt.Errorf("%w: %d", gfcamera.ErrUnknownSession, sessionID)
	}
	out := make([]ha.ICECandidate, len(candidates))
	for i, c := range candidates {
		out[i] = toHA(c)
	}
	haID := ss.haID
	if haID == "" {
		ss.pending = append(ss.pending, out...)
		h.mu.Unlock()
		return nil
	}
	h.mu.Unlock()
	for _, c := range out {
		if err := h.up.Candidate(ctx, h.entity, haID, c); err != nil {
			return err
		}
	}
	return nil
}

// EndSession implements wpdef.ProviderPort: Home Assistant's session is
// closed by unsubscribing the offer.
func (h *Host) EndSession(ctx context.Context, sessionID uint16, reason uint8) error {
	h.mu.Lock()
	ss := h.sessions[sessionID]
	delete(h.sessions, sessionID)
	h.mu.Unlock()
	if ss != nil {
		h.log.Info("camera.session_ended", slog.Int("session", int(sessionID)), slog.Int("reason", int(reason)))
		h.unsubscribe(ctx, ss)
	}
	return nil
}

func toHA(c wpdef.ICECandidateStruct) ha.ICECandidate {
	out := ha.ICECandidate{Candidate: c.Candidate}
	if !c.SdpMid.Null {
		mid := c.SdpMid.Value
		out.SdpMid = &mid
	}
	if !c.SdpmLineIndex.Null {
		idx := int(c.SdpmLineIndex.Value)
		out.SdpMLineIndex = &idx
	}
	return out
}

func fromHA(c ha.ICECandidate) (gfcamera.ProviderICECandidate, bool) {
	var out gfcamera.ProviderICECandidate
	out.Candidate = c.Candidate
	out.SdpMid.Null = c.SdpMid == nil || *c.SdpMid == ""
	if !out.SdpMid.Null {
		out.SdpMid.Value = *c.SdpMid
	}
	out.SdpmLineIndex.Null = c.SdpMLineIndex == nil
	if c.SdpMLineIndex != nil {
		if *c.SdpMLineIndex < 0 || *c.SdpMLineIndex > math.MaxUint16 {
			return out, false
		}
		out.SdpmLineIndex.Value = uint16(*c.SdpMLineIndex)
	}
	return out, true
}

// unsubscribe closes Home Assistant's session of ss, also when ctx ended:
// the session must not outlive the request that ends it.
func (h *Host) unsubscribe(ctx context.Context, ss *session) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), h.opts.SignalTimeout)
	defer cancel()
	if err := ss.stream.Unsubscribe(ctx); err != nil {
		h.log.Debug("camera.unsubscribe", slog.Int("session", int(ss.id)), slog.String("error", err.Error()))
	}
}

// own reports whether ss is still the host's session of its id.
func (h *Host) own(ss *session) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.sessions[ss.id] == ss
}

// relay carries one session's Home Assistant events to the controller,
// in their order.
func (h *Host) relay(ctx context.Context, ss *session) {
	defer h.wg.Done()
	for raw := range ss.stream.Events() {
		var ev ha.WebRTCEvent
		if err := json.Unmarshal(raw, &ev); err != nil {
			h.log.Warn("camera.event_refused", slog.Int("session", int(ss.id)), slog.String("error", err.Error()))
			continue
		}
		if !h.event(ctx, ss, ev) {
			return
		}
	}
	// The subscription ended: unsubscribed by the host, or the
	// connection dropped (Err set) — then the session went with it.
	if err := ss.stream.Err(); err != nil && h.own(ss) {
		h.log.Info("camera.session_lost", slog.Int("session", int(ss.id)), slog.String("error", err.Error()))
		h.end(ctx, ss, ReasonUnknown)
	}
}

// event handles one event; false ends the relay.
func (h *Host) event(ctx context.Context, ss *session, ev ha.WebRTCEvent) bool {
	switch ev.Type {
	case ha.WebRTCSession:
		h.mu.Lock()
		ss.haID = ev.SessionID
		pending := ss.pending
		ss.pending = nil
		h.mu.Unlock()
		for _, c := range pending {
			cctx, cancel := context.WithTimeout(ctx, h.opts.SignalTimeout)
			err := h.up.Candidate(cctx, h.entity, ev.SessionID, c)
			cancel()
			if err != nil {
				h.log.Warn("camera.candidate_failed", slog.Int("session", int(ss.id)), slog.String("error", err.Error()))
			}
		}
	case ha.WebRTCAnswer:
		return h.signal(ctx, ss, "Answer", func(ctx context.Context, s Signaller) error { return s.Answer(ctx, ss.id, ev.Answer) })
	case ha.WebRTCCandidate:
		if ev.Candidate == nil || ev.Candidate.Candidate == "" {
			// The end-of-candidates indication (webrtc_models/__init__.py:
			// 89-92 at 0.3.0) has no counterpart in ICECandidates.
			return true
		}
		c, ok := fromHA(*ev.Candidate)
		if !ok {
			h.log.Warn("camera.candidate_refused", slog.Int("session", int(ss.id)), slog.Int("sdp_mline_index", *ev.Candidate.SdpMLineIndex))
			return true
		}
		return h.signal(ctx, ss, "IceCandidates", func(ctx context.Context, s Signaller) error {
			return s.IceCandidates(ctx, ss.id, []gfcamera.ProviderICECandidate{c})
		})
	case ha.WebRTCError:
		h.log.Warn("camera.ha_error", slog.Int("session", int(ss.id)), slog.String("code", ev.Code), slog.String("message", ev.Message))
		h.end(ctx, ss, ReasonICEFailed)
		return false
	default:
		h.log.Debug("camera.event_ignored", slog.Int("session", int(ss.id)), slog.String("type", ev.Type))
	}
	return true
}

// signaller waits for Attach up to the retry bound.
func (h *Host) signaller(ctx context.Context) Signaller {
	t := time.NewTimer(time.Duration(h.opts.Retries) * h.opts.RetryInterval)
	defer t.Stop()
	select {
	case <-h.ready:
	case <-t.C:
	case <-ctx.Done():
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.sig
}

// signal sends one signal, retrying while the kit does not hold the
// session yet; a failure is logged once and ends the session. false ends
// the relay.
func (h *Host) signal(ctx context.Context, ss *session, command string, fn func(context.Context, Signaller) error) bool {
	s := h.signaller(ctx)
	if s == nil {
		h.log.Error("camera.signal_failed", slog.Int("session", int(ss.id)), slog.String("command", command), slog.String("error", "no WebRtcTransportProvider attached"))
		h.drop(ctx, ss)
		return false
	}
	var err error
	for attempt := 0; ; attempt++ {
		if !h.own(ss) {
			return false
		}
		sctx, cancel := context.WithTimeout(ctx, h.opts.SignalTimeout)
		err = fn(sctx, s)
		cancel()
		if err == nil {
			return true
		}
		if !errors.Is(err, gfcamera.ErrUnknownSession) || attempt+1 >= h.opts.Retries {
			break
		}
		select {
		case <-time.After(h.opts.RetryInterval):
		case <-ctx.Done():
			return false
		}
	}
	if errors.Is(err, gfbridge.ErrPayloadTooLarge) {
		h.log.Error("camera.signal_too_large", slog.Int("session", int(ss.id)), slog.String("command", command), slog.String("error", err.Error()),
			slog.String("hint", "the controller reached the bridge over UDP; a signal carrying an SDP needs the controller's TCP session"))
	} else {
		h.log.Warn("camera.signal_failed", slog.Int("session", int(ss.id)), slog.String("command", command), slog.String("error", err.Error()))
	}
	h.end(ctx, ss, ReasonICEFailed)
	return false
}

// end drops the session and tells the controller.
func (h *Host) end(ctx context.Context, ss *session, reason uint8) {
	if !h.drop(ctx, ss) {
		return
	}
	h.mu.Lock()
	s := h.sig
	h.mu.Unlock()
	if s == nil {
		return
	}
	ectx, cancel := context.WithTimeout(context.WithoutCancel(ctx), h.opts.SignalTimeout)
	defer cancel()
	if err := s.End(ectx, ss.id, reason); err != nil {
		h.log.Debug("camera.end", slog.Int("session", int(ss.id)), slog.String("error", err.Error()))
	}
}

// drop removes the session and closes Home Assistant's; false when it
// was gone already.
func (h *Host) drop(ctx context.Context, ss *session) bool {
	h.mu.Lock()
	ours := h.sessions[ss.id] == ss
	if ours {
		delete(h.sessions, ss.id)
	}
	h.mu.Unlock()
	if ours {
		h.unsubscribe(ctx, ss)
	}
	return ours
}

// VideoStreamAllocate implements csdef.VideoStreamPort: the nominal stream.
func (h *Host) VideoStreamAllocate(context.Context, csdef.VideoStreamStruct) (uint16, error) {
	return NominalStreamID, nil
}

// VideoStreamAllocated implements csdef.VideoStreamPort: nothing to start.
func (h *Host) VideoStreamAllocated(context.Context, csdef.VideoStreamStruct, csdef.StreamAllocationAction) {
}

// VideoStreamModify implements csdef.VideoStreamPort.
func (h *Host) VideoStreamModify(_ context.Context, id uint16, _, _ *bool) error { return nominal(id) }

// VideoStreamDeallocate implements csdef.VideoStreamPort.
func (h *Host) VideoStreamDeallocate(_ context.Context, id uint16) error { return nominal(id) }

// AudioStreamAllocate implements csdef.AudioStreamPort: the nominal stream.
func (h *Host) AudioStreamAllocate(context.Context, csdef.AudioStreamStruct) (uint16, error) {
	return NominalStreamID, nil
}

// AudioStreamDeallocate implements csdef.AudioStreamPort.
func (h *Host) AudioStreamDeallocate(_ context.Context, id uint16) error { return nominal(id) }

// SnapshotStreamAllocate implements csdef.SnapshotStreamPort: the nominal
// stream.
func (h *Host) SnapshotStreamAllocate(context.Context, csdef.SnapshotStreamStruct) (uint16, error) {
	return NominalStreamID, nil
}

// SnapshotStreamModify implements csdef.SnapshotStreamPort.
func (h *Host) SnapshotStreamModify(_ context.Context, id uint16, _, _ *bool) error {
	return nominal(id)
}

// SnapshotStreamDeallocate implements csdef.SnapshotStreamPort.
func (h *Host) SnapshotStreamDeallocate(_ context.Context, id uint16) error { return nominal(id) }

func nominal(id uint16) error {
	if id != NominalStreamID {
		return fmt.Errorf("%w: %d", ErrUnknownStream, id)
	}
	return nil
}

// CaptureSnapshot implements csdef.SnapshotCapturePort: Home Assistant's
// camera proxy still at the requested resolution, which Home Assistant
// scales down only; the response names the size the JPEG has.
func (h *Host) CaptureSnapshot(ctx context.Context, _ *uint16, res csdef.VideoResolutionStruct) (csdef.CaptureSnapshotResponse, error) {
	data, ct, err := h.up.Image(ctx, h.entity, int(res.Width), int(res.Height))
	if err != nil {
		h.log.Warn("camera.snapshot_failed", slog.String("error", err.Error()))
		return csdef.CaptureSnapshotResponse{}, err
	}
	if ct != "image/jpeg" {
		return csdef.CaptureSnapshotResponse{}, fmt.Errorf("%w: %q", ErrNotJPEG, ct)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || format != "jpeg" {
		return csdef.CaptureSnapshotResponse{}, fmt.Errorf("%w: %w", ErrNotJPEG, err)
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > math.MaxUint16 || cfg.Height > math.MaxUint16 {
		return csdef.CaptureSnapshotResponse{}, fmt.Errorf("camera: a still of %dx%d", cfg.Width, cfg.Height)
	}
	return csdef.CaptureSnapshotResponse{
		Data: data, ImageCodec: csdef.ImageCodecJpeg,
		Resolution: csdef.VideoResolutionStruct{Width: uint16(cfg.Width), Height: uint16(cfg.Height)},
	}, nil
}

// The host serves every port of the camera row.
var (
	_ wpdef.ProviderPort        = (*Host)(nil)
	_ csdef.VideoStreamPort     = (*Host)(nil)
	_ csdef.AudioStreamPort     = (*Host)(nil)
	_ csdef.SnapshotStreamPort  = (*Host)(nil)
	_ csdef.SnapshotCapturePort = (*Host)(nil)
)
