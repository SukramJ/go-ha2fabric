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
// rename) and read once at start. Before each write the file it replaces
// is kept as <file>.bak, the last-good copy. A file of another version is
// read leniently (its entities in the known shape apply); a file that
// does not parse is renamed aside and logged, and the last-good copy is
// read instead and written back as the file — it loses at most the last
// change; so is a missing file beside a last-good copy. Only when neither
// helps does the daemon start without overrides (decision 25 of the
// 2026-11-23 review), rather than not at all.
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
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/SukramJ/go-ha2fabric/internal/text"
)

// Version is the file format's version.
const Version = 1

// BackupSuffix names the last-good copy beside the file.
const BackupSuffix = ".bak"

// MaxName bounds an override's name in bytes.
const MaxName = 256

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
	closed    bool
}

// ErrClosed is [Store.Set]'s error after [Store.Close].
var ErrClosed = errors.New("selection: the store is closed (its node was deleted)")

// Close ends the store's writes: a Set in flight finishes first, every
// later one fails with ErrClosed and writes nothing. A node's delete
// closes it before it removes the node's files, so a web UI apply racing
// the delete leaves no orphan file behind (HR-P2-conc-68).
func (s *Store) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
}

// Recovery is an overrides file [Open] did not read as it is: one that
// did not parse and was moved aside, one that was missing beside a
// last-good copy, or one of another version.
type Recovery struct {
	// MovedTo is the path the file was renamed to.
	MovedTo string
	// Missing is a file that was not there while its last-good copy was.
	Missing bool
	// Empty is a file of another version of which nothing was understood
	// ({} or null): no overrides apply.
	Empty bool
	// Err is why it did not parse.
	Err string
	// At is when it was moved.
	At time.Time
	// Restored is the last-good copy read instead; Version is the
	// version of a file read leniently.
	Restored string
	Version  int
}

// Message is the status page's text of the recovery: what was read, and
// what it means for the entities the web UI turned off.
func (r Recovery) Message() string {
	const exposed = "the web UI's overrides started empty, so entities turned off there are exposed again where the configured rule exposes them"
	switch {
	case r.Missing && r.Restored != "":
		return fmt.Sprintf("the overrides file was missing; the last-good copy %s was read instead and written back as the file, so the last change made in the web UI may be lost", r.Restored)
	case r.Missing:
		return fmt.Sprintf("the overrides file was missing and its last-good copy could not be read (%s): %s", r.Err, exposed)
	case r.MovedTo == "" && r.Empty:
		return fmt.Sprintf("the overrides file is version %d, not %d, and nothing in it was understood: %s", r.Version, Version, exposed)
	case r.MovedTo == "":
		return fmt.Sprintf("the overrides file is version %d, not %d: its overrides were read as this version understands them and apply; saving the selection rewrites it as version %d", r.Version, Version, Version)
	case r.Restored != "":
		return fmt.Sprintf("the overrides file did not parse (%s) and was moved to %s; the last-good copy %s was read instead and written back as the file, so the last change made in the web UI may be lost", r.Err, r.MovedTo, r.Restored)
	default:
		return fmt.Sprintf("the overrides file did not parse (%s) and was moved to %s, and no last-good copy could be read: the web UI's overrides started empty, so entities turned off there are exposed again where the configured rule exposes them", r.Err, r.MovedTo)
	}
}

// Open reads the file at path; a missing file without a last-good copy is
// no overrides. A file of another version is read leniently and reported
// by [Store.Recovered]. A file that does not parse is renamed to
// <path>.corrupt-<unix seconds> and logged, and the last-good copy
// <path>.bak is read instead and written back as the file, so the next
// start reads it too; a missing file beside a last-good copy alike. When
// the copy does not parse either, the store starts empty. Keys that are
// not entity ids are dropped and names cut to MaxName.
func Open(path string, log *slog.Logger) (*Store, error) {
	if log == nil {
		log = slog.Default()
	}
	s := &Store{path: path, log: log, entries: map[string]Entry{}}
	if path == "" {
		return s, nil
	}
	now := time.Now()
	b, err := os.ReadFile(path) //nolint:gosec // the daemon's own file
	if errors.Is(err, fs.ErrNotExist) {
		return s.openMissing(now), nil
	}
	if err != nil {
		return nil, fmt.Errorf("selection: %w", err)
	}
	f, err := parse(b)
	if err == nil {
		s.load(f.Entities)
		if f.Version != Version {
			s.recovered = &Recovery{Err: fmt.Sprintf("version %d, want %d", f.Version, Version), At: now, Version: f.Version, Empty: len(f.Entities) == 0}
			log.Warn("selection.version", slog.String("file", path), slog.Int("version", f.Version), slog.Int("entries", len(s.entries)),
				slog.String("hint", s.recovered.Message()))
		}
		return s, nil
	}
	aside := fmt.Sprintf("%s.corrupt-%d", path, now.Unix())
	if rerr := os.Rename(path, aside); rerr != nil {
		return nil, fmt.Errorf("selection: %s does not parse (%w) and cannot be moved aside: %w", path, err, rerr)
	}
	s.recovered = &Recovery{MovedTo: aside, Err: err.Error(), At: now}
	if bb, berr := os.ReadFile(path + BackupSuffix); berr == nil { //nolint:gosec // the daemon's own file
		if bf, perr := parse(bb); perr == nil {
			s.restore(bf, bb)
		}
	}
	log.Warn("selection.corrupt", slog.String("file", path), slog.String("moved_to", aside), slog.String("error", err.Error()),
		slog.String("restored", s.recovered.Restored), slog.String("hint", s.recovered.Message()))
	return s, nil
}

