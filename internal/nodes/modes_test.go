// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package nodes_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"thingfabric.dev/go-ha2fabric/internal/nodes"
)

func mode(t *testing.T, path string) os.FileMode {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return fi.Mode().Perm()
}

// TestSecretFileModesTightened (HR-P1-cfg-10): nodes.json and
// passcode.json hold passcodes; one that is there with a wider mode — a
// restored backup, a copy — is set to 0600 when read, and so is a file
// moved aside as corrupt.
func TestSecretFileModesTightened(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no POSIX modes on Windows")
	}
	dir := t.TempDir()
	r := open(t, dir)
	n, err := r.Allocate("Alexa", nodes.Configured{Listen: "127.0.0.1:5540", Discriminator: 3840})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Add(n); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.ConfiguredPasscode(); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{nodes.File, nodes.PasscodeFile} {
		if err := os.Chmod(filepath.Join(dir, f), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r = open(t, dir)
	if len(r.Nodes()) != 1 {
		t.Fatal("the registry lost its node")
	}
	if _, created, err := r.ConfiguredPasscode(); err != nil || created {
		t.Fatalf("passcode: created %v, %v", created, err)
	}
	for _, f := range []string{nodes.File, nodes.PasscodeFile} {
		if m := mode(t, filepath.Join(dir, f)); m != 0o600 {
			t.Errorf("%s is %v after a read", f, m)
		}
	}

	// A file moved aside keeps the passcodes it holds: 0600 as well.
	if err := os.WriteFile(filepath.Join(dir, nodes.File), []byte(`{"nodes": [{"key": "x", "passcode": 1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	open(t, dir)
	aside, _ := filepath.Glob(filepath.Join(dir, nodes.File+".corrupt-*"))
	if len(aside) != 1 {
		t.Fatalf("aside %v", aside)
	}
	if m := mode(t, aside[0]); m != 0o600 {
		t.Errorf("the file moved aside is %v", m)
	}
}
