// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

// Package version is the build metadata of go-ha2fabric, set at link time
// (-ldflags -X); the defaults keep go run and the tests working.
package version

import (
	"runtime/debug"
	"strconv"
	"strings"
)

// Build-time information. Version tracks the latest release tag so a
// build without -ldflags still reports a sensible version; bump it with
// the release's changelog section (the release workflow refuses a tag that
// differs from it, and addon/config.yaml's version equals it).
var (
	// Version is the release (e.g. "0.1.0").
	Version = "0.3.6"
	// Commit is the short Git SHA the binary was built from.
	Commit = "none"
	// BuildDate is the UTC RFC 3339 build time.
	BuildDate = "unknown"
)

// GoFabric is the version of the go-fabric module the binary is built
// with.
func GoFabric() string { return dep("github.com/SukramJ/go-fabric") }

// GoHamapping is the version of the go-hamapping module the binary is
// built with.
func GoHamapping() string { return dep("github.com/SukramJ/go-hamapping") }

func dep(path string) string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, d := range info.Deps {
		if d.Path == path {
			if d.Replace != nil {
				return d.Replace.Version
			}
			return d.Version
		}
	}
	return "unknown"
}

// Software is Version as BasicInformation's SoftwareVersion: "X.Y.Z" as
// X·10⁶ + Y·10³ + Z, monotonic across releases; anything else (a
// development build) is 0. The encoding is the one of go-fabric's cookbook
// daemon (go-zendure2mqtt internal/fabric softwareVersion).
func Software() uint32 {
	parts := strings.Split(strings.TrimPrefix(Version, "v"), ".")
	if len(parts) != 3 {
		return 0
	}
	var out uint32
	for _, p := range parts {
		n, err := strconv.ParseUint(p, 10, 32)
		if err != nil || n > 999 {
			return 0
		}
		out = out*1000 + uint32(n)
	}
	return out
}

// String is a one-line build banner.
func String() string {
	return "go-ha2fabric " + Version + " (go-fabric " + GoFabric() + ", go-hamapping " + GoHamapping() +
		", commit " + Commit + ", built " + BuildDate + ")"
}
