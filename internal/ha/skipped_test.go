// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package ha_test

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/SukramJ/go-ha2fabric/internal/expose"
	"github.com/SukramJ/go-ha2fabric/internal/ha"
	"github.com/SukramJ/go-ha2fabric/internal/ha/hatest"
	"github.com/SukramJ/go-ha2fabric/internal/plan"
)

// skippedEnv is a client on the fake Home Assistant and a reader of one
// entity's verdict under rule: exposed, and the reason.
func skippedEnv(t *testing.T, rule expose.Rule) (srv *hatest.Server, verdict func(id string) (bool, string, error)) {
	t.Helper()
	srv = hatest.New(t)
	c, r, _ := start(t, ha.Options{URL: srv.URL(), Token: hatest.Token, Logger: slog.New(slog.DiscardHandler)})
	r.wait(t, "connected")
	in := plan.Input{Rule: rule}
	return srv, func(id string) (bool, string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		snap, err := ha.ReadSnapshot(ctx, c, false)
		if err == nil {
			err = plan.Check(snap, in)
		}
		if err != nil {
			return false, "", err
		}
		vs, _ := plan.Assess(snap, in)
		for i := range vs {
			if vs[i].EntityID == id {
				return vs[i].Exposed, vs[i].ExposeReason, nil
			}
		}
		return false, "no verdict", nil
	}
}

// TestSkippedEntityEntryIsNotExposed (HR-P1-data-90): a hidden entity
// whose registry entry stops decoding is not exposed as an unregistered
// one by a rule that exposes everything.
func TestSkippedEntityEntryIsNotExposed(t *testing.T) {
	srv, verdict := skippedEnv(t, expose.Rule{Include: []string{"*"}})
	srv.UpdateEntity("light.kitchen", map[string]any{"hidden_by": "user"})
	if exposed, _, err := verdict("light.kitchen"); err != nil || exposed {
		t.Fatalf("hidden: exposed %v, %v", exposed, err)
	}
	srv.UpdateEntity("light.kitchen", map[string]any{"original_name": 5})
	if exposed, reason, err := verdict("light.kitchen"); err != nil || exposed {
		t.Fatalf("entry does not decode: exposed %v (%s), %v", exposed, reason, err)
	}
}

// TestSkippedDeviceEntryIsNotExposed (HR-P1-data-90): an entity whose
// device carries an exclude label is not exposed once the device's entry
// stops decoding.
func TestSkippedDeviceEntryIsNotExposed(t *testing.T) {
	srv, verdict := skippedEnv(t, expose.Rule{Include: []string{"*"}, ExcludeLabels: []string{"secret"}})
	srv.UpdateDevice("dev-hue-1", map[string]any{"labels": []any{"secret"}})
	if exposed, _, err := verdict("light.kitchen"); err != nil || exposed {
		t.Fatalf("labelled device: exposed %v, %v", exposed, err)
	}
	srv.UpdateDevice("dev-hue-1", map[string]any{"name": 7})
	if exposed, reason, err := verdict("light.kitchen"); err != nil || exposed {
		t.Fatalf("device entry does not decode: exposed %v (%s), %v", exposed, reason, err)
	}
}

// TestSkippedLabelEntryFailsTheSync (HR-P1-data-90): an exclude label
// configured by its name whose registry entry stops decoding does not
// let its entities through: the snapshot is refused (plan.Check).
func TestSkippedLabelEntryFailsTheSync(t *testing.T) {
	srv, verdict := skippedEnv(t, expose.Rule{Include: []string{"*"}, ExcludeLabels: []string{"Matter"}})
	if exposed, _, err := verdict("sensor.outdoor"); err != nil || exposed {
		t.Fatalf("labelled: exposed %v, %v", exposed, err)
	}
	srv.UpdateLabelFields("matter", map[string]any{"color": 3})
	if exposed, reason, err := verdict("sensor.outdoor"); err == nil {
		t.Fatalf("label entry does not decode: no error, exposed %v (%s)", exposed, reason)
	}
	// Without label values in the rule the snapshot is used.
	_, verdict = skippedEnv(t, expose.Rule{Include: []string{"*"}})
	if _, _, err := verdict("sensor.outdoor"); err != nil {
		t.Fatalf("a rule without labels: %v", err)
	}
}

// TestSkippedEntryWithoutID (HR-P1-data-90): an entity or device entry
// whose id cannot be read fails the snapshot rather than leaving an
// entity unjudged.
func TestSkippedEntryWithoutID(t *testing.T) {
	srv, verdict := skippedEnv(t, expose.Rule{Include: []string{"*"}})
	srv.UpdateDevice("dev-hue-1", map[string]any{"id": 7})
	if _, _, err := verdict("light.kitchen"); err == nil || !strings.HasPrefix(err.Error(), "config/device_registry/list: ") {
		t.Fatalf("device without an id: %v", err)
	}
	srv, verdict = skippedEnv(t, expose.Rule{Include: []string{"*"}})
	srv.UpdateEntity("light.kitchen", map[string]any{"entity_id": 7})
	if _, _, err := verdict("light.kitchen"); err == nil || !strings.HasPrefix(err.Error(), "config/entity_registry/list: ") {
		t.Fatalf("entity without an id: %v", err)
	}
}
