// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package bridge_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	fabricui "github.com/SukramJ/go-fabric-ui/fabricui"
	bridgedbidef "github.com/SukramJ/go-fabric/cluster/spec/bridgeddevicebasicinformation"
	"github.com/SukramJ/go-fabric/fabrictest"

	"github.com/SukramJ/go-ha2fabric/internal/bridge"
	"github.com/SukramJ/go-ha2fabric/internal/expose"
	"github.com/SukramJ/go-ha2fabric/internal/selection"
)

// TestLabels: a device label exposes its entities, an entity label its
// entity; removing the label (label_registry_updated) removes the
// endpoints; a label value that names no label is a warning.
func TestLabels(t *testing.T) {
	e := newEnv(t, expose.Rule{IncludeLabels: []string{"MATTER", "Later"}})
	e.commission()
	plan := byID(e.br.Plan())
	for _, id := range []string{"sensor.living_temperature", "sensor.living_battery", "sensor.outdoor"} {
		v := plan[id]
		if !v.Exposed || v.ExposeReason != "included by label 'Matter'" || v.Source != expose.SourceLabel || len(v.Labels) != 1 || v.Labels[0] != "Matter" {
			t.Errorf("%s: %+v", id, v)
		}
	}
	if v := plan["light.kitchen"]; v.Exposed {
		t.Errorf("light.kitchen exposed without the label: %+v", v)
	}
	temp, ok := e.endpoint("sensor.living_temperature")
	if !ok {
		t.Fatal("no endpoint for the labelled device")
	}
	if bat, _ := e.endpoint("sensor.living_battery"); bat != temp {
		t.Fatalf("the battery is on %d, the sensor on %d", bat, temp)
	}
	if _, ok := e.endpoint("sensor.outdoor"); !ok {
		t.Fatal("no endpoint for the labelled entity")
	}
	if w := e.br.Status().Warnings; len(w) != 1 || !strings.Contains(w[0], `"Later"`) {
		t.Fatalf("warnings %q", w)
	}

	e.ha.RemoveLabel("matter")
	e.eventually("the labelled endpoints are gone", func() bool {
		_, a := e.endpoint("sensor.living_temperature")
		_, b := e.endpoint("sensor.outdoor")
		return !a && !b
	})
	if w := e.br.Status().Warnings; len(w) != 2 {
		t.Fatalf("warnings after the removal %q", w)
	}
}

// TestRenameKeepsTheEndpoint: a renamed entity id keeps its endpoint
// number, because its stable key is the registry entry id.
func TestRenameKeepsTheEndpoint(t *testing.T) {
	e := newEnv(t, expose.Rule{Assist: true})
	e.commission()
	light, ok := e.endpoint("light.kitchen")
	if !ok {
		t.Fatal("no light")
	}
	e.ha.RenameEntity("light.kitchen", "light.kitchen_main")
	e.eventually("the renamed light is planned", func() bool {
		_, ok := e.endpoint("light.kitchen_main")
		return ok
	})
	if again, _ := e.endpoint("light.kitchen_main"); again != light {
		t.Fatalf("the renamed light moved from %d to %d", light, again)
	}
	if got, ok := e.dev.EndpointOf("ha:dev-hue-1/" + e.regID("light.kitchen")); !ok || got != light {
		t.Fatalf("stable key: %d %v", got, ok)
	}
}

// TestDisabledAndHiddenNeverAppear: even a glob that names them leaves
// a disabled and a hidden entity without an endpoint, and the verdict
// says why (brief H3 §2.3).
func TestDisabledAndHiddenNeverAppear(t *testing.T) {
	e := newEnv(t, expose.Rule{Include: []string{"light.*"}, Overrides: map[string]bool{"light.cellar": true}})
	plan := byID(e.br.Plan())
	for id, want := range map[string][2]string{
		"light.cellar": {"disabled in Home Assistant (by user)", expose.SourceDisabled},
		"light.attic":  {"hidden in Home Assistant (by user)", expose.SourceHidden},
	} {
		if v := plan[id]; v.Exposed || v.ExposeReason != want[0] || v.Source != want[1] || v.Endpoint != 0 {
			t.Errorf("%s: %+v", id, v)
		}
	}
	if _, ok := e.endpoint("light.kitchen"); !ok {
		t.Fatal("the glob exposed nothing")
	}
}

