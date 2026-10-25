#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Copyright (C) 2026 SukramJ.
#
# Prints the changelog.md section of a version (Keep a Changelog form,
# "## [X.Y.Z] - date") as the release notes, with a compare link to the
# previous version. Shared by `make release` and release-on-tag.yml.
# Fails when the section is missing, so a release never has an empty body.
#
# Usage: script/extract-release-notes.sh <version>
set -euo pipefail
if [ $# -lt 1 ]; then
	echo "usage: $0 <version>" >&2
	exit 2
fi
VERSION="$1"
CHANGELOG="${CHANGELOG:-changelog.md}"
body=$(awk -v ver="$VERSION" '
	/^## \[/ {
		if (insec) exit
		if (index($0, "## [" ver "]") == 1) { insec=1; next }
	}
	insec { print }
' "$CHANGELOG")
if [ -z "$body" ]; then
	echo "error: no '## [$VERSION]' section in $CHANGELOG" >&2
	exit 1
fi
prev=$(awk -v ver="$VERSION" '
	index($0, "## [" ver "]") == 1 { insec=1; next }
	insec && /^## \[[0-9]/ { sub(/^## \[/, ""); sub(/\].*$/, ""); print; exit }
' "$CHANGELOG")
repo="${GITHUB_REPOSITORY:-SukramJ/go-ha2fabric}"
printf '%s\n' "$body"
if [ -n "$prev" ]; then
	printf '\n**Full Changelog**: https://github.com/%s/compare/v%s...v%s\n' "$repo" "$prev" "$VERSION"
fi
