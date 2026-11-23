// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package main

import (
	"net"
	"os"
	"strings"
	"testing"
)

// TestImageDefaults holds the plain image's defaults against the
// repository's Dockerfile (HR-P1-cfg-12, decision 27; HR-P1-sup-19): the
// web UI is not given a listener beyond loopback — the binary's default
// 127.0.0.1:8099 holds unless the operator sets WEB_LISTEN — the camera's
// TCP port is declared, and a .dockerignore keeps the notes, the held
// files and the repository's history out of the build context.
func TestImageDefaults(t *testing.T) {
	b, err := os.ReadFile("../../Dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	var expose []string
	for line := range strings.Lines(string(b)) {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		switch strings.ToUpper(f[0]) {
		case "ENV":
			for _, kv := range f[1:] {
				k, v, _ := strings.Cut(kv, "=")
				if k != "WEB_LISTEN" {
					continue
				}
				host, _, err := net.SplitHostPort(v)
				if ip := net.ParseIP(host); err != nil || ip == nil || !ip.IsLoopback() {
					t.Errorf("the image sets WEB_LISTEN=%s, which is not loopback", v)
				}
			}
		case "EXPOSE":
			expose = append(expose, f[1:]...)
		}
	}
	for _, p := range []string{"5540/udp", "5540/tcp"} {
		if !strings.Contains(strings.Join(expose, " "), p) {
			t.Errorf("EXPOSE %v lacks %s", expose, p)
		}
	}
	ig, err := os.ReadFile("../../.dockerignore")
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for line := range strings.Lines(string(ig)) {
		have[strings.TrimSpace(line)] = true
	}
	for _, p := range []string{".git", "notes", "held", "dist", ".env", "*.db"} {
		if !have[p] {
			t.Errorf(".dockerignore lacks %s", p)
		}
	}
}
