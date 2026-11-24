// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package selection

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/SukramJ/go-ha2fabric/internal/text"
)

var quietLog = slog.New(slog.DiscardHandler)

// FuzzOpen holds Open against any content of the overrides file: it never
// panics and never fails on content (only on the file system); a file that
// does not parse is moved aside and the store starts empty (there is no
// last-good copy), a file that parses — of any version — keeps its
// entries with an entity id for a key, and what Open read writes back and
// reads again, as this version, as the same entries.
func FuzzOpen(f *testing.F) {
	for _, s := range []string{
		`{"version":1,"entities":{"light.a":{"enabled":true,"name":"Lamp"},"switch.b":{"enabled":false}}}`,
		`{"version":2,"entities":{}}`, `{"version":1}`, `{"version":1,"entities":null}`,
		`{"version":1,"entities":{"":{"enabled":true,"name":"\u0000"}}}`, `{`, ``, `null`, `[]`,
	} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		path := filepath.Join(t.TempDir(), "selection.json")
		if err := os.WriteFile(path, b, 0o600); err != nil {
			t.Fatal(err)
		}
		s, err := Open(path, quietLog)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		if r, ok := s.Recovered(); ok && r.MovedTo != "" {
			if r.Restored == "" && len(s.Entries()) != 0 {
				t.Fatalf("recovered %+v with entries", r)
			}
			if _, err := os.Stat(r.MovedTo); err != nil {
				t.Fatalf("moved aside to %s: %v", r.MovedTo, err)
			}
			return
		}
		for id, e := range s.Entries() {
			if !EntityID(id) || len(e.Name) > MaxName || !text.Safe(e.Name) {
				t.Fatalf("entry %q %+v", id, e)
			}
		}
		want := s.Entries()
		if err := s.Set(nil, nil); err != nil {
			t.Fatalf("Set: %v", err)
		}
		again, err := Open(path, quietLog)
		if err != nil {
			t.Fatalf("Open again: %v", err)
		}
		if _, ok := again.Recovered(); ok || len(again.Entries()) != len(want) {
			t.Fatalf("round trip: %d entries, want %d", len(again.Entries()), len(want))
		}
		for id, e := range want {
			if again.Entries()[id] != e {
				t.Fatalf("round trip %q: %+v, want %+v", id, again.Entries()[id], e)
			}
		}
	})
}
