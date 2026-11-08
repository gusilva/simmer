#!/usr/bin/env bash
#
# Fails when the unit tests write into the developer's home directory.
#
# A test that saves to the real ~/.config/simmer/config.toml or the View state
# under ~/.local/state passes every other check while overwriting the files
# of whoever runs it; a test killed mid-way loses them. Tests point
# XDG_CONFIG_HOME / XDG_STATE_HOME at t.TempDir() instead. This runs the suite
# with HOME and every XDG directory on an empty, read-only scratch directory:
# a write there fails the test that made it, even one that would have cleaned
# up after itself.
set -euo pipefail

scratch=$(mktemp -d)
chmod 500 "$scratch"
trap 'chmod 700 "$scratch"; rm -rf "$scratch"' EXIT

# The Go toolchain's own caches stay where they are: it needs them writable,
# and the run is no slower than a normal one.
export GOCACHE GOMODCACHE GOPATH
GOCACHE=$(go env GOCACHE)
GOMODCACHE=$(go env GOMODCACHE)
GOPATH=$(go env GOPATH)

if ! out=$(HOME="$scratch" \
	XDG_CONFIG_HOME="" XDG_STATE_HOME="" XDG_CACHE_HOME="" XDG_DATA_HOME="" \
	go test -count=1 ./... 2>&1); then
	echo "$out" | grep -E '^(--- FAIL|FAIL|ok|panic)|permission denied|read-only' || echo "$out"
	echo
	echo "tests failed with \$HOME read-only: a test is writing to the developer's"
	echo "home. Point XDG_CONFIG_HOME / XDG_STATE_HOME at t.TempDir() instead."
	exit 1
fi
