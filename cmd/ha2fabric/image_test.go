// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package main

import (
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
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
	for _, p := range imageListenProblems(string(b)) {
		t.Error(p)
	}
	var expose []string
	for _, in := range instructions(string(b)) {
		if f := strings.Fields(in); len(f) > 0 && strings.EqualFold(f[0], "EXPOSE") {
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

// instructions are a Dockerfile's instructions with their continuation
// lines joined and the comments left out.
func instructions(dockerfile string) []string {
	var out []string
	var cur strings.Builder
	for line := range strings.Lines(dockerfile) {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "#") || t == "" && cur.Len() == 0 {
			continue
		}
		if c, ok := strings.CutSuffix(t, "\\"); ok {
			cur.WriteString(c + " ")
			continue
		}
		cur.WriteString(t)
		out = append(out, cur.String())
		cur.Reset()
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// imageListenProblems are the Dockerfile's settings of the web UI's
// listener beyond loopback (HR-P3-test-14): WEB_LISTEN in an ENV, in
// either form and on a continuation line, and --web-listen in an
// ENTRYPOINT or CMD.
func imageListenProblems(dockerfile string) []string {
	var out []string
	check := func(v string) {
		v = strings.Trim(v, `"'`)
		host, _, err := net.SplitHostPort(v)
		if ip := net.ParseIP(host); err != nil || ip == nil || !ip.IsLoopback() {
			out = append(out, "the image sets WEB_LISTEN="+v+", which is not loopback")
		}
	}
	for _, in := range instructions(dockerfile) {
		f := strings.Fields(in)
		if len(f) == 0 {
			continue
		}
		switch strings.ToUpper(f[0]) {
		case "ENV":
			if len(f) == 3 && !strings.Contains(f[1], "=") {
				if f[1] == "WEB_LISTEN" {
					check(f[2])
				}
				continue
			}
			for _, kv := range f[1:] {
				if k, v, _ := strings.Cut(kv, "="); k == "WEB_LISTEN" {
					check(v)
				}
			}
		case "ENTRYPOINT", "CMD":
			if strings.Contains(in, "--web-listen") || strings.Contains(in, "-web-listen") {
				out = append(out, "the image's "+strings.ToUpper(f[0])+" passes --web-listen: "+in)
			}
		}
	}
	return out
}

// TestImageListenProblems holds the reading of the Dockerfile on the
// forms that escaped it.
func TestImageListenProblems(t *testing.T) {
	for src, want := range map[string]int{
		"ENV STORE_PATH=/data/x.db\nENTRYPOINT [\"/ha2fabric\"]\n":          0,
		"ENV WEB_LISTEN=127.0.0.1:8099\n":                                   0,
		"ENV WEB_LISTEN=0.0.0.0:8099\n":                                     1,
		"ENV A=1 \\\n    WEB_LISTEN=0.0.0.0:8099\n":                         1,
		"ENV WEB_LISTEN :8099\n":                                            1,
		"# ENV WEB_LISTEN=0.0.0.0:8099\n":                                   0,
		"ENTRYPOINT [\"/ha2fabric\", \"--web-listen\", \"0.0.0.0:8099\"]\n": 1,
		"CMD [\"-web-listen=:8099\"]\n":                                     1,
	} {
		if got := imageListenProblems(src); len(got) != want {
			t.Errorf("%q: %v, want %d problems", src, got, want)
		}
	}
}

// TestAddonStopTimeout (HR-P1-err-64, decision 29): the add-on's stop
// timeout leaves the daemon's stop budget room, so the Supervisor's
// SIGKILL does not land inside a node's stop (its default is 10 s).
func TestAddonStopTimeout(t *testing.T) {
	b, err := os.ReadFile("../../addon/config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for line := range strings.Lines(string(b)) {
		v, ok := strings.CutPrefix(strings.TrimSpace(line), "timeout:")
		if !ok {
			continue
		}
		secs, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil || time.Duration(secs)*time.Second < defaultStopBudget+5*time.Second {
			t.Fatalf("timeout: %s leaves the stop budget %v no room", strings.TrimSpace(v), defaultStopBudget)
		}
		return
	}
	t.Fatalf("addon/config.yaml sets no timeout; the Supervisor's default 10 s is below the stop budget %v", defaultStopBudget)
}
