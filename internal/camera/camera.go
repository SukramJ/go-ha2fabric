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
// It is the one package that imports go-fabric's cluster/camera: the
// signals are methods of *camera.Provider, and camera.ErrPayloadTooLarge
// is the one error a signal over UDP returns (docs/guides/camera.md:122-127
// at v0.15.6).
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

	"github.com/SukramJ/go-hamqtt/model"
	gfcamera "thingfabric.dev/go-fabric/cluster/camera"
	csdef "thingfabric.dev/go-fabric/cluster/spec/cameraavstreammanagement"
	wpdef "thingfabric.dev/go-fabric/cluster/spec/webrtctransportprovider"
	"thingfabric.dev/go-fabric/contract"
	"thingfabric.dev/go-fabric/device"

	"thingfabric.dev/go-ha2fabric/internal/ha"
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
	// ErrTooManyCandidates refuses a ProvideICECandidates call of more
	// than MaxPendingCandidates, one that would queue more than that
	// before Home Assistant named its session, or one beyond the
	// session's MaxSessionCandidates; it travels with RESOURCE_EXHAUSTED.
	ErrTooManyCandidates = errors.New("camera: too many ICE candidates")
	// ErrTooManySessions refuses an offer beyond MaxSessions while no
	// session is idle; it travels with RESOURCE_EXHAUSTED.
	ErrTooManySessions = errors.New("camera: too many WebRTC sessions of the camera")
)

// MaxPendingCandidates bounds the controller's ICE candidates a session
// queues until Home Assistant names its session (HR-P0-bound-28). Neither
// the command (ProvideICECandidates' list is "min 1" only, go-fabric
// cluster/spec/webrtctransportprovider/definition_gen.go:696 at v0.25.0)
// nor the kit (cluster/camera/provider.go:603-617) bounds them. An ICE
// agent gathers one host candidate per local address and one server
// reflexive or relayed candidate per STUN or TURN server and address —
// with bundling a few dozen at most — so 64 holds any real gathering
// while it bounds what a controller can park in a session it never sees
// answered. After Home Assistant's session event candidates pass through
// unqueued.
const MaxPendingCandidates = 64

// MaxSessionCandidates bounds the controller's ICE candidates of one
// session, queued or sent; MaxPendingCandidates also bounds one call
// after Home Assistant's session event. Each candidate is one
// camera/webrtc/candidate command, and the kit holds its command lock for
// the whole call, so an unbounded list would hold the camera's other
// commands. MaxPendingCandidates' reasoning — a few dozen per gathering —
// holds for the whole session; four gatherings' worth allows for ICE
// restarts.
const MaxSessionCandidates = 256

// MaxCandidateBytes bounds one of Home Assistant's ICE candidates on its
// way to the controller (HR-P2-bound-40). A candidate line is a
// foundation, a component, a transport, a priority, an address, a port,
// a type and a few extensions (RFC 8839 section 5.1) — a real one is
// under 300 bytes; a longer one is refused and logged, the session
// stays.
const MaxCandidateBytes = 1024

// MaxSessions bounds a camera's WebRTC sessions. Each is a session of
// Home Assistant's WebRTC provider (a go2rtc consumer, a peer connection)
// and a relay here; go-fabric's kit bounds only the id space (65 535).
// A controller ecosystem opens one live view per viewer, and eight
// viewers of one camera is beyond any household. At the bound the
// session idle longest makes room when it has been idle for EvictIdle
// (a controller that went away never ends its session); otherwise the
// offer is refused with RESOURCE_EXHAUSTED before Home Assistant is
// asked.
const MaxSessions = 8

// Defaults of [Options].
const (
	// DefaultIdleTimeout ends a session without signalling for an hour:
	// once the peer connection stands, a live view signals nothing, so
	// the bound is long; it ends what a controller left behind.
	DefaultIdleTimeout = time.Hour
	// DefaultEvictIdle is a minute: far beyond a session's signalling
	// phase, so a session being set up is never evicted.
	DefaultEvictIdle = time.Minute
	// StillReuse is how long a camera proxy still is handed to a further
	// snapshot of the same size instead of a new request.
	StillReuse = time.Second
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
	// IdleTimeout ends a session without a controller command or a Home
	// Assistant event the relay acts on for so long; 0 is
	// DefaultIdleTimeout.
	IdleTimeout time.Duration
	// EvictIdle is how long a session must have been idle to make room
	// for a new offer at MaxSessions; 0 is DefaultEvictIdle.
	EvictIdle time.Duration
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
	// still is the latest camera proxy request, shared by the snapshots
	// that arrive while it runs and reused for StillReuse.
	still *still
}

