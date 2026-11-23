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

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestOtherVersionReadLeniently (HR-P1-err-06, decision 25): a file of
// another version whose entities have the known shape is honoured — a
// hidden entity stays hidden — with a warning, and stays where it is.
func TestOtherVersionReadLeniently(t *testing.T) {
	for name, body := range map[string]string{
		"later":      `{"version": 2, "entities": {"lock.front_door": {"enabled": false, "colour": "red"}}, "more": [1]}`,
		"no version": `{"entities": {"lock.front_door": {"enabled": false}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), fileName)
			writeFile(t, path, body)
			var log bytes.Buffer
			s, err := Open(path, slog.New(slog.NewTextHandler(&log, nil)))
			if err != nil {
				t.Fatal(err)
			}
			if e, ok := s.Entries()["lock.front_door"]; !ok || e.Enabled {
				t.Fatalf("entries %v", s.Entries())
			}
			r, ok := s.Recovered()
			if !ok || r.MovedTo != "" || r.Version == Version || !strings.Contains(r.Message(), "version") {
				t.Fatalf("recovered %+v %v", r, ok)
			}
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("the file was moved: %v", err)
			}
			if !strings.Contains(log.String(), "selection.version") {
				t.Fatalf("log %q", log.String())
			}
		})
	}
}

// TestCorruptFallsBackToLastGood (HR-P1-err-06, decision 25): each write
// keeps the file it replaces as selection.json.bak; a file that does not
// parse is moved aside and the last-good copy is read, losing at most the
// last change.
func TestCorruptFallsBackToLastGood(t *testing.T) {
	path := filepath.Join(t.TempDir(), fileName)
	s, err := Open(path, quietLog)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Set(map[string]Entry{"lock.front_door": {Enabled: false}}, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.Set(map[string]Entry{"light.kitchen": {Enabled: true}}, nil); err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, `{"version": 1, "entit`)
	var log bytes.Buffer
	s, err = Open(path, slog.New(slog.NewTextHandler(&log, nil)))
	if err != nil {
		t.Fatal(err)
	}
	got := s.Entries()
	if e, ok := got["lock.front_door"]; !ok || e.Enabled || len(got) != 1 {
		t.Fatalf("entries %v", got)
	}
	r, ok := s.Recovered()
	if !ok || r.MovedTo == "" || r.Restored != path+BackupSuffix || !strings.Contains(r.Message(), "last change") {
		t.Fatalf("recovered %+v %v: %s", r, ok, r.Message())
	}
	if !strings.Contains(log.String(), "selection.corrupt") {
		t.Fatalf("log %q", log.String())
	}
}

// TestRestoreLastsPastOneStart (HR-P1-err-91, decision 25): the
// last-good copy read at one start is written back, so the next start
// reads it as the file; and a missing file beside a last-good copy reads
// the copy and says so.
func TestRestoreLastsPastOneStart(t *testing.T) {
	path := filepath.Join(t.TempDir(), fileName)
	s, err := Open(path, quietLog)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Set(map[string]Entry{"light.a": {Enabled: false}}, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.Set(map[string]Entry{"light.b": {Enabled: false}}, nil); err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, `{trunc`)
	if _, err := Open(path, quietLog); err != nil {
		t.Fatal(err)
	}
	s2, err := Open(path, quietLog)
	if err != nil {
		t.Fatal(err)
	}
	if got := s2.Entries(); len(got) != 1 || got["light.a"].Enabled {
		t.Fatalf("second start: entries %v", got)
	}
	if r, ok := s2.Recovered(); ok {
		t.Fatalf("second start: recovered %+v", r)
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	s3, err := Open(path, quietLog)
	if err != nil {
		t.Fatal(err)
	}
	r, ok := s3.Recovered()
	if got := s3.Entries(); len(got) != 1 || !ok || r.Restored != path+BackupSuffix || !strings.Contains(r.Message(), "missing") {
		t.Fatalf("missing file: entries %v, recovered %+v %v: %q", got, r, ok, r.Message())
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("missing file not written back: %v", err)
	}
}

// TestNothingUnderstoodWarns (HR-P1-cfg-95): a file that parses but
// carries nothing this version understands ({} or null) says that the
// overrides started empty, not that they apply.
func TestNothingUnderstoodWarns(t *testing.T) {
	for _, body := range []string{`{}`, `null`, `{"version": 2}`} {
		path := filepath.Join(t.TempDir(), fileName)
		writeFile(t, path, body)
		s, err := Open(path, quietLog)
		if err != nil {
			t.Fatal(err)
		}
		r, ok := s.Recovered()
		if !ok || !strings.Contains(r.Message(), "exposed again") || strings.Contains(r.Message(), "apply") {
			t.Fatalf("%s: recovered %+v %v: %q", body, r, ok, r.Message())
		}
	}
}

// TestCorruptBackupIsNotKept (HR-P1-err-06): a write over a file that
// does not parse keeps the last-good copy rather than the damaged file.
func TestCorruptBackupIsNotKept(t *testing.T) {
	path := filepath.Join(t.TempDir(), fileName)
	s, err := Open(path, quietLog)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Set(map[string]Entry{"lock.front_door": {Enabled: false}}, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.Set(map[string]Entry{"light.kitchen": {Enabled: true}}, nil); err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, `{`)
	if err := s.Set(map[string]Entry{"light.hall": {Enabled: true}}, nil); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path + BackupSuffix)
	if err != nil || !strings.Contains(string(b), "lock.front_door") {
		t.Fatalf("backup %q %v", b, err)
	}
}

// TestNothingToRecoverWarns (HR-P1-err-06): neither the file nor a
// last-good copy parses — the store starts empty and the warning says
// that entities turned off are exposed again.
func TestNothingToRecoverWarns(t *testing.T) {
	path := filepath.Join(t.TempDir(), fileName)
	writeFile(t, path, `{not json`)
	writeFile(t, path+BackupSuffix, `also not`)
	s, err := Open(path, quietLog)
	if err != nil {
		t.Fatal(err)
	}
	r, ok := s.Recovered()
	if !ok || len(s.Entries()) != 0 || r.Restored != "" || !strings.Contains(r.Message(), "exposed again") {
		t.Fatalf("recovered %+v %v: %q", r, ok, r.Message())
	}
}

// TestEntityKeys (HR-P1-data-20): Open drops a key that is not an entity
// id, and Set refuses one; a name longer than MaxName is cut at Open and
// refused by Set.
func TestEntityKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), fileName)
	long := strings.Repeat("é", MaxName)
	writeFile(t, path, `{"version": 1, "entities": {"": {"enabled": true}, "nodot": {"enabled": true}, "Light.A": {"enabled": true}, "light.a": {"enabled": true, "name": "`+long+`"}}}`)
	s, err := Open(path, quietLog)
	if err != nil {
		t.Fatal(err)
	}
	got := s.Entries()
	if len(got) != 1 || len(got["light.a"].Name) > MaxName || got["light.a"].Name == "" {
		t.Fatalf("entries %v", got)
	}
	if err := s.Set(map[string]Entry{"not an id": {Enabled: true}}, nil); err == nil {
		t.Fatal("Set took a key that is not an entity id")
	}
	if err := s.Set(map[string]Entry{"light.b": {Enabled: true, Name: long}}, nil); err == nil {
		t.Fatal("Set took an over-long name")
	}
	if err := s.Set(map[string]Entry{"light.b": {Enabled: true, Name: "Lamp"}}, nil); err != nil {
		t.Fatal(err)
	}
}
