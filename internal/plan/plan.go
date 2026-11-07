// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

// Package plan is the bridge's pure core. A snapshot of Home Assistant and
// the effective exposure rule ([Input]) become the candidates' verdicts
// and the device groups go-hamapping plans ([Assess], [Dry],
// [Candidates]); the groups the reconciler serves and the ones it is to
// serve become one batch of operations ([Diff]).
//
// Pure means: every function is a function of its arguments, nothing is
// started and nothing is stored. go-hamapping's Map is called here, its
// Build never, and no go-fabric node is reached (internal/arch holds
// that); internal/bridge reconciles a node with what this package
// decides.
package plan

import (
	"encoding/json"
	"slices"
	"sort"
	"strings"

	"github.com/SukramJ/go-fabric/device"
	"github.com/SukramJ/go-fabric/eligibility"
	hamapping "github.com/SukramJ/go-hamapping"
	"github.com/SukramJ/go-hamqtt/model"

	"github.com/SukramJ/go-ha2fabric/internal/adapt"
	"github.com/SukramJ/go-ha2fabric/internal/expose"
	"github.com/SukramJ/go-ha2fabric/internal/ha"
)

// Verdict is one candidate entity's line of the plan an operator reads:
// whether the exposure rule takes it and why, and go-hamapping's verdict.
type Verdict struct {
	EntityID     string `json:"entity_id"`
	Name         string `json:"name,omitempty"`
	Device       string `json:"device,omitempty"`
	Exposed      bool   `json:"exposed"`
	ExposeReason string `json:"expose_reason"`
	// Source is the exposure rule's layer that decided (expose.Source…).
	Source string `json:"source"`
	// Labels are the names of the entity's and its device's labels.
	Labels     []string `json:"labels,omitempty"`
	Row        string   `json:"row,omitempty"`
	State      string   `json:"state,omitempty"`
	Reason     string   `json:"reason,omitempty"`
	DeviceType uint16   `json:"device_type,omitempty"`
	Endpoint   uint16   `json:"endpoint,omitempty"`

	// Full is go-hamapping's verdict in full (with the clusters), as the
	// web UI shows it.
	Full eligibility.Verdict `json:"-"`
}

// Group is one Home Assistant device's exposed entities, ready for
// go-hamapping: the device, its entities in entity id order, their
// registry entries, descriptions and web UI names by entity id, and the
// signature that any change of them moves.
type Group struct {
	Key      string
	Device   *model.Device
	Entities []model.Entity
	Regs     map[string]*ha.EntityEntry
	Descs    map[string]string
	Names    map[string]string
	Sig      string
}

// Describe is the description entity id of g would have with state st:
// a result other than g.Descs[id] means a new plan for the device.
func (g *Group) Describe(id string, st *ha.State, unit string) string {
	return Describe(adapt.Entity(adapt.Input{EntityID: id, Registry: g.Regs[id], State: st, Device: g.Device, TemperatureUnit: unit, Name: g.Names[id]}))
}

// CameraOff is the verdict of an exposed camera while the camera option
// is off.
const CameraOff = "the camera option is off (CAMERA, add-on option camera)"

// isCamera reports whether an entity id is of the camera domain.
func isCamera(id string) bool { return strings.HasPrefix(id, "camera.") }

// Labels is the snapshot's labels as the exposure rule resolves them.
func Labels(s *ha.Snapshot) expose.Labels {
	out := make(expose.Labels, len(s.Labels))
	for _, l := range s.Labels {
		out[l.LabelID] = l.Name
	}
	return out
}

