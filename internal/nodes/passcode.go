// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package nodes

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"strconv"
	"time"

	"thingfabric.dev/go-fabric/node"
)

// PasscodeFile is the configured node's passcode when PASSCODE is unset,
// beside the store as nodes.json is: {"passcode": 34567890,
// "created_at": "…"}, written 0600 and atomically like the registry.
const PasscodeFile = "passcode.json"

// passcodeFile is PasscodeFile's form.
type passcodeFile struct {
	Passcode  uint32    `json:"passcode"`
	CreatedAt time.Time `json:"created_at"`
}

// ConfiguredPasscode is the configured node's passcode when the operator
// set none: the one in PasscodeFile, or on the first start a random valid
// one, as a registry node gets ([Registry.Allocate]), written there
// before it is used. created reports a new one. A file that does not
// parse or holds an invalid passcode is renamed to
// passcode.json.corrupt-<unix seconds>, logged, and a new one drawn: a
// node with fabrics keeps them, and its own window, the only user of the
// passcode, opens only without one.
func (r *Registry) ConfiguredPasscode() (p uint32, created bool, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	path := r.Path(PasscodeFile)
	b, err := os.ReadFile(r.Path(PasscodeFile))
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return 0, false, fmt.Errorf("nodes: %w", err)
	default:
		var f passcodeFile
		perr := json.Unmarshal(b, &f)
		if perr == nil {
			if perr = node.ValidateSetupPIN(f.Passcode); perr == nil {
				return f.Passcode, false, nil
			}
		}
		moved := path + ".corrupt-" + strconv.FormatInt(r.now().Unix(), 10)
		if rerr := os.Rename(path, moved); rerr != nil {
			return 0, false, fmt.Errorf("nodes: %s is unusable (%w) and cannot be moved aside: %w", path, perr, rerr)
		}
		r.log.Error("nodes.passcode_corrupt", slog.String("moved_to", moved), slog.String("error", perr.Error()),
			slog.String("hint", "the configured node gets a new random passcode; a paired node keeps its fabrics, the new code is on the pairing page"))
	}
	if p, err = r.passcode(); err != nil {
		return 0, false, err
	}
	if err := r.writeJSON(PasscodeFile, passcodeFile{Passcode: p, CreatedAt: r.now().UTC().Truncate(time.Second)}); err != nil {
		return 0, false, err
	}
	return p, true, nil
}
