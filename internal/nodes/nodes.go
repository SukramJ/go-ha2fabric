// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

// Package nodes is the daemon's node registry (go-fabric-ui contract
// 0.6.0, "Node registry"; concept U6, C.3): the nodes beside the
// configured one, created, renamed and deleted in the web UI. The first
// node is the daemon's configuration (its flags, the add-on options);
// every further one is registry state in nodes.json beside the store:
//
//	{"nodes": [{"key": "alexa", "label": "Alexa", "listen": ":5541",
//	  "discriminator": 1234, "passcode": 34567890,
//	  "created_at": "2026-11-05T10:00:00Z"}]}
//
// A node keeps its own store, ha2fabric-<key>.db, and its own selection,
// selection-<key>.json, in the same directory. The registry allocates a
// new node's values: the key, a unique slug of the label in the
// contract's key pattern; the port, the next free one from the
// configured node's port + 1; the discriminator, random in 12 bits and
// unique among the nodes; and the passcode, random and valid
// ([node.ValidateSetupPIN]). The file holds the passcodes, so it is
// written 0600, atomically (a temporary file in the same directory, then a
// rename), like the store beside it.
//
// The registry is the file and the allocation; starting and stopping the
// nodes is the daemon's (cmd/ha2fabric).
package nodes

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	fabricui "github.com/SukramJ/go-fabric-ui/fabricui"
	"github.com/SukramJ/go-fabric/node"
)

// File is the registry's file name, beside the store.
const File = "nodes.json"

// DefaultPort is the port a configured listen address without one
// (or with the ephemeral port 0) counts from: node.Network's default.
const DefaultPort = 5540

// maxKey is the longest key the contract's pattern takes
// (^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$).
const maxKey = 64

// reservedKeys are the node family's page names and the default node's
// key, which a registry key may not be (go-fabric-ui fabricui/handler.go
// reservedNodeKeys and options.go DefaultNodeKey at v0.6.0).
var reservedKeys = []string{
	"status", "pair", "fabrics", "selection", "endpoints", "endpoint", "surface", "diagnostics", "topology",
	fabricui.DefaultNodeKey,
}

// Node is one registry node as nodes.json holds it.
type Node struct {
	Key           string    `json:"key"`
	Label         string    `json:"label"`
	Listen        string    `json:"listen"`
	Discriminator uint16    `json:"discriminator"`
	Passcode      uint32    `json:"passcode"`
	CreatedAt     time.Time `json:"created_at"`
}

// StoreFile is the node's store file name: ha2fabric-<key>.db.
func (n *Node) StoreFile() string { return "ha2fabric-" + n.Key + ".db" }

// SelectionFile is the node's selection file name: selection-<key>.json.
func (n *Node) SelectionFile() string { return "selection-" + n.Key + ".json" }

// file is the on-disk form.
type file struct {
	Nodes []Node `json:"nodes"`
}

// Registry is the registry file and its nodes, in the order they were
// created. Its methods are safe for concurrent use.
type Registry struct {
	dir string
	log *slog.Logger

	// rand and free are the allocation's sources; tests replace them.
	rand io.Reader
	free func(network, addr string) bool
	now  func() time.Time

	mu    sync.Mutex
	nodes []Node
}

// ErrNotFound is a key the registry does not hold.
var ErrNotFound = errors.New("nodes: no such node")

