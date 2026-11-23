// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package nodes_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"thingfabric.dev/go-fabric/node"

	"github.com/SukramJ/go-ha2fabric/internal/nodes"
)

// TestConfiguredPasscode: the configured node's passcode without PASSCODE
// is drawn once, valid, kept beside the store (0600) and the same at the
// next start; a file that does not parse or holds an invalid passcode is
// moved aside and a new one drawn (HR-P0-cfg-21).
func TestConfiguredPasscode(t *testing.T) {
	dir := t.TempDir()
	p, created, err := open(t, dir).ConfiguredPasscode()
	if err != nil || !created || node.ValidateSetupPIN(p) != nil {
		t.Fatalf("first start: %d %v %v", p, created, err)
	}
	st, err := os.Stat(filepath.Join(dir, nodes.PasscodeFile))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && st.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", st.Mode().Perm())
	}
	again, created, err := open(t, dir).ConfiguredPasscode()
	if err != nil || created || again != p {
		t.Fatalf("second start: %d (first %d) %v %v", again, p, created, err)
	}
	for _, bad := range []string{`{`, `{"passcode":11111111}`, `{"passcode":0}`} {
		if err := os.WriteFile(filepath.Join(dir, nodes.PasscodeFile), []byte(bad), 0o600); err != nil {
			t.Fatal(err)
		}
		q, created, err := open(t, dir).ConfiguredPasscode()
		if err != nil || !created || node.ValidateSetupPIN(q) != nil {
			t.Fatalf("%s: %d %v %v", bad, q, created, err)
		}
		if aside, _ := filepath.Glob(filepath.Join(dir, nodes.PasscodeFile+".corrupt-*")); len(aside) == 0 {
			t.Fatalf("%s: not moved aside", bad)
		}
	}
}

// TestConfiguredPasscodeNotKept: a passcode file that cannot be read,
// moved aside or written still yields a valid passcode for this start,
// with ErrPasscodeNotKept (HR-P0-cfg-54).
func TestConfiguredPasscodeNotKept(t *testing.T) {
	dir := t.TempDir()
	// A directory in the file's place: it cannot be read as a file, and a
	// rename onto it fails.
	if err := os.Mkdir(filepath.Join(dir, nodes.PasscodeFile), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, nodes.PasscodeFile, "x"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	p, created, err := open(t, dir).ConfiguredPasscode()
	if !errors.Is(err, nodes.ErrPasscodeNotKept) || !created || node.ValidateSetupPIN(p) != nil {
		t.Fatalf("%d %v %v", p, created, err)
	}
}
