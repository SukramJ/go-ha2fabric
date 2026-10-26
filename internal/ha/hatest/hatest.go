// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

// Package hatest is a fake Home Assistant WebSocket API for the daemon's
// tests: the auth handshake, the registry lists (entities, devices,
// areas, labels), get_states, get_config,
// homeassistant/expose_entity/list, subscribe_events with the events the
// test raises, ping, and call_service recording. Its data is the recorded
// fixtures under testdata/ha/ of the repository, each of which names the
// Home Assistant core lines (tag 2026.10.3) whose shape it mirrors.
package hatest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/SukramJ/go-ha2fabric/internal/ha"
)

// Token is the access token the fake accepts.
const Token = "hatest-token"

// Server is the fake. Its methods are safe for concurrent use.
type Server struct {
	t   testing.TB
	srv *httptest.Server

	mu       sync.Mutex
	entities []map[string]any
	devices  []map[string]any
	areas    []map[string]any
	labels   []map[string]any
	states   []map[string]any
	exposed  map[string]map[string]bool
	// deleted are removed registry entries and their states, by entity
	// id, as Home Assistant's deleted_entities keeps them.
	deleted map[string][2]map[string]any
	config  map[string]any
	calls   []ha.ServiceCall
	conns   map[*conn]bool
	down    bool
	callCh  chan ha.ServiceCall
	authed  int
	// OnCall, when set, runs for every call_service after it is recorded
	// (a test's model of the entity: a turn_on that sets the state).
	OnCall func(ha.ServiceCall)
	// Fail, when set, makes call_service fail with its error.
	Fail func(ha.ServiceCall) *ha.Error
}

type conn struct {
	ws     *websocket.Conn
	mu     sync.Mutex
	subs   map[string][]int64
	lastID int64
}

// Fixtures is the directory of the recorded fixtures: testdata/ha/ at the
// repository root.
func Fixtures() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "testdata", "ha")
}

