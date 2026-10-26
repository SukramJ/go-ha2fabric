// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package selection

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fileName = "selection.json"

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), fileName)
	s, err := Open(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Entries()) != 0 || s.Path() != path {
		t.Fatalf("a missing file: %v %q", s.Entries(), s.Path())
	}
	if err := s.Set(map[string]Entry{"light.a": {Enabled: true, Name: "Desk"}, "switch.b": {}}, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.Set(map[string]Entry{"light.c": {Enabled: true}}, []string{"switch.b"}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte(`"version": 1`)) || !bytes.Contains(b, []byte(`"name": "Desk"`)) || bytes.Contains(b, []byte("switch.b")) {
		t.Fatalf("file %s", b)
	}
	matches, _ := filepath.Glob(path + ".tmp-*")
	if len(matches) != 0 {
		t.Fatalf("temporary files left: %v", matches)
	}
	again, err := Open(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if e := again.Entries(); len(e) != 2 || e["light.a"].Name != "Desk" || !e["light.c"].Enabled {
		t.Fatalf("read back %v", e)
	}
	if en := again.Enabled(); !en["light.a"] || len(en) != 2 {
		t.Fatalf("Enabled %v", en)
	}
}

func TestCorruptFileIsMovedAside(t *testing.T) {
	for name, body := range map[string]string{"garbage": "{not json", "version": `{"version": 2, "entities": {}}`} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, fileName)
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			var log bytes.Buffer
			s, err := Open(path, slog.New(slog.NewTextHandler(&log, nil)))
			if err != nil {
				t.Fatal(err)
			}
			if len(s.Entries()) != 0 {
				t.Fatalf("entries %v", s.Entries())
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("the corrupt file stayed: %v", err)
			}
			aside, _ := filepath.Glob(path + ".corrupt-*")
			if len(aside) != 1 || !strings.Contains(log.String(), "selection.corrupt") {
				t.Fatalf("aside %v, log %q", aside, log.String())
			}
			if r, ok := s.Recovered(); !ok || r.MovedTo != aside[0] || r.Err == "" || r.At.IsZero() {
				t.Fatalf("recovered %+v %v", r, ok)
			}
		})
	}
}

func TestWriteFailureChangesNothing(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "missing", fileName), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Set(map[string]Entry{"light.a": {Enabled: true}}, nil); err == nil {
		t.Fatal("a write into a missing directory succeeded")
	}
	if len(s.Entries()) != 0 {
		t.Fatalf("entries after a failed write %v", s.Entries())
	}
	mem, _ := Open("", nil)
	if err := mem.Set(map[string]Entry{"light.a": {Enabled: true}}, nil); err != nil || !mem.Enabled()["light.a"] {
		t.Fatalf("in-memory store: %v %v", err, mem.Enabled())
	}
}

func TestUnreadableFile(t *testing.T) {
	dir := t.TempDir()
	if _, err := Open(dir, nil); err == nil {
		t.Fatal("a directory opened as the file")
	}
}
