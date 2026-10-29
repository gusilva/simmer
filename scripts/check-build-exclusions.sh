#!/usr/bin/env bash
#
# Fails when a Go file is excluded from the build by its filename rather than
# by an explicit //go:build line.
#
# Go treats a trailing _<GOOS> or _<GOARCH> in a filename as a build
# constraint, so search_android.go and sqlite_ios_test.go compile on no Mac
# and on no CI runner here. The file disappears from the build with no error
# anywhere: `go build`, `go vet` and `go test` all stay green while the code
# and its tests are simply absent. An interface the file was meant to satisfy
# then fails at run time, far from the cause.
#
# Deliberate exclusions carry a //go:build line (cmd/simmer/profiling_release.go),
# and are left alone.
set -euo pipefail

status=0

while read -r pkg files; do
	dir=$(go list -f '{{.Dir}}' "$pkg")
	for file in $(echo "$files" | tr -d '[]'); do
		if ! grep -q '^//go:build' "$dir/$file"; then
			echo "excluded from the build by its filename: $pkg/$file"
			echo "  rename it, or add an explicit //go:build line if the exclusion is intended"
			status=1
		fi
	done
done < <(go list -f '{{.ImportPath}} {{.IgnoredGoFiles}}' ./... | grep -v ' \[\]$')

exit $status
