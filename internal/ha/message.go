// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package ha

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// The message types of the WebSocket API this client speaks. Home
// Assistant core at 2026.10.3:
// homeassistant/components/websocket_api/auth.py:25-28 (auth,
// auth_invalid, auth_ok, auth_required), websocket_api/messages.py:63-65
// (result), :107-109 (event), websocket_api/commands.py:730-736 (pong).
const (
	TypeAuthRequired = "auth_required"
	TypeAuth         = "auth"
	TypeAuthOK       = "auth_ok"
	TypeAuthInvalid  = "auth_invalid"
	TypeResult       = "result"
	TypeEvent        = "event"
	TypePing         = "ping"
	TypePong         = "pong"
)

// The events the daemon subscribes to. homeassistant/const.py:286
// (state_changed), homeassistant/helpers/entity_registry.py:83-85
// (entity_registry_updated), homeassistant/helpers/device_registry.py:78-80
// (device_registry_updated), homeassistant/helpers/label_registry.py:23-26
// (label_registry_updated), all at 2026.10.3. Each is in the
// SUBSCRIBE_ALLOWLIST (homeassistant/auth/permissions/events.py:30-50;
// label_registry_updated at :46), so none needs an administrator.
const (
	EventStateChanged          = "state_changed"
	EventEntityRegistryUpdated = "entity_registry_updated"
	EventDeviceRegistryUpdated = "device_registry_updated"
	EventLabelRegistryUpdated  = "label_registry_updated"
)