// session is one WebRTC session: the controller's id and Home Assistant's.
type session struct {
	id     uint16
	stream Stream
	// haID is Home Assistant's session id, from its session event;
	// pending holds the controller's candidates until it arrived, at most
	// MaxPendingCandidates.
	haID    string
	pending []ha.ICECandidate
	// sent counts the controller's candidates, queued or sent, against
	// MaxSessionCandidates.
	sent int
	// got counts Home Assistant's candidates against MaxSessionCandidates.
	got int
	// last is the time of the last controller command or Home Assistant
	// event the relay acts on; idle ends the session IdleTimeout after it.
	last time.Time
	idle *time.Timer
}

// still is one camera proxy request; its fields are set before done
// closes.
type still struct {
	w, h int
	done chan struct{}
	at   time.Time
	data []byte
	ct   string
	err  error
}

// dryHost stands in for a camera's host in a plan without a node: it has
// no upstream and is never attached, and New starts nothing.
var dryHost = New("", nil, Options{})

// DryPorts is go-hamapping's Options.Ports for a plan without a node
// (--dump-plan, the web UI's previews): a camera entity gets a host that
// serves nothing, every other entity none. go-hamapping decides a camera
// row on the ports' interfaces (go-hamapping plan.go:120-128 at v0.4.3),
// so the stand-in is a real *Host; the bridge's live hosts take its place
// when it builds.
func DryPorts(e model.Entity) any {
	if e.Platform() == "camera" {
		return dryHost
	}
	return nil
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
	if opts.IdleTimeout <= 0 {
		opts.IdleTimeout = DefaultIdleTimeout
	}
	if opts.EvictIdle <= 0 {
		opts.EvictIdle = DefaultEvictIdle
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
		ss.stopIdle()
		h.unsubscribe(h.ctx, ss)
	}
}

// ErrNoProvider is a device without the kit's *camera.Provider: one the
// camera row did not build on the host's ports.
var ErrNoProvider = errors.New("camera: the device has no WebRtcTransportProvider of the kit")

// AttachDevice attaches the *camera.Provider of the device the plan built
// on the host's ports — the server the node attaches its signalling route
// to (go-fabric node/webrtc_provider_peers.go:43-56 at v0.15.5).
func (h *Host) AttachDevice(d *device.Device) error {
	p, ok := d.Server(wpdef.ClusterID).(*gfcamera.Provider)
	if !ok {
		return fmt.Errorf("%w: %T", ErrNoProvider, d.Server(wpdef.ClusterID))
	}
	h.Attach(p)
	return nil
}