// Open reads dir/nodes.json; a missing file is no node. A file that does
// not parse is renamed to nodes.json.corrupt-<unix seconds> and logged,
// and the registry starts empty — the nodes' stores stay on disk, and no
// new node takes a key whose files exist (see [Registry.Allocate]), so
// restoring the file brings them back.
func Open(dir string, log *slog.Logger) (*Registry, error) {
	if log == nil {
		log = slog.Default()
	}
	r := &Registry{dir: dir, log: log, rand: rand.Reader, free: bindable, now: time.Now}
	b, err := os.ReadFile(r.Path(File))
	if errors.Is(err, fs.ErrNotExist) {
		return r, nil
	}
	if err != nil {
		return nil, fmt.Errorf("nodes: %w", err)
	}
	var f file
	if err := json.Unmarshal(b, &f); err != nil || f.check() != nil {
		if err == nil {
			err = f.check()
		}
		moved := r.Path(File) + ".corrupt-" + strconv.FormatInt(r.now().Unix(), 10)
		if rerr := os.Rename(r.Path(File), moved); rerr != nil {
			return nil, fmt.Errorf("nodes: %s does not parse (%w) and cannot be moved aside: %w", r.Path(File), err, rerr)
		}
		log.Error("nodes.corrupt", slog.String("moved_to", moved), slog.String("error", err.Error()),
			slog.String("hint", "the daemon starts with the configured node only; the nodes' stores stay on disk, restore the file to bring them back"))
		return r, nil
	}
	r.nodes = f.Nodes
	return r, nil
}

// check refuses a file whose nodes are not usable: an invalid or repeated
// key, an empty listen address, an invalid passcode or discriminator.
func (f *file) check() error {
	seen := map[string]bool{}
	for i := range f.Nodes {
		n := &f.Nodes[i]
		switch {
		case !validKey(n.Key) || seen[n.Key]:
			return fmt.Errorf("node %d: key %q is invalid or repeated", i, n.Key)
		case n.Listen == "":
			return fmt.Errorf("node %s: no listen address", n.Key)
		case n.Discriminator > node.MaxDiscriminator:
			return fmt.Errorf("node %s: discriminator %d is not 12 bits", n.Key, n.Discriminator)
		case node.ValidateSetupPIN(n.Passcode) != nil:
			return fmt.Errorf("node %s: the passcode is invalid", n.Key)
		}
		seen[n.Key] = true
	}
	return nil
}

// Path is name in the registry's directory.
func (r *Registry) Path(name string) string { return filepath.Join(r.dir, name) }

// Nodes is every node, in the order they were created.
func (r *Registry) Nodes() []Node {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.nodes)
}

// Len is the number of nodes.
func (r *Registry) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.nodes)
}

// Configured is what the allocation keeps clear of: the configured
// node's listen address and discriminator.
type Configured struct {
	Listen        string
	Discriminator uint16
	// TCP also probes the port over TCP: the node accepts Matter over TCP
	// on it (the camera, config.Config.Node).
	TCP bool
}

// Allocate returns a new node for label: its key, port, discriminator and
// passcode, the creation time. It persists nothing — [Registry.Add] does,
// once the node runs. The port is the first from the configured node's
// port + 1 that no registry node names and that binds now (UDP, and TCP
// with c.TCP); the key is the label's slug, with -2, -3, … when a node,
// a reserved name or a file on disk has it.
func (r *Registry) Allocate(label string, c Configured) (Node, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := Node{Label: label, CreatedAt: r.now().UTC().Truncate(time.Second)}
	var err error
	if n.Key, err = r.key(label); err != nil {
		return Node{}, err
	}
	if n.Listen, err = r.port(c); err != nil {
		return Node{}, err
	}
	if n.Discriminator, err = r.discriminator(c.Discriminator); err != nil {
		return Node{}, err
	}
	if n.Passcode, err = r.passcode(); err != nil {
		return Node{}, err
	}
	return n, nil
}

// fold transliterates the letters a label commonly carries before Slug
// strips what is left: German umlauts and ß to their two-letter forms,
// Latin letters with a diacritic to their base letter (the module does
// not depend on golang.org/x/text, so a table rather than NFD). Keys are
// lower case.
var fold = strings.NewReplacer(
	"ä", "ae", "ö", "oe", "ü", "ue", "ß", "ss",
	"à", "a", "á", "a", "â", "a", "ã", "a", "å", "a", "ā", "a", "ą", "a", "æ", "ae",
	"ç", "c", "ć", "c", "č", "c",
	"ď", "d", "đ", "d", "ð", "d",
	"è", "e", "é", "e", "ê", "e", "ë", "e", "ē", "e", "ę", "e", "ě", "e",
	"ì", "i", "í", "i", "î", "i", "ï", "i", "ī", "i", "ı", "i",
	"ł", "l", "ľ", "l",
	"ñ", "n", "ń", "n", "ň", "n",
	"ò", "o", "ó", "o", "ô", "o", "õ", "o", "ø", "o", "ō", "o", "ő", "o", "œ", "oe",
	"ř", "r",
	"ś", "s", "š", "s", "ş", "s",
	"ť", "t", "ţ", "t", "þ", "th",
	"ù", "u", "ú", "u", "û", "u", "ū", "u", "ů", "u", "ű", "u",
	"ý", "y", "ÿ", "y",
	"ź", "z", "ż", "z", "ž", "z",
)

