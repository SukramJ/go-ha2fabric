// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package hatest

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"time"
)

// Offer is a camera/webrtc/offer the fake received.
type Offer struct {
	// ID is the command's id, the subscription unsubscribe_events names.
	ID       int64
	EntityID string
	SDP      string
	// SessionID is the session id the fake announced in its session
	// event.
	SessionID string

	c *conn
}

// Candidate is a camera/webrtc/candidate the fake received.
type Candidate struct {
	EntityID  string
	SessionID string
	Candidate map[string]any
}

// ImageRequest is a GET of the camera proxy that passed the auth check.
type ImageRequest struct {
	EntityID      string
	Width, Height string
}

type camera struct {
	script       func(o Offer) []map[string]any
	images       map[string][]byte
	off          map[string]bool
	offers       []Offer
	candidates   []Candidate
	unsubscribed []int64
	imageReqs    []ImageRequest
}

// SessionEvent, AnswerEvent, CandidateEvent and ErrorEvent are the event
// objects camera/webrtc/offer pushes (camera/webrtc.py:40-90 at
// 2026.10.3; a candidate's keys are webrtc-models 0.3.0's aliases,
// webrtc_models/__init__.py:76-85).
func SessionEvent(id string) map[string]any {
	return map[string]any{"type": "session", "session_id": id}
}

// AnswerEvent: see [SessionEvent].
func AnswerEvent(sdp string) map[string]any { return map[string]any{"type": "answer", "answer": sdp} }

// CandidateEvent: see [SessionEvent].
func CandidateEvent(candidate, mid string, index int) map[string]any {
	return map[string]any{"type": "candidate", "candidate": map[string]any{"candidate": candidate, "sdpMid": mid, "sdpMLineIndex": index}}
}

// ErrorEvent: see [SessionEvent].
func ErrorEvent(code, message string) map[string]any {
	return map[string]any{"type": "error", "code": code, "message": message}
}

// FakeAnswer and FakeCandidate are what the default script answers.
const (
	FakeAnswer    = "v=0\r\no=- 0 0 IN IP4 127.0.0.1\r\ns=hatest answer\r\n"
	FakeCandidate = "candidate:1 1 UDP 2122252543 192.0.2.10 40000 typ host"
)

// SetWebRTCScript sets the events the fake pushes after a
// camera/webrtc/offer's result; nil restores the default: session, the
// answer FakeAnswer and one candidate FakeCandidate.
func (s *Server) SetWebRTCScript(script func(o Offer) []map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cam.script = script
}

// SetImage sets the still the camera proxy serves for an entity.
func (s *Server) SetImage(entityID string, jpeg []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cam.images == nil {
		s.cam.images = map[string][]byte{}
	}
	s.cam.images[entityID] = jpeg
}

// SetCameraOff makes the camera proxy answer 503 for an entity, as for a
// camera that is off (camera/__init__.py:806-808 at 2026.10.3).
func (s *Server) SetCameraOff(entityID string, off bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cam.off == nil {
		s.cam.off = map[string]bool{}
	}
	s.cam.off[entityID] = off
}

// Offers, Candidates, Unsubscribed and ImageRequests return what the fake
// recorded so far.
func (s *Server) Offers() []Offer {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.cam.offers)
}

// Candidates: see [Server.Offers].
func (s *Server) Candidates() []Candidate {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.cam.candidates)
}

// Unsubscribed: see [Server.Offers].
func (s *Server) Unsubscribed() []int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.cam.unsubscribed)
}

// ImageRequests: see [Server.Offers].
func (s *Server) ImageRequests() []ImageRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.cam.imageReqs)
}

// Await polls cond until it holds or ctx ends.
func (s *Server) Await(ctx context.Context, what string, cond func() bool) error {
	t := time.NewTicker(5 * time.Millisecond)
	defer t.Stop()
	for !cond() {
		select {
		case <-ctx.Done():
			return errors.Join(ctx.Err(), fmt.Errorf("hatest: no %s", what))
		case <-t.C:
		}
	}
	return nil
}