// Assess applies the exposure rule and groups the exposed entities by
// device: the candidates' verdicts (go-hamapping's verdict left to the
// caller) and the devices to plan. A candidate is every entity with a
// state or a registry entry. Without in.Cameras, an exposed camera is not
// planned; its verdict is unmappable with [CameraOff].
func Assess(s *ha.Snapshot, in Input) ([]Verdict, []*Group) {
	reg := map[string]*ha.EntityEntry{}
	for i := range s.Entities {
		reg[s.Entities[i].EntityID] = &s.Entities[i]
	}
	devs := map[string]*ha.DeviceEntry{}
	for i := range s.Devices {
		devs[s.Devices[i].ID] = &s.Devices[i]
	}
	areas := map[string]string{}
	for _, a := range s.Areas {
		areas[a.AreaID] = a.Name
	}
	states := map[string]*ha.State{}
	ids := map[string]bool{}
	for i := range s.States {
		states[s.States[i].EntityID] = &s.States[i]
		ids[s.States[i].EntityID] = true
	}
	for id := range reg {
		ids[id] = true
	}
	order := make([]string, 0, len(ids))
	for id := range ids {
		order = append(order, id)
	}
	sort.Strings(order)

	labels := Labels(s)
	unit := s.Config.TemperatureUnit()
	names := in.Names
	groups := map[string]*Group{}
	var keys []string
	verdicts := make([]Verdict, 0, len(order))
	for _, id := range order {
		r, st := reg[id], states[id]
		e := expose.Entity{EntityID: id, Registered: r != nil, ExposedToAssist: s.Exposed.Entities[id][ha.AssistantConversation]}
		if r != nil {
			e.DisabledBy, e.HiddenBy = r.DisabledBy, r.HiddenBy
			e.Labels = slices.Clone(r.Labels)
			if dev := devs[r.DeviceID]; dev != nil {
				for _, l := range dev.Labels {
					if !slices.Contains(e.Labels, l) {
						e.Labels = append(e.Labels, l)
					}
				}
			}
		}
		d := in.Rule.Decide(e, labels)
		v := Verdict{EntityID: id, Exposed: d.Exposed, ExposeReason: d.Reason, Source: d.Source}
		for _, l := range e.Labels {
			if n := labels[l]; n != "" {
				v.Labels = append(v.Labels, n)
			} else {
				v.Labels = append(v.Labels, l)
			}
		}
		if r != nil && r.DeviceID != "" && devs[r.DeviceID] != nil {
			v.Device = deviceName(devs[r.DeviceID])
		}
		if st != nil {
			v.Name, _ = st.Attributes["friendly_name"].(string)
		}
		if d.Exposed && st == nil {
			v.Exposed, v.ExposeReason = false, d.Reason+"; no state yet"
		}
		if !v.Exposed {
			verdicts = append(verdicts, v)
			continue
		}
		if !in.Cameras && isCamera(id) {
			v.State, v.Reason = eligibility.StateUnmappable.String(), CameraOff
			verdicts = append(verdicts, v)
			continue
		}
		var md *model.Device
		var key string
		if r != nil && r.DeviceID != "" && devs[r.DeviceID] != nil {
			md = adapt.Device(devs[r.DeviceID], areas)
		} else {
			name, _ := st.Attributes["friendly_name"].(string)
			if name == "" {
				name = id
			}
			md = adapt.OwnDevice(id, name)
		}
		key = md.UID()
		v.Device = md.Name.Default
		g := groups[key]
		if g == nil {
			g = &Group{Key: key, Device: md, Descs: map[string]string{}, Regs: map[string]*ha.EntityEntry{}, Names: map[string]string{}}
			groups[key] = g
			keys = append(keys, key)
		}
		ent := adapt.Entity(adapt.Input{EntityID: id, Registry: r, State: st, Device: md, TemperatureUnit: unit, Name: names[id]})
		g.Entities = append(g.Entities, ent)
		g.Regs[id] = r
		if names[id] != "" {
			g.Names[id] = names[id]
		}
		g.Descs[id] = Describe(ent)
		verdicts = append(verdicts, v)
	}
	out := make([]*Group, 0, len(keys))
	for _, k := range keys {
		g := groups[k]
		dev, _ := json.Marshal(g.Device)
		sig := string(dev)
		for _, e := range g.Entities {
			sig += "|" + e.Key() + "=" + g.Descs[e.Key()]
		}
		g.Sig = sig
		out = append(out, g)
	}
	return verdicts, out
}