// Slug is label as a key of the contract's pattern: transliterated
// ([fold]), lower-case letters and digits kept, every other run of
// characters one '-', no '-' at either end, at most 56 characters (room
// for a suffix); "node" when nothing is left.
func Slug(label string) string {
	var b strings.Builder
	dash := false
	for _, c := range fold.Replace(strings.ToLower(label)) {
		switch {
		case c >= 'a' && c <= 'z' || c >= '0' && c <= '9':
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			dash = false
			b.WriteRune(c)
		default:
			dash = true
		}
		if b.Len() >= maxKey-8 {
			break
		}
	}
	if b.Len() == 0 {
		return "node"
	}
	return b.String()
}

// validKey reports whether key matches the contract's pattern and is not
// reserved.
func validKey(key string) bool {
	if key == "" || len(key) > maxKey || slices.Contains(reservedKeys, key) {
		return false
	}
	for i, c := range key {
		alnum := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
		if !alnum && (i == 0 || c != '.' && c != '_' && c != '-') {
			return false
		}
	}
	return true
}

// key is the first free key of label's slug; r.mu is held.
func (r *Registry) key(label string) (string, error) {
	base := Slug(label)
	for i := 1; i < 1000; i++ {
		k := base
		if i > 1 {
			k = base + "-" + strconv.Itoa(i)
		}
		if !validKey(k) || slices.ContainsFunc(r.nodes, func(n Node) bool { return n.Key == k }) || r.onDisk(k) {
			continue
		}
		return k, nil
	}
	return "", fmt.Errorf("nodes: no free key for %q", label)
}

// onDisk reports whether a store or selection file of key exists: a node
// of a lost registry file keeps its files, and a new node must not take
// them over.
func (r *Registry) onDisk(key string) bool {
	n := Node{Key: key}
	for _, name := range []string{n.StoreFile(), n.SelectionFile()} {
		if _, err := os.Stat(r.Path(name)); err == nil {
			return true
		}
	}
	return false
}

// port is the first free port from the configured one + 1; r.mu is held.
func (r *Registry) port(c Configured) (string, error) {
	host, base := "", DefaultPort
	if h, p, err := net.SplitHostPort(c.Listen); err == nil {
		host = h
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			base = v
		}
	}
	taken := map[int]bool{base: true}
	for i := range r.nodes {
		if _, p, err := net.SplitHostPort(r.nodes[i].Listen); err == nil {
			if v, err := strconv.Atoi(p); err == nil {
				taken[v] = true
			}
		}
	}
	for p := base + 1; p <= 65535; p++ {
		if taken[p] {
			continue
		}
		addr := net.JoinHostPort(host, strconv.Itoa(p))
		if r.free("udp", addr) && (!c.TCP || r.free("tcp", addr)) {
			return addr, nil
		}
	}
	return "", fmt.Errorf("nodes: no free port above %d", base)
}

// bindable reports whether addr binds now on network.
func bindable(network, addr string) bool {
	if network == "tcp" {
		l, err := net.Listen("tcp", addr) //nolint:noctx // a probe, closed at once
		if err != nil {
			return false
		}
		_ = l.Close()
		return true
	}
	c, err := net.ListenPacket("udp", addr) //nolint:noctx // a probe, closed at once
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}