// Close ends every session with ReasonUnknown and stops the host. The
// sessions end together: each one's unsubscribe and End go out at once,
// also when ctx has ended (a daemon shutting down), and all of them are
// bounded by one deadline — SignalTimeout from now, or ctx's deadline
// when that is earlier (HR-P2-conc-60). Close returns by that deadline;
// a signal still running then is abandoned, as is the host's other work
// (a relay's last signal, a still).
func (h *Host) Close(ctx context.Context) {
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
	deadline := time.Now().Add(h.opts.SignalTimeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	ectx, cancel := context.WithDeadline(context.WithoutCancel(ctx), deadline)
	defer cancel()
	var ends sync.WaitGroup
	for _, ss := range all {
		ss.stopIdle()
		ends.Go(func() {
			if err := ss.stream.Unsubscribe(ectx); err != nil {
				h.log.Debug("camera.unsubscribe", slog.Int("session", int(ss.id)), slog.String("error", err.Error()))
			}
		})
		if sig != nil {
			ends.Go(func() {
				if err := sig.End(ectx, ss.id, ReasonUnknown); err != nil {
					h.log.Debug("camera.end", slog.Int("session", int(ss.id)), slog.String("error", err.Error()))
				}
			})
		}
	}
	h.cancel()
	done := make(chan struct{})
	go func() {
		ends.Wait()
		h.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ectx.Done():
		select {
		case <-done:
			return
		default:
		}
		h.log.Warn("camera.close_overrun", slog.Int("sessions", len(all)), slog.String("error", ectx.Err().Error()))
	}
}

// SolicitOffer implements wpdef.ProviderPort: not served, since Home
// Assistant's API takes the client's offer only. The error carries
// INVALID_IN_STATE, which the kit's server passes through as the answer
// (cluster/camera/chime.go:291-299 at v0.15.6, spec.CarriesStatus).
func (h *Host) SolicitOffer(context.Context, wpdef.OfferArgs) (wpdef.WebRTCSessionStruct, bool, error) {
	return wpdef.WebRTCSessionStruct{}, false, contract.StatusError{Status: statusInvalidInState, Message: ErrSolicitOffer.Error()}
}

// ProvideAnswer implements wpdef.ProviderPort: never expected.
func (h *Host) ProvideAnswer(context.Context, uint16, string) error { return ErrProvideAnswer }

// ProvideOffer implements wpdef.ProviderPort: the controller's offer goes
// to camera/webrtc/offer, and the session the kit stores is returned. A
// re-offer of a session the host holds replaces its Home Assistant
// session; when the new one fails, the session ends (the controller is
// told, which takes it out of CurrentSessions). A new session beyond
// MaxSessions evicts the one idle longest, or is refused.
//
// The cap check and the insert after Home Assistant's answer are not one
// step under h.mu: the cap holds because go-fabric runs one command of a
// camera's provider at a time (cluster/camera/provider.go:125-131 and
// state.go:15-26 at v0.25.0), so no second offer of this camera runs
// between them (HR-P2-conc-69). The same lock means a slow Home Assistant
// holds the camera's other commands (EndSession, ProvideIceCandidates)
// until this offer returns (an evicted or replaced session's unsubscribe,
// each within SignalTimeout, and the offer's call, within the request's
// deadline).
func (h *Host) ProvideOffer(ctx context.Context, args wpdef.OfferArgs) (wpdef.WebRTCSessionStruct, error) {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return wpdef.WebRTCSessionStruct{}, ErrClosed
	}
	old := h.sessions[args.SessionID]
	delete(h.sessions, args.SessionID)
	var victim *session
	if old == nil && len(h.sessions) >= MaxSessions {
		victim = h.idlest()
		if victim == nil {
			n := len(h.sessions)
			h.mu.Unlock()
			return wpdef.WebRTCSessionStruct{}, fmt.Errorf("%w: %d held, session %d: %w", ErrTooManySessions, n, args.SessionID,
				contract.StatusError{Status: contract.StatusResourceExhausted, Message: ErrTooManySessions.Error()})
		}
		delete(h.sessions, victim.id)
		h.wg.Add(1)
	}
	h.mu.Unlock()
	if victim != nil {
		h.log.Info("camera.session_evicted", slog.Int("session", int(victim.id)), slog.Int("for", int(args.SessionID)))
		victim.stopIdle()
		h.unsubscribe(ctx, victim)
		// The controller that left it may be gone: its End must not hold
		// the new offer.
		go func() {
			defer h.wg.Done()
			h.tell(h.ctx, victim.id, ReasonUnknown) //nolint:contextcheck // outlives the request, bounded by the host
		}()
	}
	if old != nil {
		old.stopIdle()
		h.unsubscribe(ctx, old)
	}
	stream, err := h.up.Subscribe(ctx, h.entity, args.Sdp)
	if err != nil {
		h.log.Warn("camera.offer_failed", slog.Int("session", int(args.SessionID)), slog.String("error", err.Error()))
		if old != nil {
			// The kit keeps a session whose re-offer failed; its stream is
			// gone, so end it.
			h.tell(ctx, args.SessionID, ReasonICEFailed)
		}
		return wpdef.WebRTCSessionStruct{}, err
	}
	ss := &session{id: args.SessionID, stream: stream, last: time.Now()}
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		h.unsubscribe(ctx, ss)
		return wpdef.WebRTCSessionStruct{}, ErrClosed
	}
	ss.idle = time.AfterFunc(h.opts.IdleTimeout, func() { h.expire(ss) })
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