// ui is the web UI's handler on the env's bridge, as cmd/ha2fabric
// wires it.
type ui struct {
	srv  *httptest.Server
	sel  *selection.Store
	path string
}

func newUI(t *testing.T) (*env, *ui) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "selection.json")
	sel, err := selection.Open(path, logger())
	if err != nil {
		t.Fatal(err)
	}
	ev := fabricui.NewEvents()
	e := newEnv(t, expose.Rule{Assist: true}, func(o *bridge.Options) {
		o.Selection = sel
		o.OnChange = func(keys []string, endpoints bool) {
			if len(keys) > 0 {
				ev.Publish(fabricui.Event{Type: fabricui.EventExposureChanged, Data: fabricui.ExposureEvent{Keys: keys}})
			}
		}
	})
	h := fabricui.Handler(fabricui.Options{
		Node: e.dev.Node(), Exposure: e.br.Exposure(), Admin: func(*http.Request) bool { return true },
		Reassemble: e.br.Resync, Events: ev, Warnings: e.br.Warnings, Logger: logger(), ConfiguredWindow: true,
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return e, &ui{srv: srv, sel: sel, path: path}
}

func (u *ui) do(t *testing.T, method, path string, body, out any) int {
	t.Helper()
	var rd io.Reader = http.NoBody
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, u.srv.URL+"/api/v1/node"+path, rd)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	b, _ := io.ReadAll(res.Body)
	if out != nil && res.StatusCode < 300 {
		if err := json.Unmarshal(b, out); err != nil {
			t.Fatalf("%s %s: %v: %s", method, path, err, b)
		}
	}
	return res.StatusCode
}

// item is the API's form of one selection item.
type item struct {
	Key     string `json:"key"`
	Name    string `json:"name"`
	Group   string `json:"group"`
	Enabled bool   `json:"enabled"`
	Source  string `json:"source"`
	Fields  map[string]string
	Verdict struct {
		State string `json:"state"`
	} `json:"verdict"`
}

func (u *ui) items(t *testing.T) map[string]item {
	t.Helper()
	var list struct{ Items []item }
	if code := u.do(t, http.MethodGet, "/exposure", nil, &list); code != http.StatusOK {
		t.Fatalf("list %d", code)
	}
	out := map[string]item{}
	for _, it := range list.Items {
		out[it.Key] = it
	}
	return out
}

// stream reads the UI's event stream and records the event types.
type stream struct {
	mu     sync.Mutex
	events []string
}

func (s *stream) has(typ string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.events {
		if e == typ {
			return true
		}
	}
	return false
}

func (u *ui) stream(t *testing.T) *stream {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.srv.URL+"/api/v1/node/stream", http.NoBody)
	res, err := http.DefaultClient.Do(req) //nolint:bodyclose // closed in the cleanup below
	if err != nil {
		t.Fatal(err)
	}
	s := &stream{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		sc := bufio.NewScanner(res.Body)
		for sc.Scan() {
			if typ, ok := strings.CutPrefix(sc.Text(), "event: "); ok {
				s.mu.Lock()
				s.events = append(s.events, typ)
				s.mu.Unlock()
			}
		}
	}()
	t.Cleanup(func() {
		cancel()
		_ = res.Body.Close()
		<-done
	})
	return s
}