// Fixture reads one fixture file's payload ("result" or "message").
func Fixture(tb testing.TB, name string) json.RawMessage {
	tb.Helper()
	b, err := os.ReadFile(filepath.Join(Fixtures(), name)) //nolint:gosec // the repository's fixture
	if err != nil {
		tb.Fatal(err)
	}
	var f struct {
		Mirrors string          `json:"_mirrors"`
		Result  json.RawMessage `json:"result"`
		Message json.RawMessage `json:"message"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		tb.Fatalf("%s: %v", name, err)
	}
	if f.Mirrors == "" {
		tb.Fatalf("%s: no _mirrors citation", name)
	}
	if f.Result != nil {
		return f.Result
	}
	return f.Message
}

func decode[T any](tb testing.TB, name string) T {
	tb.Helper()
	var v T
	if err := json.Unmarshal(Fixture(tb, name), &v); err != nil {
		tb.Fatalf("%s: %v", name, err)
	}
	return v
}

// New starts the fake with the recorded fixtures; the test's cleanup
// stops it.
func New(tb testing.TB) *Server {
	tb.Helper()
	s := &Server{
		t:        tb,
		entities: decode[[]map[string]any](tb, "entity_registry_list.json"),
		devices:  decode[[]map[string]any](tb, "device_registry_list.json"),
		areas:    decode[[]map[string]any](tb, "area_registry_list.json"),
		labels:   decode[[]map[string]any](tb, "label_registry_list.json"),
		states:   decode[[]map[string]any](tb, "get_states.json"),
		config:   decode[map[string]any](tb, "get_config.json"),
		conns:    map[*conn]bool{},
		callCh:   make(chan ha.ServiceCall, 64),
	}
	ex := decode[ha.Exposed](tb, "expose_entity_list.json")
	s.exposed = ex.Entities
	s.srv = httptest.NewServer(http.HandlerFunc(s.serve))
	tb.Cleanup(func() {
		s.Down()
		s.srv.Close()
	})
	return s
}

// URL is the fake's WebSocket URL.
func (s *Server) URL() string { return "ws" + strings.TrimPrefix(s.srv.URL, "http") + "/api/websocket" }

// Authenticated is the number of connections that authenticated so far.
func (s *Server) Authenticated() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.authed
}

// Down closes every connection and refuses new ones until [Server.Up]:
// Home Assistant restarting.
func (s *Server) Down() {
	s.mu.Lock()
	s.down = true
	conns := make([]*conn, 0, len(s.conns))
	for c := range s.conns {
		conns = append(conns, c)
	}
	s.mu.Unlock()
	for _, c := range conns {
		_ = c.ws.Close(websocket.StatusGoingAway, "restart")
	}
}

// Up accepts connections again.
func (s *Server) Up() {
	s.mu.Lock()
	s.down = false
	s.mu.Unlock()
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	down := s.down
	s.mu.Unlock()
	if down {
		http.Error(w, "down", http.StatusServiceUnavailable)
		return
	}
	ws, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	ws.SetReadLimit(1 << 20)
	ctx := r.Context()
	c := &conn{ws: ws, subs: map[string][]int64{}}
	// Registered before the handshake: the client reports itself
	// connected on auth_ok, and a Down right after must reach this
	// connection (TestClientReconnects raced the registration).
	s.mu.Lock()
	s.conns[c] = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.conns, c)
		s.mu.Unlock()
		_ = ws.CloseNow()
	}()
	if !s.handshake(ctx, c) {
		_ = ws.Close(websocket.StatusPolicyViolation, "auth")
		return
	}
	for {
		_, b, err := ws.Read(ctx)
		if err != nil {
			return
		}
		var cmd map[string]any
		if err := json.Unmarshal(b, &cmd); err != nil {
			return
		}
		id, _ := cmd["id"].(float64)
		if int64(id) <= c.lastID {
			// websocket_api/connection.py refuses an id that does not
			// increase; the fake fails the test instead.
			s.t.Errorf("hatest: id %v after %d", cmd["id"], c.lastID)
			return
		}
		c.lastID = int64(id)
		s.command(ctx, c, int64(id), cmd)
	}
}

func (s *Server) handshake(ctx context.Context, c *conn) bool {
	if err := c.ws.Write(ctx, websocket.MessageText, Fixture(s.t, "auth_required.json")); err != nil {
		return false
	}
	_, b, err := c.ws.Read(ctx)
	if err != nil {
		return false
	}
	var auth struct {
		Type        string `json:"type"`
		AccessToken string `json:"access_token"`
	}
	if json.Unmarshal(b, &auth) != nil || auth.Type != ha.TypeAuth || auth.AccessToken != Token {
		_ = c.ws.Write(ctx, websocket.MessageText, Fixture(s.t, "auth_invalid.json"))
		return false
	}
	// Counted before auth_ok leaves: the client reports itself connected
	// on it, and a test reads the count right after.
	s.mu.Lock()
	s.authed++
	s.mu.Unlock()
	return c.ws.Write(ctx, websocket.MessageText, Fixture(s.t, "auth_ok.json")) == nil
}

func (s *Server) command(ctx context.Context, c *conn, id int64, cmd map[string]any) {
	typ, _ := cmd["type"].(string)
	s.mu.Lock()
	var result any
	switch typ {
	case "ping":
		s.mu.Unlock()
		c.send(ctx, map[string]any{"id": id, "type": "pong"})
		return
	case "config/entity_registry/list":
		result = clone(s.entities)
	case "config/device_registry/list":
		result = clone(s.devices)
	case "config/area_registry/list":
		result = clone(s.areas)
	case "config/label_registry/list":
		result = clone(s.labels)
	case "get_states":
		result = clone(s.states)
	case "get_config":
		result = s.config
	case "homeassistant/expose_entity/list":
		result = map[string]any{"exposed_entities": s.exposed}
	case "subscribe_events":
		et, _ := cmd["event_type"].(string)
		c.mu.Lock()
		c.subs[et] = append(c.subs[et], id)
		c.mu.Unlock()
	case "call_service":
		sc := serviceCall(cmd)
		s.calls = append(s.calls, sc)
		onCall, fail := s.OnCall, s.Fail
		s.mu.Unlock()
		select {
		case s.callCh <- sc:
		default:
		}
		if fail != nil {
			if e := fail(sc); e != nil {
				c.send(ctx, map[string]any{"id": id, "type": "result", "success": false, "error": e})
				return
			}
		}
		if onCall != nil {
			onCall(sc)
		}
		c.send(ctx, map[string]any{
			"id": id, "type": "result", "success": true,
			"result": map[string]any{"context": map[string]any{"id": "01JBCALL", "parent_id": nil, "user_id": nil}},
		})
		return
	default:
		s.mu.Unlock()
		c.send(ctx, map[string]any{
			"id": id, "type": "result", "success": false,
			"error": map[string]any{"code": "unknown_command", "message": "Unknown command."},
		})
		return
	}
	s.mu.Unlock()
	c.send(ctx, map[string]any{"id": id, "type": "result", "success": true, "result": result})
}

func serviceCall(cmd map[string]any) ha.ServiceCall {
	sc := ha.ServiceCall{}
	sc.Domain, _ = cmd["domain"].(string)
	sc.Service, _ = cmd["service"].(string)
	sc.Data, _ = cmd["service_data"].(map[string]any)
	if t, ok := cmd["target"].(map[string]any); ok {
		sc.EntityID, _ = t["entity_id"].(string)
	}
	return sc
}

func (c *conn) send(ctx context.Context, m any) {
	b, err := json.Marshal(m)
	if err != nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.ws.Write(ctx, websocket.MessageText, b)
}

func clone[T any](v T) T {
	b, _ := json.Marshal(v)
	var out T
	_ = json.Unmarshal(b, &out)
	return out
}

// fire sends an event to every connection subscribed to its type
// (websocket_api/messages.py:107-109, core.py Event._as_dict :1397-1412 at
// 2026.10.3).
func (s *Server) fire(eventType string, data any) {
	ev := map[string]any{
		"event_type": eventType, "data": data, "origin": "LOCAL",
		"time_fired": time.Now().UTC().Format("2006-01-02T15:04:05.000000+00:00"),
		"context":    map[string]any{"id": "01JBEVENT", "parent_id": nil, "user_id": nil},
	}
	s.mu.Lock()
	conns := make([]*conn, 0, len(s.conns))
	for c := range s.conns {
		conns = append(conns, c)
	}
	s.mu.Unlock()
	for _, c := range conns {
		c.mu.Lock()
		ids := slices.Clone(c.subs[eventType])
		c.mu.Unlock()
		for _, id := range ids {
			c.send(context.Background(), map[string]any{"id": id, "type": "event", "event": ev})
		}
	}
}

// SetState writes an entity's state and fires state_changed with the old
// and the new state object.
func (s *Server) SetState(entityID, state string, attrs map[string]any) {
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000000+00:00")
	s.mu.Lock()
	var old map[string]any
	idx := -1
	for i, st := range s.states {
		if st["entity_id"] == entityID {
			old, idx = st, i
		}
	}
	if attrs == nil && old != nil {
		attrs, _ = clone(old)["attributes"].(map[string]any)
	}
	next := map[string]any{
		"entity_id": entityID, "state": state, "attributes": attrs,
		"last_changed": now, "last_reported": now, "last_updated": now,
		"context": map[string]any{"id": "01JBSTATE", "parent_id": nil, "user_id": nil},
	}
	if idx >= 0 {
		s.states[idx] = next
	} else {
		s.states = append(s.states, next)
	}
	s.mu.Unlock()
	s.fire(ha.EventStateChanged, map[string]any{"entity_id": entityID, "old_state": old, "new_state": next})
}

// State returns the state object the fake holds for an entity.
func (s *Server) State(entityID string) map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, st := range s.states {
		if st["entity_id"] == entityID {
			return clone(st)
		}
	}
	return nil
}

// UpdateEntity merges changes into an entity's registry entry and fires
// entity_registry_updated with action update
// (homeassistant/helpers/entity_registry.py:178-184 at 2026.10.3).
func (s *Server) UpdateEntity(entityID string, changes map[string]any) {
	s.mu.Lock()
	old := map[string]any{}
	for _, e := range s.entities {
		if e["entity_id"] == entityID {
			for k, v := range changes {
				old[k] = e[k]
				e[k] = v
			}
		}
	}
	s.mu.Unlock()
	s.fire(ha.EventEntityRegistryUpdated, map[string]any{"action": "update", "entity_id": entityID, "changes": old})
}

// UpdateDevice merges changes into a device's registry entry and fires
// device_registry_updated with action update
// (homeassistant/helpers/device_registry.py:78-80 and its update event
// data, device_id and changes, at 2026.10.3).
func (s *Server) UpdateDevice(deviceID string, changes map[string]any) {
	s.mu.Lock()
	old := map[string]any{}
	for _, d := range s.devices {
		if d["id"] == deviceID {
			for k, v := range changes {
				old[k] = d[k]
				d[k] = v
			}
		}
	}
	s.mu.Unlock()
	s.fire(ha.EventDeviceRegistryUpdated, map[string]any{"action": "update", "device_id": deviceID, "changes": old})
}

// RenameEntity changes an entity's id in its registry entry and state,
// keeping the entry's id, and fires entity_registry_updated with
// old_entity_id (homeassistant/helpers/entity_registry.py:171-189,
// :2051-2061 at 2026.10.3).
func (s *Server) RenameEntity(oldID, newID string) {
	s.mu.Lock()
	for _, e := range s.entities {
		if e["entity_id"] == oldID {
			e["entity_id"] = newID
		}
	}
	for _, st := range s.states {
		if st["entity_id"] == oldID {
			st["entity_id"] = newID
		}
	}
	if ex, ok := s.exposed[oldID]; ok {
		s.exposed[newID] = ex
		delete(s.exposed, oldID)
	}
	s.mu.Unlock()
	s.fire(ha.EventEntityRegistryUpdated, map[string]any{
		"action": "update", "entity_id": newID, "old_entity_id": oldID,
		"changes": map[string]any{"entity_id": oldID},
	})
}

// UpdateLabel renames a label and fires label_registry_updated with
// action update (homeassistant/helpers/label_registry.py:183-224 at
// 2026.10.3).
func (s *Server) UpdateLabel(labelID, name string) {
	s.mu.Lock()
	for _, l := range s.labels {
		if l["label_id"] == labelID {
			l["name"] = name
		}
	}
	s.mu.Unlock()
	s.fire(ha.EventLabelRegistryUpdated, map[string]any{"action": "update", "label_id": labelID})
}

// RemoveLabel deletes a label, clears it from every entity and device
// that carries it, and fires label_registry_updated with action remove
// (homeassistant/helpers/label_registry.py:169-180; the registries clear
// the id, helpers/entity_registry.py:2529-2532, at 2026.10.3).
func (s *Server) RemoveLabel(labelID string) {
	s.mu.Lock()
	s.labels = slices.DeleteFunc(s.labels, func(l map[string]any) bool { return l["label_id"] == labelID })
	for _, list := range [][]map[string]any{s.entities, s.devices} {
		for _, e := range list {
			if ls, ok := e["labels"].([]any); ok {
				e["labels"] = slices.DeleteFunc(slices.Clone(ls), func(v any) bool { return v == labelID })
			}
		}
	}
	s.mu.Unlock()
	s.fire(ha.EventLabelRegistryUpdated, map[string]any{"action": "remove", "label_id": labelID})
}

// SetExposed sets an entity's exposure to Assist, as
// homeassistant/expose_entity does, and fires the registry update the
// stored option causes.
func (s *Server) SetExposed(entityID string, exposed bool) {
	s.mu.Lock()
	if exposed {
		s.exposed[entityID] = map[string]bool{ha.AssistantConversation: true}
	} else {
		delete(s.exposed, entityID)
	}
	s.mu.Unlock()
	s.fire(ha.EventEntityRegistryUpdated, map[string]any{"action": "update", "entity_id": entityID, "changes": map[string]any{"options": map[string]any{}}})
}

// RemoveEntity deletes an entity's registry entry and its state and fires
// entity_registry_updated with action remove; the entry is kept for
// [Server.RestoreEntity] as Home Assistant keeps it in deleted_entities
// (homeassistant/helpers/entity_registry.py:171-175, :1728-1775 at
// 2026.10.3).
func (s *Server) RemoveEntity(entityID string) {
	s.mu.Lock()
	var kept [2]map[string]any
	s.entities = slices.DeleteFunc(s.entities, func(e map[string]any) bool {
		if e["entity_id"] == entityID {
			kept[0] = e
			return true
		}
		return false
	})
	s.states = slices.DeleteFunc(s.states, func(st map[string]any) bool {
		if st["entity_id"] == entityID {
			kept[1] = st
			return true
		}
		return false
	})
	if s.deleted == nil {
		s.deleted = map[string][2]map[string]any{}
	}
	s.deleted[entityID] = kept
	s.mu.Unlock()
	s.fire(ha.EventEntityRegistryUpdated, map[string]any{"action": "remove", "entity_id": entityID})
}

// RestoreEntity creates a removed entity again with its old registry
// entry id (homeassistant/helpers/entity_registry.py:1624 at 2026.10.3)
// and its state, and fires entity_registry_updated with action create.
func (s *Server) RestoreEntity(entityID string) {
	s.mu.Lock()
	kept, ok := s.deleted[entityID]
	delete(s.deleted, entityID)
	if ok && kept[0] != nil {
		s.entities = append(s.entities, kept[0])
	}
	if ok && kept[1] != nil {
		s.states = append(s.states, kept[1])
	}
	s.mu.Unlock()
	s.fire(ha.EventEntityRegistryUpdated, map[string]any{"action": "create", "entity_id": entityID})
}

// Calls returns the service calls recorded so far.
func (s *Server) Calls() []ha.ServiceCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.calls)
}

// WaitCall waits for a call that match accepts.
func (s *Server) WaitCall(ctx context.Context, match func(ha.ServiceCall) bool) (ha.ServiceCall, error) {
	for _, c := range s.Calls() {
		if match(c) {
			return c, nil
		}
	}
	for {
		select {
		case c := <-s.callCh:
			if match(c) {
				return c, nil
			}
		case <-ctx.Done():
			return ha.ServiceCall{}, errors.Join(ctx.Err(), errors.New("hatest: no matching call_service"))
		}
	}
}