// openMissing is [Open] of a missing file: no overrides, unless a
// last-good copy is there — a write once happened, so the file was lost —
// which is read and written back.
func (s *Store) openMissing(now time.Time) *Store {
	bb, err := os.ReadFile(s.path + BackupSuffix) //nolint:gosec // the daemon's own file
	if errors.Is(err, fs.ErrNotExist) {
		return s
	}
	s.recovered = &Recovery{Missing: true, At: now}
	if err == nil {
		var bf file
		if bf, err = parse(bb); err == nil {
			s.restore(bf, bb)
		}
	}
	if err != nil {
		s.recovered.Err = err.Error()
	}
	s.log.Warn("selection.missing", slog.String("file", s.path), slog.String("restored", s.recovered.Restored), slog.String("hint", s.recovered.Message()))
	return s
}

// restore loads the last-good copy bf (read as b) and writes it back as
// the file, so the next start reads it as the file; a failed write is
// logged, and the next start falls back to the copy again.
func (s *Store) restore(bf file, b []byte) {
	s.load(bf.Entities)
	s.recovered.Restored, s.recovered.Version = s.path+BackupSuffix, bf.Version
	if err := atomicWrite(s.path, b); err != nil {
		s.log.Warn("selection.restore_failed", slog.String("file", s.path), slog.String("error", err.Error()))
	}
}

// parse reads a file's content leniently: any version, unknown fields
// ignored; an error when it is not JSON of the known shape.
func parse(b []byte) (file, error) {
	var f file
	err := json.Unmarshal(b, &f)
	return f, err
}

// load takes entries into the store: a key that is not an entity id is
// dropped, a name cut to MaxName on a character boundary and cleaned
// (text.Clean).
func (s *Store) load(entries map[string]Entry) {
	dropped := 0
	for id, e := range entries {
		if !EntityID(id) {
			dropped++
			continue
		}
		e.Name = text.Clean(cut(e.Name, MaxName))
		s.entries[id] = e
	}
	if dropped > 0 {
		s.log.Warn("selection.keys_dropped", slog.String("file", s.path), slog.Int("dropped", dropped),
			slog.String("hint", "keys that are not entity ids (domain.object_id) were ignored"))
	}
}

// EntityID reports whether id has an entity id's form: a domain and an
// object id of lower-case letters, digits and underscores, joined by one
// dot. It is a plausibility check of the file's keys, looser than Home
// Assistant's own.
func EntityID(id string) bool {
	domain, object, ok := strings.Cut(id, ".")
	if !ok || domain == "" || object == "" {
		return false
	}
	for _, r := range domain + object {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}

// cut is s cut to at most n bytes on a character boundary.
func cut(s string, n int) string {
	if len(s) <= n {
		return s
	}
	s = s[:n]
	for s != "" && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
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

// Names returns the overrides' names as the plan reads them: the
// entities with a name, by entity id.
func (s *Store) Names() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string]string{}
	for id, e := range s.entries {
		if e.Name != "" {
			out[id] = e.Name
		}
	}
	return out
}

// Set stores the given overrides over the current ones, drops the
// overrides of remove, and writes the file; on a write error nothing
// changes. A key that is not an entity id, a name longer than MaxName,
// or one with a control or bidirectional control character (it becomes
// a NodeLabel, text.Safe; HR-P2-data-31) is refused.
func (s *Store) Set(updates map[string]Entry, remove []string) error {
	for id, e := range updates {
		if !EntityID(id) {
			return fmt.Errorf("selection: %q is not an entity id", id)
		}
		if len(e.Name) > MaxName {
			return fmt.Errorf("selection: the name of %s is %d bytes, at most %d", id, len(e.Name), MaxName)
		}
		if !text.Safe(e.Name) {
			return fmt.Errorf("selection: the name of %s, %q, carries a control or bidirectional control character", id, e.Name)
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
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

// write writes entries atomically, after keeping the file it replaces as
// the last-good copy when that parses (a damaged file never replaces the
// copy). A copy that cannot be written is logged; the write goes on.
func (s *Store) write(entries map[string]Entry) error {
	if s.path == "" {
		return nil
	}
	b, err := json.MarshalIndent(file{Version: Version, Entities: entries}, "", "  ")
	if err != nil {
		return fmt.Errorf("selection: %w", err)
	}
	if cur, rerr := os.ReadFile(s.path); rerr == nil { //nolint:gosec // the daemon's own file
		if _, perr := parse(cur); perr == nil {
			if berr := atomicWrite(s.path+BackupSuffix, cur); berr != nil {
				s.log.Warn("selection.backup_failed", slog.String("file", s.path+BackupSuffix), slog.String("error", berr.Error()))
			}
		}
	}
	return atomicWrite(s.path, append(b, '\n'))
}

// atomicWrite writes b to path: a temporary file in the target's
// directory (mode 0600), synced, then renamed over the target.
func atomicWrite(path string, b []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("selection: %w", err)
	}
	name := tmp.Name()
	_, err = tmp.Write(b)
	if err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(name, path)
	}
	if err != nil {
		_ = os.Remove(name)
		return fmt.Errorf("selection: %w", err)
	}
	return nil
}