// TestWebUI: the window opened through the UI's API takes a second
// controller; an override through PUT removes an endpoint, the stream
// reports it, the preview said so beforehand, and the file holds it.
func TestWebUI(t *testing.T) {
	e, u := newUI(t)
	e.commission()

	// The labels column is list-valued (H3 Q2): the UI filters by one
	// label, not by the combination.
	var sch struct {
		Columns []fabricui.Column `json:"columns"`
	}
	if code := u.do(t, http.MethodGet, "/exposure/schema", nil, &sch); code != http.StatusOK {
		t.Fatalf("schema %d", code)
	}
	multi := false
	for _, c := range sch.Columns {
		if c.Key == "labels" {
			multi = c.Multi && c.Filter
		}
	}
	if !multi {
		t.Fatalf("labels column %+v", sch.Columns)
	}

	items := u.items(t)
	if it := items["light.kitchen"]; !it.Enabled || it.Source != expose.SourceAssist || it.Group != "Kitchen ceiling" || it.Fields["domain"] != "light" || it.Verdict.State != "mappable" {
		t.Fatalf("light.kitchen %+v", it)
	}
	if it := items["sensor.processor_use"]; it.Enabled || it.Verdict.State == "" {
		t.Fatalf("an entity Assist leaves out %+v", it)
	}
	if it := items["light.cellar"]; it.Enabled || it.Source != expose.SourceDisabled || it.Verdict.State != "unmappable" {
		t.Fatalf("disabled light %+v", it)
	}

	// The window through the API; a second controller commissions.
	if code := u.do(t, http.MethodPost, "/pairing/window", map[string]any{}, nil); code != http.StatusOK {
		t.Fatalf("open window %d", code)
	}
	ctrl := e.site.AddController(fabrictest.ControllerOptions{})
	if _, err := ctrl.Commission(e.ctx, e.dev, fabrictest.CommissionOptions{}); err != nil {
		t.Fatalf("second controller: %v", err)
	}

	ev := u.stream(t)
	plug, ok := e.endpoint("switch.coffee")
	if !ok {
		t.Fatal("no plug")
	}
	off := map[string]any{"items": []map[string]any{{"key": "switch.coffee", "enabled": false}}}
	var preview struct{ Items []item }
	if code := u.do(t, http.MethodPost, "/exposure/preview", off, &preview); code != http.StatusOK || len(preview.Items) != 1 {
		t.Fatalf("preview %d %+v", code, preview)
	}
	if p := preview.Items[0]; p.Key != "switch.coffee" || p.Enabled || p.Source != expose.SourceOverrideOff {
		t.Fatalf("preview item %+v", p)
	}
	if _, ok := e.endpoint("switch.coffee"); !ok {
		t.Fatal("the preview changed the endpoints")
	}
	var applied struct{ Applied int }
	if code := u.do(t, http.MethodPut, "/exposure", off, &applied); code != http.StatusOK || applied.Applied != 1 {
		t.Fatalf("apply %d %+v", code, applied)
	}
	// Reassemble ran before the response.
	if _, ok := e.endpoint("switch.coffee"); ok {
		t.Fatal("the plug kept its endpoint")
	}
	if got := u.items(t)["switch.coffee"]; got.Enabled != preview.Items[0].Enabled || got.Source != preview.Items[0].Source || got.Verdict != preview.Items[0].Verdict {
		t.Fatalf("after apply %+v, the preview said %+v", got, preview.Items[0])
	}
	e.eventually("the stream reports the change", func() bool {
		return ev.has(fabricui.EventExposureChanged) && ev.has(fabricui.EventEndpointsChanged)
	})
	if b, _ := os.ReadFile(u.path); !bytes.Contains(b, []byte(`"switch.coffee"`)) || !bytes.Contains(b, []byte(`"enabled": false`)) {
		t.Fatalf("selection file %s", b)
	}

	// Back to what the rule says: the override goes, the endpoint
	// returns under its number (go-fabric v0.12.1 keeps a removed key's
	// number for its re-add).
	on := map[string]any{"items": []map[string]any{{"key": "switch.coffee", "enabled": true}}}
	if code := u.do(t, http.MethodPut, "/exposure", on, nil); code != http.StatusOK {
		t.Fatalf("apply on %d", code)
	}
	if again, ok := e.endpoint("switch.coffee"); !ok || again != plug {
		t.Fatalf("the plug is not back on its number (%d %v, was %d)", again, ok, plug)
	}
	if len(u.sel.Entries()) != 0 {
		t.Fatalf("overrides left %v", u.sel.Entries())
	}
	if it := u.items(t)["switch.coffee"]; it.Source != expose.SourceAssist {
		t.Fatalf("plug source %+v", it)
	}

	// A rename override names the endpoint; a disabled entity cannot be
	// turned on; an unknown one is refused.
	name := "Espresso"
	if code := u.do(t, http.MethodPut, "/exposure", map[string]any{"items": []map[string]any{{"key": "switch.coffee", "enabled": true, "name": name}}}, nil); code != http.StatusOK {
		t.Fatalf("rename %d", code)
	}
	if it := u.items(t)["switch.coffee"]; it.Name != name || it.Source != expose.SourceOverrideOn {
		t.Fatalf("renamed plug %+v", it)
	}
	// The rename is the node label as typed, without the device name
	// (hamapping.Options.Name, verbatim; H3 Q3).
	renamed, _ := e.endpoint("switch.coffee")
	e.eventually("the node label is the rename", func() bool {
		values, err := fabrictest.ReadCluster(e.ctx, e.pair, renamed, bridgedbidef.ClusterID)
		if err != nil {
			return false
		}
		st, _ := bridgedbidef.StateFrom(values)
		return st.NodeLabel == name
	})
	for _, key := range []string{"light.cellar", "light.nope"} {
		if code := u.do(t, http.MethodPut, "/exposure", map[string]any{"items": []map[string]any{{"key": key, "enabled": true}}}, nil); code != http.StatusBadRequest {
			t.Errorf("%s: %d, want 400", key, code)
		}
	}
}

