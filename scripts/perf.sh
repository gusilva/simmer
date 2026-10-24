#!/usr/bin/env bash
# Benchmark runner that names results after the git revision measured.
#
#   scripts/perf.sh run            -> perf/<git-describe>.bench.txt
#   scripts/perf.sh compare A [B]  -> benchstat perf/A.bench.txt perf/B.bench.txt
#                                     (B defaults to the current revision's file)
#   scripts/perf.sh list           -> list saved results
#
# Env: COUNT (default 10), ALLOW_DIRTY=1 to run on a dirty tree.
# Only compare results taken on the same machine.
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p perf

rev() { git describe --tags --always --dirty 2>/dev/null || echo unknown; }
file() { echo "perf/$1.bench.txt"; }

cmd="${1:-run}"
case "$cmd" in
run)
  r="$(rev)"
  if [[ "$r" == *-dirty && -z "${ALLOW_DIRTY:-}" ]]; then
    echo "working tree is dirty ($r); commit first or set ALLOW_DIRTY=1" >&2
    exit 1
  fi
  out="$(file "$r")"
  {
    echo "commit: $(git rev-parse HEAD)"
    echo "describe: $r"
    echo "date: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "go: $(go version)"
    echo "machine: $(uname -sm)"
    go test -run='^$' -bench=. -benchmem -count="${COUNT:-10}" \
      ./internal/app ./internal/device
  } | tee "$out"
  echo "saved $out"
  ;;
compare)
  a="${2:?usage: compare <revA> [revB]}"
  b="${3:-$(rev)}"
  command -v benchstat >/dev/null || { echo "install: go install golang.org/x/perf/cmd/benchstat@latest" >&2; exit 1; }
  benchstat "$(file "$a")" "$(file "$b")"
  ;;
list)
  for f in perf/*.bench.txt; do
    [ -e "$f" ] && echo "$(basename "$f" .bench.txt)  $(grep -m1 '^date:' "$f")"
  done
  ;;
*)
  echo "usage: $0 run | compare A [B] | list" >&2; exit 1 ;;
esac