// Describe is an entity's description as a comparable string: a change
// means a new plan for its device.
func Describe(e model.Entity) string {
	b, _ := json.Marshal(struct {
		P string
		D *model.Description
	}{string(e.Platform()), e.Desc()})
	return string(b)
}

// MergeVerdicts merges go-hamapping's verdicts mv into the candidates'.
func MergeVerdicts(verdicts []Verdict, mv []hamapping.Verdict) {
	idx := map[string]int{}
	for i := range verdicts {
		idx[verdicts[i].EntityID] = i
	}
	for _, m := range mv {
		i, ok := idx[m.Key]
		if !ok {
			continue
		}
		verdicts[i].Row, verdicts[i].State, verdicts[i].Reason, verdicts[i].DeviceType = m.Row, m.State.String(), m.Reason, m.DeviceType
		verdicts[i].Full = m.Verdict
	}
}

// deviceName is a registry device's name: the user's over the
// integration's (adapt.Device).
func deviceName(d *ha.DeviceEntry) string {
	if d.NameByUser != "" {
		return d.NameByUser
	}
	return d.Name
}

// MapOptions are go-hamapping's options for one device: the stable key
// of an entity is its registry entry id (helpers/entity_registry.py:
// 261-263), which a rename of the entity id keeps (:2051-2061, :2104),
// so a renamed entity keeps its endpoint number. An entity without a
// registry entry keeps go-hamapping's default, its entity id.
//
// names are the web UI's name overrides: an overridden entity's node label
// is the name as typed, without the device name (hamapping.Options.Name,
// verbatim; the owner's decision on H3 Q3, a name typed in a UI is final).
// ports is handed through as Options.Ports ([Input.Ports]).
func MapOptions(regs map[string]*ha.EntityEntry, names map[string]string, ports func(model.Entity) any) hamapping.Options {
	return hamapping.Options{
		Ports: ports,
		StableKey: func(e model.Entity) string {
			if r := regs[e.Key()]; r != nil {
				return r.ID
			}
			return ""
		},
		Name: func(e model.Entity) (string, bool) {
			n := names[e.Key()]
			return n, n != ""
		},
	}
}

// Dry is the plan without a node: what --dump-plan prints and the web
// UI's preview shows. Every exposed group goes through go-hamapping's Map
// with in.Ports; nothing is built.
func Dry(s *ha.Snapshot, in Input) []Verdict {
	verdicts, groups := Assess(s, in)
	for _, g := range groups {
		_, mv := hamapping.Map(g.Device, g.Entities, MapOptions(g.Regs, g.Names, in.Ports))
		MergeVerdicts(verdicts, mv)
	}
	return verdicts
}

// Candidates is every entity's verdict as if the lists exposed it: the
// verdict the web UI shows for an entity the rule leaves out. Disabled
// and hidden entities stay out. Of in, the names, the camera option and
// the ports count; the rule does not.
func Candidates(s *ha.Snapshot, in Input) map[string]*Verdict {
	out := map[string]*Verdict{}
	vs := Dry(s, Input{Rule: expose.Rule{Include: []string{"*"}}, Names: in.Names, Cameras: in.Cameras, Ports: in.Ports})
	for i := range vs {
		out[vs[i].EntityID] = &vs[i]
	}
	return out
}

// RemovedKey is the stable key go-hamapping gives entityID's endpoint
// (its device's UID, "/", the registry entry id; see [MapOptions] and
// hamapping's Plan.options), or nil when s is nil or has no registry
// entry for it.
func RemovedKey(s *ha.Snapshot, entityID string) device.SourceKey {
	if s == nil {
		return nil
	}
	for i := range s.Entities {
		r := &s.Entities[i]
		if r.EntityID != entityID || r.ID == "" {
			continue
		}
		md := adapt.OwnDevice(entityID, "")
		for j := range s.Devices {
			if r.DeviceID != "" && s.Devices[j].ID == r.DeviceID {
				areas := map[string]string{}
				for _, a := range s.Areas {
					areas[a.AreaID] = a.Name
				}
				md = adapt.Device(&s.Devices[j], areas)
			}
		}
		return device.StringKey(md.UID() + "/" + r.ID)
	}
	return nil
}
