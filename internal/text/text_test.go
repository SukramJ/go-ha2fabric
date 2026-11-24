// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package text_test

import (
	"testing"

	"github.com/SukramJ/go-ha2fabric/internal/text"
)

func TestClean(t *testing.T) {
	for in, want := range map[string]string{
		"Living room":           "Living room",
		"  Lamp ":               "Lamp",
		"A\x00B\nC\x1b[31m":     "AB C[31m",
		"‮evil⁦⁩‎":              "evil",
		"V\x00\a\x1b]0;x\aACME": "V]0;xACME",
		"S\r\n9\u0085x\u009b":   "S  9 x",
		"می‌خو":                 "می‌خو", // ZWNJ kept
		"👩‍💻":                   "👩‍💻",   // ZWJ kept
		"\xff":                  "�",
	} {
		if got := text.Clean(in); got != want {
			t.Errorf("Clean(%q) = %q, want %q", in, got, want)
		}
		if !text.Safe(text.Clean(in)) {
			t.Errorf("Clean(%q) is not Safe", in)
		}
	}
}

func TestSafe(t *testing.T) {
	for s, want := range map[string]bool{
		"Alexa":    true,
		"Küche 2":  true,
		"a‍b":      true,
		"a\tb":     false,
		"a\x7fb":   false,
		"a؜b":      false,
		"a‪b":      false,
		"a⁩b":      false,
		"\xff":     false,
		"\u0085":   false,
		"plain  x": true,
	} {
		if got := text.Safe(s); got != want {
			t.Errorf("Safe(%q) = %v, want %v", s, got, want)
		}
	}
}

func FuzzClean(f *testing.F) {
	f.Add("A\x00B‮")
	f.Fuzz(func(t *testing.T, s string) {
		c := text.Clean(s)
		if !text.Safe(c) {
			t.Fatalf("Clean(%q) = %q is not Safe", s, c)
		}
		if text.Clean(c) != c {
			t.Fatalf("Clean is not idempotent on %q", s)
		}
	})
}