// TestUIWarnings: the UI's status carries the half-valid configuration
// the log reports — a label value that names no label and an overrides
// file moved aside at start (fabricui.Options.Warnings, H3 side effect).
func TestUIWarnings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "selection.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	sel, err := selection.Open(path, logger())
	if err != nil {
		t.Fatal(err)
	}
	e := newEnv(t, expose.Rule{IncludeLabels: []string{"MATTER", "Later"}}, func(o *bridge.Options) { o.Selection = sel })
	srv := httptest.NewServer(fabricui.Handler(fabricui.Options{
		Node: e.dev.Node(), Exposure: e.br.Exposure(), Warnings: e.br.Warnings, Logger: logger(),
	}))
	t.Cleanup(srv.Close)
	res, err := http.Get(srv.URL + "/api/v1/node/status") //nolint:noctx // a test request
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	var st struct {
		Warnings []fabricui.Warning `json:"warnings"`
	}
	if err := json.NewDecoder(res.Body).Decode(&st); err != nil {
		t.Fatal(err)
	}
	codes := map[string]fabricui.Warning{}
	for _, w := range st.Warnings {
		codes[w.Code] = w
	}
	if w, ok := codes[bridge.WarningLabelUnresolved]; len(st.Warnings) != 2 || !ok || !strings.Contains(w.Message, `"Later"`) || w.Since.IsZero() {
		t.Fatalf("warnings %+v", st.Warnings)
	}
	if w, ok := codes[bridge.WarningSelectionCorrupt]; !ok || !strings.Contains(w.Message, ".corrupt-") || w.Since.IsZero() {
		t.Fatalf("warnings %+v", st.Warnings)
	}
	// The label's first sighting stays across syncs.
	first := codes[bridge.WarningLabelUnresolved].Since
	if err := e.br.Resync(e.ctx); err != nil {
		t.Fatal(err)
	}
	for _, w := range e.br.Warnings(e.ctx) {
		if w.Code == bridge.WarningLabelUnresolved && !w.Since.Equal(first) {
			t.Fatalf("since moved from %v to %v", first, w.Since)
		}
	}
}