// discriminator is a random 12-bit value no node has; r.mu is held.
func (r *Registry) discriminator(configured uint16) (uint16, error) {
	taken := map[uint16]bool{configured: true}
	for i := range r.nodes {
		taken[r.nodes[i].Discriminator] = true
	}
	for range 1000 {
		v, err := rand.Int(r.rand, big.NewInt(int64(node.MaxDiscriminator)+1))
		if err != nil {
			return 0, fmt.Errorf("nodes: discriminator: %w", err)
		}
		if d := uint16(v.Int64()); !taken[d] { //nolint:gosec // below MaxDiscriminator + 1
			return d, nil
		}
	}
	return 0, errors.New("nodes: no free discriminator")
}

// maxPasscode is the largest setup passcode (Matter's 1..99999998,
// node.ValidateSetupPIN).
const maxPasscode = 99999998

// passcode is a random valid setup passcode.
func (r *Registry) passcode() (uint32, error) {
	for range 1000 {
		v, err := rand.Int(r.rand, big.NewInt(maxPasscode))
		if err != nil {
			return 0, fmt.Errorf("nodes: passcode: %w", err)
		}
		if p := uint32(v.Int64()) + 1; node.ValidateSetupPIN(p) == nil { //nolint:gosec // below maxPasscode
			return p, nil
		}
	}
	return 0, errors.New("nodes: no valid passcode")
}

// Add appends n and writes the file.
func (r *Registry) Add(n Node) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if slices.ContainsFunc(r.nodes, func(x Node) bool { return x.Key == n.Key }) {
		return fmt.Errorf("nodes: key %q is taken", n.Key)
	}
	next := append(slices.Clone(r.nodes), n)
	if err := r.write(next); err != nil {
		return err
	}
	r.nodes = next
	return nil
}

// Rename sets the label of key and writes the file.
func (r *Registry) Rename(key, label string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	i := slices.IndexFunc(r.nodes, func(x Node) bool { return x.Key == key })
	if i < 0 {
		return fmt.Errorf("%w: %s", ErrNotFound, key)
	}
	next := slices.Clone(r.nodes)
	next[i].Label = label
	if err := r.write(next); err != nil {
		return err
	}
	r.nodes = next
	return nil
}

// Remove takes key out and writes the file.
func (r *Registry) Remove(key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	i := slices.IndexFunc(r.nodes, func(x Node) bool { return x.Key == key })
	if i < 0 {
		return fmt.Errorf("%w: %s", ErrNotFound, key)
	}
	next := slices.Delete(slices.Clone(r.nodes), i, i+1)
	if err := r.write(next); err != nil {
		return err
	}
	r.nodes = next
	return nil
}

// RemoveFiles deletes the store of n — with SQLite's -journal, -wal and
// -shm beside it — its selection and any selection moved aside as
// corrupt. A file that is not there is no error.
func (r *Registry) RemoveFiles(n *Node) error {
	store := r.Path(n.StoreFile())
	paths := []string{store, store + "-journal", store + "-wal", store + "-shm", r.Path(n.SelectionFile())}
	aside, _ := filepath.Glob(r.Path(n.SelectionFile()) + ".corrupt-*")
	var errs []error
	for _, p := range append(paths, aside...) {
		if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// write replaces the file with nodes; r.mu is held.
func (r *Registry) write(nodes []Node) error {
	if nodes == nil {
		nodes = []Node{}
	}
	b, err := json.MarshalIndent(file{Nodes: nodes}, "", "  ")
	if err != nil {
		return fmt.Errorf("nodes: %w", err)
	}
	tmp, err := os.CreateTemp(r.dir, File+".tmp-*")
	if err != nil {
		return fmt.Errorf("nodes: %w", err)
	}
	name := tmp.Name()
	_, werr := tmp.Write(append(b, '\n'))
	if werr == nil {
		werr = tmp.Sync()
	}
	if cerr := tmp.Close(); werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Rename(name, r.Path(File))
	}
	if werr != nil {
		_ = os.Remove(name)
		return fmt.Errorf("nodes: write %s: %w", r.Path(File), werr)
	}
	return nil
}