// idlest is the session idle longest when it has been idle for
// EvictIdle, else nil. h.mu is held.
func (h *Host) idlest() *session {
	var out *session
	for _, ss := range h.sessions {
		if out == nil || ss.last.Before(out.last) {
			out = ss
		}
	}
	if out == nil || time.Since(out.last) < h.opts.EvictIdle {
		return nil
	}
	return out
}

// touch records activity on ss and pushes its idle end back.
func (h *Host) touch(ss *session) {
	h.mu.Lock()
	defer h.mu.Unlock()
	ss.last = time.Now()
	if ss.idle != nil {
		ss.idle.Reset(h.opts.IdleTimeout)
	}
}

// expire ends ss after IdleTimeout without activity.
func (h *Host) expire(ss *session) {
	if !h.own(ss) {
		return
	}
	h.log.Info("camera.session_idle", slog.Int("session", int(ss.id)), slog.Duration("idle", h.opts.IdleTimeout))
	h.end(h.ctx, ss, ReasonUnknown)
}

// stopIdle stops the session's idle end.
func (ss *session) stopIdle() {
	if ss.idle != nil {
		ss.idle.Stop()
	}
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
// Home Assistant named it, the candidates wait. A call carries at most
// MaxPendingCandidates, the queue holds at most as many, and a session
// takes at most MaxSessionCandidates; a call beyond is refused whole.
func (h *Host) ProvideIceCandidates(ctx context.Context, sessionID uint16, candidates []wpdef.ICECandidateStruct) error {
	h.mu.Lock()
	ss := h.sessions[sessionID]
	if ss == nil {
		h.mu.Unlock()
		return fmt.Errorf("%w: %d", gfcamera.ErrUnknownSession, sessionID)
	}
	refuse := func(what string, n int) error {
		return fmt.Errorf("%w: %s %d, %d more, session %d: %w", ErrTooManyCandidates, what, n, len(candidates), sessionID,
			contract.StatusError{Status: contract.StatusResourceExhausted, Message: ErrTooManyCandidates.Error()})
	}
	switch {
	case len(candidates) > MaxPendingCandidates:
		h.mu.Unlock()
		return refuse("call of at most", MaxPendingCandidates)
	case ss.sent+len(candidates) > MaxSessionCandidates:
		sent := ss.sent
		h.mu.Unlock()
		return refuse("session holds", sent)
	case ss.haID == "" && len(ss.pending)+len(candidates) > MaxPendingCandidates:
		queued := len(ss.pending)
		h.mu.Unlock()
		return refuse("queued", queued)
	}
	ss.sent += len(candidates)
	ss.last = time.Now()
	ss.idle.Reset(h.opts.IdleTimeout)
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
		ss.stopIdle()
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

// event handles one event; false ends the relay. Only an event it acts
// on is activity: one it ignores or refuses does not put off the
// session's idle end (HR-P2-bound-40).
func (h *Host) event(ctx context.Context, ss *session, ev ha.WebRTCEvent) bool {
	switch ev.Type {
	case ha.WebRTCSession:
		h.touch(ss)
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
		if ev.Answer == "" {
			// No SDP to relay: the session cannot stand, as after Home
			// Assistant's error.
			h.log.Warn("camera.answer_empty", slog.Int("session", int(ss.id)))
			h.end(ctx, ss, ReasonICEFailed)
			return false
		}
		h.touch(ss)
		return h.signal(ctx, ss, "Answer", func(ctx context.Context, s Signaller) error { return s.Answer(ctx, ss.id, ev.Answer) })
	case ha.WebRTCCandidate:
		if ev.Candidate == nil || ev.Candidate.Candidate == "" {
			// The end-of-candidates indication (webrtc_models/__init__.py:
			// 89-92 at 0.3.0) has no counterpart in ICECandidates.
			return true
		}
		// Home Assistant's side is held to the controller's bound
		// (HR-P2-bound-40): MaxSessionCandidates per session, refused or
		// relayed, and MaxCandidateBytes per candidate.
		h.mu.Lock()
		ss.got++
		got := ss.got
		h.mu.Unlock()
		if got > MaxSessionCandidates {
			h.log.Warn("camera.candidates_exceeded", slog.Int("session", int(ss.id)), slog.Int("max", MaxSessionCandidates))
			h.end(ctx, ss, ReasonICEFailed)
			return false
		}
		if n := len(ev.Candidate.Candidate); n > MaxCandidateBytes {
			h.log.Warn("camera.candidate_refused", slog.Int("session", int(ss.id)), slog.Int("bytes", n), slog.Int("max", MaxCandidateBytes))
			return true
		}
		c, ok := fromHA(*ev.Candidate)
		if !ok {
			h.log.Warn("camera.candidate_refused", slog.Int("session", int(ss.id)), slog.Int("sdp_mline_index", *ev.Candidate.SdpMLineIndex))
			return true
		}
		h.touch(ss)
		return h.signal(ctx, ss, "IceCandidates", func(ctx context.Context, s Signaller) error {
			return s.IceCandidates(ctx, ss.id, []gfcamera.ProviderICECandidate{c})
		})
	case ha.WebRTCError:
		h.log.Warn("camera.ha_error", slog.Int("session", int(ss.id)), slog.String("code", ha.Excerpt(ev.Code)), slog.String("message", ha.Excerpt(ev.Message)))
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
	if errors.Is(err, gfcamera.ErrPayloadTooLarge) {
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
	h.tell(ctx, ss.id, reason)
}

// tell sends the controller End for the session of id, also when ctx has
// ended, bounded by SignalTimeout; the kit takes the session out of
// CurrentSessions whether or not the command arrives.
func (h *Host) tell(ctx context.Context, id uint16, reason uint8) {
	h.mu.Lock()
	s := h.sig
	h.mu.Unlock()
	if s == nil {
		return
	}
	ectx, cancel := context.WithTimeout(context.WithoutCancel(ctx), h.opts.SignalTimeout)
	defer cancel()
	if err := s.End(ectx, id, reason); err != nil {
		h.log.Debug("camera.end", slog.Int("session", int(id)), slog.String("error", err.Error()))
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
		ss.stopIdle()
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
// scales down only; the response names the size the JPEG has. One
// request runs per camera at a time; a snapshot of the size it asks
// shares its result, and a still younger than StillReuse is reused
// (HR-P1-bound-17).
func (h *Host) CaptureSnapshot(ctx context.Context, _ *uint16, res csdef.VideoResolutionStruct) (csdef.CaptureSnapshotResponse, error) {
	data, ct, err := h.image(ctx, int(res.Width), int(res.Height))
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

// image is the camera proxy's still of w×ht through the host's one
// request (Host.still). The request runs on the host's context, bounded
// by the upstream's own timeout, so a caller that gives up does not fail
// the others; each caller waits on its own ctx.
func (h *Host) image(ctx context.Context, w, ht int) (data []byte, contentType string, err error) {
	for {
		h.mu.Lock()
		st := h.still
		if st != nil {
			select {
			case <-st.done:
				if st.w == w && st.h == ht && st.err == nil && time.Since(st.at) < StillReuse {
					h.mu.Unlock()
					return st.data, st.ct, nil
				}
			default:
				h.mu.Unlock()
				select {
				case <-st.done:
				case <-ctx.Done():
					return nil, "", ctx.Err()
				}
				if st.w == w && st.h == ht {
					return st.data, st.ct, st.err
				}
				continue
			}
		}
		if h.closed {
			h.mu.Unlock()
			return nil, "", ErrClosed
		}
		st = &still{w: w, h: ht, done: make(chan struct{})}
		h.still = st
		h.wg.Add(1)
		h.mu.Unlock()
		go func() {
			defer h.wg.Done()
			st.data, st.ct, st.err = h.up.Image(h.ctx, h.entity, w, ht) //nolint:contextcheck // shared by every waiting snapshot; the host bounds it
			st.at = time.Now()
			close(st.done)
		}()
	}
}

// The host serves every port of the camera row.
var (
	_ wpdef.ProviderPort        = (*Host)(nil)
	_ csdef.VideoStreamPort     = (*Host)(nil)
	_ csdef.AudioStreamPort     = (*Host)(nil)
	_ csdef.SnapshotStreamPort  = (*Host)(nil)
	_ csdef.SnapshotCapturePort = (*Host)(nil)
)