// PushOfferEvent pushes one more event on an offer's subscription.
func (s *Server) PushOfferEvent(o Offer, ev map[string]any) {
	o.c.send(context.Background(), map[string]any{"id": o.ID, "type": "event", "event": ev})
}

// cameraCommand answers the camera commands and unsubscribe_events
// (websocket_api/commands.py:246-263 at 2026.10.3: a known subscription's
// result, else not_found).
func (s *Server) cameraCommand(ctx context.Context, c *conn, id int64, typ string, cmd map[string]any) {
	switch typ {
	case "camera/webrtc/offer":
		entity, _ := cmd["entity_id"].(string)
		sdp, _ := cmd["offer"].(string)
		s.mu.Lock()
		o := Offer{ID: id, EntityID: entity, SDP: sdp, SessionID: "01JBSESSION" + strconv.Itoa(len(s.cam.offers)+1), c: c}
		s.cam.offers = append(s.cam.offers, o)
		script := s.cam.script
		s.mu.Unlock()
		events := []map[string]any{SessionEvent(o.SessionID), AnswerEvent(FakeAnswer), CandidateEvent(FakeCandidate, "0", 0)}
		if script != nil {
			events = script(o)
		}
		c.send(ctx, map[string]any{"id": id, "type": "result", "success": true, "result": nil})
		for _, ev := range events {
			c.send(ctx, map[string]any{"id": id, "type": "event", "event": ev})
		}
	case "camera/webrtc/candidate":
		cd := Candidate{}
		cd.EntityID, _ = cmd["entity_id"].(string)
		cd.SessionID, _ = cmd["session_id"].(string)
		cd.Candidate, _ = cmd["candidate"].(map[string]any)
		s.mu.Lock()
		s.cam.candidates = append(s.cam.candidates, cd)
		s.mu.Unlock()
		c.send(ctx, map[string]any{"id": id, "type": "result", "success": true, "result": nil})
	case "unsubscribe_events":
		sub, _ := cmd["subscription"].(float64)
		found := false
		s.mu.Lock()
		if slices.ContainsFunc(s.cam.offers, func(o Offer) bool { return o.ID == int64(sub) && o.c == c }) &&
			!slices.Contains(s.cam.unsubscribed, int64(sub)) {
			s.cam.unsubscribed = append(s.cam.unsubscribed, int64(sub))
			found = true
		}
		s.mu.Unlock()
		c.mu.Lock()
		for et, ids := range c.subs {
			if i := slices.Index(ids, int64(sub)); i >= 0 {
				c.subs[et] = slices.Delete(ids, i, i+1)
				found = true
			}
		}
		c.mu.Unlock()
		if !found {
			c.send(ctx, map[string]any{
				"id": id, "type": "result", "success": false,
				"error": map[string]any{"code": "not_found", "message": "Subscription not found."},
			})
			return
		}
		c.send(ctx, map[string]any{"id": id, "type": "result", "success": true, "result": nil})
	}
}

// cameraProxy is GET /api/camera_proxy/{entity_id} (camera/__init__.py:
// 782-838 at 2026.10.3): 401 for a wrong Authorization header, 403
// without one, 503 for a camera that is off, the image otherwise.
func (s *Server) cameraProxy(w http.ResponseWriter, r *http.Request) {
	entity := r.PathValue("entity_id")
	if h := r.Header.Get("Authorization"); h != "Bearer "+Token {
		if h != "" {
			http.Error(w, "401: Unauthorized", http.StatusUnauthorized)
			return
		}
		http.Error(w, "403: Forbidden", http.StatusForbidden)
		return
	}
	s.mu.Lock()
	img, ok := s.cam.images[entity]
	off := s.cam.off[entity]
	if ok && !off {
		s.cam.imageReqs = append(s.cam.imageReqs, ImageRequest{EntityID: entity, Width: r.URL.Query().Get("width"), Height: r.URL.Query().Get("height")})
	}
	s.mu.Unlock()
	switch {
	case !ok:
		http.Error(w, "404: Not Found", http.StatusNotFound)
	case off:
		http.Error(w, "503: Service Unavailable", http.StatusServiceUnavailable)
	default:
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(img)
	}
}
