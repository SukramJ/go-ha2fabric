// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

// Package text is the daemon's rule for free text that reaches a
// controller or a terminal: Home Assistant's names, manufacturers and
// serials, a web UI name, a node's label (HR-P2-data-31, decision 41).
// Such text carries no control character and no bidirectional control:
// a controller's app shows it as it is, and a terminal runs an escape
// sequence. The zero-width joiners U+200C and U+200D stay — they join
// Persian, Indic and emoji sequences.
//
// Text from Home Assistant is cleaned ([Clean]); text the operator types
// is refused when it is not [Safe].
package text

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Unsafe reports whether free text must not carry r: a control character
// (C0, DEL, C1: unicode.IsControl) or a bidirectional control — the
// Arabic letter mark U+061C, the marks U+200E and U+200F, the embeddings
// and overrides U+202A to U+202E and the isolates U+2066 to U+2069
// (Unicode Standard Annex #9, table 4) — or the line and paragraph
// separators U+2028 and U+2029 (Zl, Zp), which break a line as LF does
// (HR-P4-data-22).
func Unsafe(r rune) bool {
	switch {
	case unicode.IsControl(r):
		return true
	case r == 0x061C, r == 0x200E, r == 0x200F:
		return true
	case r >= 0x202A && r <= 0x202E, r >= 0x2066 && r <= 0x2069:
		return true
	case r == 0x2028, r == 0x2029:
		return true
	}
	return false
}

// Safe reports whether s is UTF-8 without an [Unsafe] rune.
func Safe(s string) bool {
	return utf8.ValidString(s) && !strings.ContainsFunc(s, Unsafe)
}

// Clean is s without its [Unsafe] runes, trimmed: a white-space control
// (a tab, a line break) becomes a space, so the words it separated stay
// apart; every other one is dropped. Invalid UTF-8 becomes U+FFFD.
func Clean(s string) string {
	if Safe(s) {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		switch {
		case !Unsafe(r):
			return r
		case unicode.IsSpace(r):
			return ' '
		}
		return -1
	}, s))
}
