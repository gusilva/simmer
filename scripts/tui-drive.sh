#!/usr/bin/env bash
#
# Drives the Simmer TUI in a detached tmux session, for checking behaviour
# unit tests cannot reach: real key decoding, rendering, the editor's timers.
#
#   tui-drive.sh start [--resume] [-- simmer flags…]
#   tui-drive.sh keys KEY…      one tmux key per argument: Escape Enter Tab BSpace
#                               C-l C-Space Up …, or a single character
#   tui-drive.sh type TEXT      TEXT typed literally
#   tui-drive.sh paste TEXT     TEXT as one bracketed paste
#   tui-drive.sh screen [A B]   the screen as plain text, optionally lines A–B
#   tui-drive.sh log [N]        last N lines (default 40) of the TRACE log
#   tui-drive.sh stop
#
# The run is isolated: config and View state live in a scratch directory, so
# nothing typed reaches the user's own files. --resume copies the user's
# config.toml and state.json in first, so the last view — an open database —
# comes back. Simmer runs with --log --log-level TRACE; `log` reads it.
#
# Keys go one per tmux call with a pause between them, so an Esc is never
# merged with the next key into alt+key, and a text ending in ";" is sent
# so tmux does not take the ";" as its own command separator.
set -euo pipefail

session=simmer-drive
scratch=${TMPDIR:-/tmp}/simmer-drive
pause=${TUI_DRIVE_PAUSE:-0.25}
repo=$(cd "$(dirname "$0")/.." && pwd)

die() { echo "tui-drive: $*" >&2; exit 1; }
running() { tmux has-session -t "$session" 2>/dev/null; }
need_running() { running || die "not running; use: $0 start"; }

cmd=${1:-}; shift || true
case "$cmd" in
start)
	command -v tmux >/dev/null || die "tmux is not installed"
	resume=false
	while [ $# -gt 0 ]; do
		case "$1" in
		--resume) resume=true; shift ;;
		--) shift; break ;;
		*) die "unknown start flag: $1" ;;
		esac
	done
	running && tmux kill-session -t "$session"
	rm -rf "$scratch"
	mkdir -p "$scratch/config/simmer" "$scratch/state/simmer"
	if $resume; then
		cp "${XDG_CONFIG_HOME:-$HOME/.config}/simmer/config.toml" "$scratch/config/simmer/" 2>/dev/null || true
		cp "${XDG_STATE_HOME:-$HOME/.local/state}/simmer/state.json" "$scratch/state/simmer/" 2>/dev/null || true
	fi
	(cd "$repo" && go build -o "$scratch/simmer" ./cmd/simmer)

	args=(--log --log-level TRACE "$@")
	tmux new-session -d -s "$session" -x "${TUI_DRIVE_COLS:-150}" -y "${TUI_DRIVE_ROWS:-44}" -c "$scratch" \
		env XDG_CONFIG_HOME="$scratch/config" XDG_STATE_HOME="$scratch/state" "$scratch/simmer" "${args[@]}"

	for _ in $(seq 50); do
		if tmux capture-pane -t "$session" -p | grep -q '[^[:space:]]'; then
			sleep 1 # let the first device scan and any resume land
			exit 0
		fi
		sleep 0.2
	done
	die "simmer drew nothing within 10s"
	;;

keys)
	need_running
	for k in "$@"; do
		tmux send-keys -t "$session" "$k"
		sleep "$pause"
	done
	;;

type)
	need_running
	text=${1:-}
	if [ "${text%;}" != "$text" ]; then
		[ -n "${text%;}" ] && tmux send-keys -t "$session" -l -- "${text%;}"
		tmux send-keys -t "$session" '\;'
	else
		tmux send-keys -t "$session" -l -- "$text"
	fi
	sleep "$pause"
	;;

paste)
	need_running
	printf '%s' "${1:-}" | tmux load-buffer -b simmer-drive -
	tmux paste-buffer -p -d -b simmer-drive -t "$session"
	sleep "$pause"
	;;

screen)
	need_running
	if [ $# -ge 2 ]; then
		tmux capture-pane -t "$session" -p | sed -n "$1,$2p"
	else
		tmux capture-pane -t "$session" -p
	fi
	;;

log)
	tail -n "${1:-40}" "$scratch/simmer.log" 2>/dev/null || die "no log yet at $scratch/simmer.log"
	;;

stop)
	running && tmux kill-session -t "$session"
	;;

*)
	sed -n '3,22p' "$0" | sed 's/^# \{0,1\}//'
	exit 2
	;;
esac
