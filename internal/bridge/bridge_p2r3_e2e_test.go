// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package bridge_test

import (
	"strings"
	"testing"

	"github.com/SukramJ/go-ha2fabric/internal/expose"
	"github.com/SukramJ/go-ha2fabric/internal/ha"
)

// TestOversizedStateTakesTheEndpoint (HR-P2-bound-90, decision 52): a
// served entity whose state_changed comes over ha.MaxStateBytes loses
// its endpoint instead of being re-typed from its stripped state; a
// state under the cap again brings it back on its stored number.
func TestOversizedStateTakesTheEndpoint(t *testing.T) {
	e := newEnv(t, expose.Rule{Assist: true})
	ep := e.mustEndpoint("light.kitchen")
	attrs := e.attrsWith("light.kitchen", nil)
	big := e.attrsWith("light.kitchen", map[string]any{"blob": strings.Repeat("x", ha.MaxStateBytes)})
	e.ha.SetState("light.kitchen", "on", big)
	e.eventually("the oversized light's endpoint is gone", func() bool {
		_, ok := e.endpoint("light.kitchen")
		return !ok
	})
	if v := e.verdict("light.kitchen"); v.State != "unmappable" || !strings.HasPrefix(v.Reason, "state over 256 KiB") {
		t.Fatalf("light.kitchen: %s: %q", v.State, v.Reason)
	}
	delete(attrs, "blob")
	e.ha.SetState("light.kitchen", "on", attrs)
	e.eventually("the light back under the cap", func() bool {
		got, ok := e.endpoint("light.kitchen")
		return ok && got == ep
	})
}
