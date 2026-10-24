// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

// Command ha2fabric is the placeholder of the daemon until its first
// release: it states the project's status and exits with status 2.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "go-ha2fabric: concept, not started — see CONCEPT.md")
	os.Exit(2)
}
