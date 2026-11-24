// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package ha_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"thingfabric.dev/go-ha2fabric/internal/ha"
)

// TestErrorTextIsCut (HR-P2-bound-39): Home Assistant's error text — an
// integration's exception message — is cut where it becomes an error, so
// the log ring, stderr and the status hold a bounded record.
func TestErrorTextIsCut(t *testing.T) {
	long := strings.Repeat("é", 1<<19) // 1 MiB of two-byte runes
	e := &ha.Error{Code: "home_assistant_error" + long, Message: "boom " + long}
	got := e.Error()
	if len(got) > 2*ha.MaxErrorText+100 || !utf8.ValidString(got) || !strings.Contains(got, "more bytes)") || !strings.HasPrefix(got, "home assistant: home_assistant_error") {
		t.Fatalf("an error of %d bytes: %.200q", len(got), got)
	}
	if short := (&ha.Error{Code: "not_found", Message: "Entity not found"}).Error(); short != "home assistant: not_found: Entity not found" {
		t.Fatalf("a short error: %q", short)
	}
	if got := ha.Excerpt("abc"); got != "abc" {
		t.Fatalf("Excerpt of a short text: %q", got)
	}
	if got := ha.Excerpt(long); len(got) > ha.MaxErrorText+40 || !utf8.ValidString(got) {
		t.Fatalf("Excerpt of %d bytes: %d bytes", len(long), len(got))
	}
}