// Message is one message of the WebSocket API, in either direction: the
// id-tagged envelope of a command's result or of a subscription's event,
// or one of the id-less auth messages.
type Message struct {
	ID        int64           `json:"id,omitempty"`
	Type      string          `json:"type"`
	Success   *bool           `json:"success,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
	Error     *Error          `json:"error,omitempty"`
	Event     json.RawMessage `json:"event,omitempty"`
	HAVersion string          `json:"ha_version,omitempty"`
	// Message is auth_invalid's reason.
	Message string `json:"message,omitempty"`
}

// Error is a failed result's error object: websocket_api/messages.py:81-104
// at 2026.10.3 (code and message; translation fields are ignored).
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return "home assistant: " + e.Code + ": " + e.Message }

// ErrFrame is the error of a frame that is not a message.
var ErrFrame = errors.New("ha: frame is not a WebSocket API message")

// Decode reads one WebSocket frame: one message object, or the array Home
// Assistant sends when it coalesces several messages into one frame
// (websocket_api/http.py:152-179 at 2026.10.3; the client never enables
// coalescing, the decoder reads it anyway). A message without a type is
// refused.
func Decode(frame []byte) ([]Message, error) {
	frame = bytes.TrimSpace(frame)
	if len(frame) == 0 {
		return nil, ErrFrame
	}
	var out []Message
	switch frame[0] {
	case '{':
		var m Message
		if err := json.Unmarshal(frame, &m); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrFrame, err)
		}
		out = []Message{m}
	case '[':
		if err := json.Unmarshal(frame, &out); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrFrame, err)
		}
	default:
		return nil, ErrFrame
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: an empty array", ErrFrame)
	}
	for i := range out {
		if strings.TrimSpace(out[i].Type) == "" {
			return nil, fmt.Errorf("%w: message %d has no type", ErrFrame, i)
		}
		if out[i].ID < 0 {
			return nil, fmt.Errorf("%w: message %d has a negative id", ErrFrame, i)
		}
	}
	return out, nil
}

// Event is one bus event a subscription carries: core.py Event._as_dict
// (homeassistant/core.py:1397-1412 at 2026.10.3).
type Event struct {
	EventType string          `json:"event_type"`
	Data      json.RawMessage `json:"data"`
	Origin    string          `json:"origin"`
	TimeFired string          `json:"time_fired"`
}

// State is a state object: core.py State._as_dict
// (homeassistant/core.py:1988-2014 at 2026.10.3). Attributes hold the
// entity's capability attributes always and its state attributes while it
// is available (homeassistant/helpers/entity.py:1115-1129), plus
// unit_of_measurement, device_class (the registry's over the
// integration's), friendly_name and supported_features
// (entity.py:1131-1176).
type State struct {
	EntityID    string         `json:"entity_id"`
	State       string         `json:"state"`
	Attributes  map[string]any `json:"attributes"`
	LastChanged string         `json:"last_changed"`
	LastUpdated string         `json:"last_updated"`
}

// The two states that carry no value: homeassistant/const.py:303,313 at
// 2026.10.3.
const (
	StateUnknown     = "unknown"
	StateUnavailable = "unavailable"
)

// StateChanged is state_changed's data: homeassistant/core.py:129-142 at
// 2026.10.3 (entity_id, old_state, new_state; new_state null when the
// entity was removed).
type StateChanged struct {
	EntityID string `json:"entity_id"`
	OldState *State `json:"old_state"`
	NewState *State `json:"new_state"`
}

// RegistryUpdated is the data of entity_registry_updated
// (homeassistant/helpers/entity_registry.py:171-189: action create, remove
// or update, entity_id, changes and old_entity_id for an update) and of
// device_registry_updated (device_id instead of entity_id), at 2026.10.3.
type RegistryUpdated struct {
	Action      string         `json:"action"`
	EntityID    string         `json:"entity_id,omitempty"`
	OldEntityID string         `json:"old_entity_id,omitempty"`
	DeviceID    string         `json:"device_id,omitempty"`
	Changes     map[string]any `json:"changes,omitempty"`
}

// LabelRegistryUpdated is the data of label_registry_updated: action
// create, remove or update and the label's id
// (homeassistant/helpers/label_registry.py:50-54 at 2026.10.3). A rename
// is an update with the same label_id (:183-224).
type LabelRegistryUpdated struct {
	Action  string `json:"action"`
	LabelID string `json:"label_id"`
}

// Label is one entry of config/label_registry/list: _entry_dict
// (homeassistant/components/config/label_registry.py:155-166 at
// 2026.10.3; color, created_at, description, icon, label_id, name,
// modified_at). The daemon reads the id and the name.
type Label struct {
	LabelID     string `json:"label_id"`
	Name        string `json:"name"`
	Icon        string `json:"icon,omitempty"`
	Color       string `json:"color,omitempty"`
	Description string `json:"description,omitempty"`
}

// EntityEntry is one entry of config/entity_registry/list: RegistryEntry
// .as_partial_dict (homeassistant/helpers/entity_registry.py:369-403 at
// 2026.10.3). The list carries no device_class, unit, capabilities or
// supported_features (those are extended_dict's, :405-417); the state's
// attributes carry them. Platform is the integration, the domain is the
// entity id's prefix.
type EntityEntry struct {
	// ID is the registry entry's id (helpers/entity_registry.py:261-263, a
	// random uuid hex). A rename of the entity id keeps it: the update
	// evolves the entry with the new entity_id only (:2051-2061, :2104).
	ID             string                    `json:"id"`
	EntityID       string                    `json:"entity_id"`
	UniqueID       string                    `json:"unique_id"`
	Platform       string                    `json:"platform"`
	DeviceID       string                    `json:"device_id"`
	AreaID         string                    `json:"area_id"`
	DisabledBy     string                    `json:"disabled_by"`
	HiddenBy       string                    `json:"hidden_by"`
	EntityCategory string                    `json:"entity_category"`
	HasEntityName  bool                      `json:"has_entity_name"`
	Name           string                    `json:"name"`
	OriginalName   string                    `json:"original_name"`
	Options        map[string]map[string]any `json:"options"`
	// Labels are the entry's label ids (helpers/entity_registry.py:264,
	// :393 in as_partial_dict).
	Labels []string `json:"labels"`
}

// Domain is the entity's domain, the entity id's prefix.
func (e *EntityEntry) Domain() string { return Domain(e.EntityID) }

// Domain is the domain of an entity id ("light" of "light.kitchen").
func Domain(entityID string) string {
	d, _, _ := strings.Cut(entityID, ".")
	return d
}

// DeviceEntry is one entry of config/device_registry/list: DeviceEntry
// .dict_repr (homeassistant/helpers/device_registry.py:589-629 at
// 2026.10.3); a child device's entry (:739-755) carries no manufacturer,
// model or versions.
type DeviceEntry struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	NameByUser       string     `json:"name_by_user"`
	Manufacturer     string     `json:"manufacturer"`
	Model            string     `json:"model"`
	ModelID          string     `json:"model_id"`
	SWVersion        string     `json:"sw_version"`
	HWVersion        string     `json:"hw_version"`
	SerialNumber     string     `json:"serial_number"`
	AreaID           string     `json:"area_id"`
	DisabledBy       string     `json:"disabled_by"`
	ViaDeviceID      string     `json:"via_device_id"`
	ConfigurationURL string     `json:"configuration_url"`
	Identifiers      [][]string `json:"identifiers"`
	Connections      [][]string `json:"connections"`
	// Labels are the device's label ids (helpers/device_registry.py:412,
	// :616 in dict_repr).
	Labels []string `json:"labels"`
}

// Area is one entry of config/area_registry/list: AreaEntry.json_fragment
// (homeassistant/helpers/area_registry.py:88-98 at 2026.10.3).
type Area struct {
	AreaID string `json:"area_id"`
	Name   string `json:"name"`
}

// Config is the part of get_config's result the daemon reads: the core
// configuration (homeassistant/core_config.py:685-717 at 2026.10.3) with
// its unit_system (homeassistant/util/unit_system.py:213-224: temperature
// and the other units).
type Config struct {
	Version      string            `json:"version"`
	LocationName string            `json:"location_name"`
	UnitSystem   map[string]string `json:"unit_system"`
}

// TemperatureUnit is the configured temperature unit, "°C" when unstated.
func (c *Config) TemperatureUnit() string {
	if c != nil && c.UnitSystem["temperature"] != "" {
		return c.UnitSystem["temperature"]
	}
	return "°C"
}

// Exposed is homeassistant/expose_entity/list's result: every entity with
// the assistants it is exposed to, entities exposed to none left out
// (homeassistant/components/homeassistant/exposed_entities.py:419-442 at
// 2026.10.3).
type Exposed struct {
	Entities map[string]map[string]bool `json:"exposed_entities"`
}

// AssistantConversation is the Assist assistant's id
// (exposed_entities.py:25 KNOWN_ASSISTANTS at 2026.10.3).
const AssistantConversation = "conversation"

// ServiceCall is one call_service command: domain, service, the service
// data and the target entity (websocket_api/commands.py:265-274 at
// 2026.10.3).
type ServiceCall struct {
	Domain   string
	Service  string
	Data     map[string]any
	EntityID string
}

func (s ServiceCall) String() string { return s.Domain + "." + s.Service + " " + s.EntityID }
