// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package ha

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// WebRTC event types of camera/webrtc/offer: the lowercased class name
// after "WebRTC" (camera/webrtc.py:40-53, 56-90 at 2026.10.3).
const (
	WebRTCSession   = "session"
	WebRTCAnswer    = "answer"
	WebRTCCandidate = "candidate"
	WebRTCError     = "error"
)

// WebRTCEvent is one event of camera/webrtc/offer (camera/webrtc.py:
// 56-90 at 2026.10.3): "session" with session_id, "answer" with the SDP
// answer, "candidate" with an ICE candidate, "error" with code and
// message.
type WebRTCEvent struct {
	Type      string        `json:"type"`
	SessionID string        `json:"session_id,omitempty"`
	Answer    string        `json:"answer,omitempty"`
	Candidate *ICECandidate `json:"candidate,omitempty"`
	Code      string        `json:"code,omitempty"`
	Message   string        `json:"message,omitempty"`
}

// ICECandidate is webrtc-models' RTCIceCandidateInit as Home Assistant
// sends and parses it (webrtc_models/__init__.py:76-85 at 0.3.0, pinned
// by requirements.txt at 2026.10.3; the aliases are the JSON keys,
// default values omitted, :23-25). An empty Candidate is the
// end-of-candidates indication (:89-92).
type ICECandidate struct {
	Candidate     string  `json:"candidate"`
	SdpMid        *string `json:"sdpMid,omitempty"`
	SdpMLineIndex *int    `json:"sdpMLineIndex,omitempty"`
}

// WebRTCOffer is camera/webrtc/offer: the client's SDP offer for the
// camera; the subscription's events carry Home Assistant's session id,
// the answer and the camera's candidates (camera/webrtc.py:252-303 at
// 2026.10.3). Unsubscribing closes the session.
func (c *Client) WebRTCOffer(ctx context.Context, entityID, offer string) (*Subscription, error) {
	return c.Subscribe(ctx, "camera/webrtc/offer", map[string]any{"entity_id": entityID, "offer": offer})
}

// WebRTCCandidate is camera/webrtc/candidate: one ICE candidate of the
// client for the session Home Assistant named in its session event
// (camera/webrtc.py:333-348 at 2026.10.3).
func (c *Client) WebRTCCandidate(ctx context.Context, entityID, sessionID string, candidate ICECandidate) error {
	return c.Call(ctx, map[string]any{
		"type": "camera/webrtc/candidate", "entity_id": entityID,
		"session_id": sessionID, "candidate": candidate,
	}, nil)
}

// Errors of [Client.Image].
var (
	// ErrUnauthorized is the camera proxy's 401 (a token Home Assistant
	// refused) or 403 (no Authorization header reached it)
	// (camera/__init__.py:794-804 at 2026.10.3).
	ErrUnauthorized = errors.New("ha: the camera proxy refused the token")
	// ErrCameraOff is the camera proxy's 503: the camera is off
	// (camera/__init__.py:806-808 at 2026.10.3).
	ErrCameraOff = errors.New("ha: the camera is off")
	// ErrImageTooLarge is an image over the client's ImageLimit.
	ErrImageTooLarge = errors.New("ha: the camera image exceeds the limit")
)

// DefaultImageLimit bounds a still: go-fabric's large-message bound of
// 64000 bytes (secure/sessionparams/sessionparams.go:44 at v0.15.5), of
// which its snapshot bound is the image's share, 100 bytes less
// (cluster/camera/avstream.go:77-83). An image between the two reaches
// the kit, which answers RESOURCE_EXHAUSTED for it.
const DefaultImageLimit = 64000

// HTTPBase derives the REST API's base from the WebSocket URL: the
// scheme ws becomes http and wss https, and the path's last element,
// "websocket", is dropped. Stand-alone, ws://host:8123/api/websocket gives
// http://host:8123/api; in the add-on, ws://supervisor/core/websocket —
// the Supervisor's proxy of the core API — gives
// http://supervisor/core/api.
func HTTPBase(wsURL string) (string, error) {
	u, err := url.Parse(wsURL)
	if err != nil {
		return "", fmt.Errorf("ha: the URL: %w", err)
	}
	switch u.Scheme {
	case "ws":
		u.Scheme = "http"
	case "wss":
		u.Scheme = "https"
	default:
		return "", fmt.Errorf("ha: the URL's scheme %q is not ws or wss", u.Scheme)
	}
	p, ok := strings.CutSuffix(strings.TrimSuffix(u.Path, "/"), "/websocket")
	if !ok {
		return "", fmt.Errorf("ha: the URL's path %q does not end in /websocket", u.Path)
	}
	if !strings.HasSuffix(p, "/api") {
		p += "/api"
	}
	u.Path, u.RawQuery, u.Fragment = p, "", ""
	return u.String(), nil
}

// Image is the camera proxy's still (GET /api/camera_proxy/<entity_id>,
// camera/__init__.py:817-838 at 2026.10.3) with the access token as a
// Bearer header (http/auth.py:226-230, 250). width and height go as query
// parameters when above 0 (:826-827); with both, Home Assistant scales a
// JPEG down by a supported factor, never up (camera/__init__.py:169-178,
// camera/img_util.py:18-36).
// The token goes in the header only; no error carries it.
func (c *Client) Image(ctx context.Context, entityID string, width, height int) (data []byte, contentType string, err error) {
	base, err := HTTPBase(c.opts.URL)
	if err != nil {
		return nil, "", err
	}
	u := base + "/camera_proxy/" + url.PathEscape(entityID)
	q := url.Values{}
	if width > 0 {
		q.Set("width", strconv.Itoa(width))
	}
	if height > 0 {
		q.Set("height", strconv.Itoa(height))
	}
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.opts.CallTimeout)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, http.NoBody)
	if err != nil {
		return nil, "", fmt.Errorf("ha: camera_proxy %s: %w", entityID, err)
	}
	if c.opts.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.opts.Token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("ha: camera_proxy %s: %w", entityID, err)
	}
	defer func() { _ = resp.Body.Close() }()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, "", fmt.Errorf("%w: %s, status %d", ErrUnauthorized, entityID, resp.StatusCode)
	case http.StatusServiceUnavailable:
		return nil, "", fmt.Errorf("%w: %s", ErrCameraOff, entityID)
	default:
		return nil, "", fmt.Errorf("ha: camera_proxy %s: status %d", entityID, resp.StatusCode)
	}
	data, err = io.ReadAll(io.LimitReader(resp.Body, c.opts.ImageLimit+1))
	if err != nil {
		return nil, "", fmt.Errorf("ha: camera_proxy %s: %w", entityID, err)
	}
	if int64(len(data)) > c.opts.ImageLimit {
		return nil, "", fmt.Errorf("%w: %s, over %d bytes", ErrImageTooLarge, entityID, c.opts.ImageLimit)
	}
	contentType = resp.Header.Get("Content-Type")
	if mt, _, err := mime.ParseMediaType(contentType); err == nil {
		contentType = mt
	}
	return data, contentType, nil
}
