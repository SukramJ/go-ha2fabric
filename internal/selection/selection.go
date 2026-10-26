// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

// Package selection is the web UI's per-entity overrides, the daemon's
// own file (owner decision 0.4 of go-fabric concept v9): in the add-on
// /data/selection.json, beside the node's store, so the add-on's cold
// backup holds both. The file is
//
//	{"version": 1, "entities": {"<entity_id>": {"enabled": true, "name": "…"}}}
//
// written atomically (a temporary file in the same directory, then a
// rename) and read once at start. A file that does not parse is renamed
// aside and logged; the daemon starts without overrides rather than not
// at all.
package selection

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Version is the file format's version.
const Version = 1

// Entry is one entity's override.
type Entry struct {
	// Enabled exposes (true) or hides (false) the entity regardless of
	// the configured lists; a disabled or hidden registry entry stays
	// hidden.
	Enabled bool `json:"enabled"`
	// Name, when set, is the name the entity is exposed under.
	Name string `json:"name,omitempty"`
}

// file is the on-disk form.
type file struct {
	Version  int              `json:"version"`
	Entities map[string]Entry `json:"entities"`
}

// Store is the overrides and their file. The zero path keeps them in
// memory only.
type Store struct {
	path string
	log  *slog.Logger

	mu        sync.Mutex
	entries   map[string]Entry
	recovered *Recovery
}

// Recovery is an overrides file that did not parse at [Open] and was
// moved aside.
type Recovery struct {
	// MovedTo is the path the file was renamed to.
	MovedTo string
	// Err is why it did not parse.
	Err string
	// At is when it was moved.
	At time.Time
}

// Open reads the file at path; a missing file is no overrides. A file
// that does not parse, or has another version, is renamed to
// <path>.corrupt-<unix seconds> and logged, and the store starts empty.
func Open(path string, log *slog.Logger) (*Store, error) {
	if log == nil {
		log = slog.Default()
	}
	s := &Store{path: path, log: log, entries: map[string]Entry{}}
	if path == "" {
		return s, nil
	}
	b, err := os.ReadFile(path) //nolint:gosec // the daemon's own file
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("selection: %w", err)
	}
	var f file
	if err = json.Unmarshal(b, &f); err == nil && f.Version != Version {
		err = fmt.Errorf("version %d, want %d", f.Version, Version)
	}
	if err != nil {
		now := time.Now()
		aside := fmt.Sprintf("%s.corrupt-%d", path, now.Unix())
		if rerr := os.Rename(path, aside); rerr != nil {
			return nil, fmt.Errorf("selection: %s does not parse (%w) and cannot be moved aside: %w", path, err, rerr)
		}
		log.Warn("selection.corrupt", slog.String("file", path), slog.String("moved_to", aside), slog.String("error", err.Error()),
			slog.String("hint", "the web UI's overrides start empty"))
		s.recovered = &Recovery{MovedTo: aside, Err: err.Error(), At: now}
		return s, nil
	}
	for id, e := range f.Entities {
		s.entries[id] = e
	}
	return s, nil
}

// Recovered reports the file [Open] moved aside, if it did.
func (s *Store) Recovered() (Recovery, bool) {
	if s.recovered == nil {
		return Recovery{}, false
	}
	return *s.recovered, true
}

// Path is the file's path; empty for an in-memory store.
func (s *Store) Path() string { return s.path }

// Entries returns a copy of the overrides.
func (s *Store) Entries() map[string]Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	return maps.Clone(s.entries)
}

// Enabled returns the overrides as the exposure rule reads them.
func (s *Store) Enabled() map[string]bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]bool, len(s.entries))
	for id, e := range s.entries {
		out[id] = e.Enabled
	}
	return out
}

// Set stores the given overrides over the current ones, drops the
// overrides of remove, and writes the file; on a write error nothing
// changes.
func (s *Store) Set(updates map[string]Entry, remove []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := maps.Clone(s.entries)
	for _, id := range remove {
		delete(next, id)
	}
	maps.Copy(next, updates)
	if err := s.write(next); err != nil {
		return err
	}
	s.entries = next
	return nil
}

// write writes entries atomically: a temporary file in the target's
// directory, synced, then renamed over the target.
func (s *Store) write(entries map[string]Entry) error {
	if s.path == "" {
		return nil
	}
	b, err := json.MarshalIndent(file{Version: Version, Entities: entries}, "", "  ")
	if err != nil {
		return fmt.Errorf("selection: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), filepath.Base(s.path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("selection: %w", err)
	}
	name := tmp.Name()
	_, err = tmp.Write(append(b, '\n'))
	if err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(name, s.path)
	}
	if err != nil {
		_ = os.Remove(name)
		return fmt.Errorf("selection: %w", err)
	}
	return nil
}
