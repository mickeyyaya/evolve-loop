#!/usr/bin/env bash
#
# probe-tool.sh — Reusable CLI-availability probe (v8.13.3).
#
# /insights identified a recurring failure pattern: Claude declaring a tool
# unavailable based on a single check, when the tool was actually installed
# in a non-default location (~/.local/bin, /opt/homebrew/bin, /usr/local/bin).
# This helper does the canonical probe and prints what was checked, so the
# operator can see the diagnostic trail.
#
# Usage:
#   bash scripts/probe-tool.sh <tool>          # exit 0 if found, 1 if not
#   bash scripts/probe-tool.sh <tool> --quiet  # same, no stdout (for scripting)
#   bash scripts/probe-tool.sh <tool> --json   # emit JSON: {"tool":"...","found":true,"path":"..."}
#
# Probe order (first match wins):
#   1. command -v $tool          (PATH lookup, the canonical bash check)
#   2. type -P $tool             (alternate PATH probe; catches some shell aliases)
#   3. /usr/local/bin/$tool      (Homebrew Intel mac default)
#   4. /opt/homebrew/bin/$tool   (Homebrew Apple-silicon default)
#   5. $HOME/.local/bin/$tool    (pipx, cargo, manual installs)
#   6. $HOME/bin/$tool           (legacy install location)
#   7. /usr/bin/$tool            (system install)
#
# Exit codes:
#   0 — tool found (path printed to stdout unless --quiet)
#   1 — tool not found anywhere we checked
#  10 — bad arguments

set -uo pipefail

TOOL=""
QUIET=0
JSON=0

while [ $# -gt 0 ]; do
    case "$1" in
        --quiet) QUIET=1 ;;
        --json)  JSON=1 ;;
        --help|-h) sed -n '2,28p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
        --*) echo "[probe-tool] unknown flag: $1" >&2; exit 10 ;;
        *)
            if [ -z "$TOOL" ]; then TOOL="$1"
            else echo "[probe-tool] extra positional arg: $1" >&2; exit 10
            fi ;;
    esac
    shift
done

[ -n "$TOOL" ] || { echo "[probe-tool] usage: probe-tool.sh <tool> [--quiet] [--json]" >&2; exit 10; }

# Probe order — first match wins. (Probing is inlined in the main loop below;
# audit cycle 8204 LOW-1 noted a previously-defined probe_path() helper as
# dead code, removed.)
CHECKED=()

# 1. command -v / type -P (PATH lookup).
if cv=$(command -v "$TOOL" 2>/dev/null) && [ -n "$cv" ]; then
    CHECKED+=("command -v $TOOL → $cv")
    found="$cv"
elif tp=$(type -P "$TOOL" 2>/dev/null) && [ -n "$tp" ]; then
    CHECKED+=("type -P $TOOL → $tp")
    found="$tp"
else
    CHECKED+=("command -v $TOOL → not found")
    CHECKED+=("type -P $TOOL → not found")
    # 2-7: explicit common locations.
    found=""
    for p in \
        "/usr/local/bin/$TOOL" \
        "/opt/homebrew/bin/$TOOL" \
        "$HOME/.local/bin/$TOOL" \
        "$HOME/bin/$TOOL" \
        "/usr/bin/$TOOL"; do
        if [ -x "$p" ]; then
            CHECKED+=("$p → executable")
            found="$p"
            break
        fi
        CHECKED+=("$p → not present")
    done
fi

# Emit result.
if [ "$JSON" = "1" ]; then
    if [ -n "$found" ]; then
        printf '{"tool":"%s","found":true,"path":"%s"}\n' "$TOOL" "$found"
    else
        printf '{"tool":"%s","found":false,"path":null,"checked":%d}\n' "$TOOL" "${#CHECKED[@]}"
    fi
elif [ "$QUIET" != "1" ]; then
    if [ -n "$found" ]; then
        echo "$found"
        echo "[probe-tool] OK: $TOOL found at $found" >&2
    else
        echo "[probe-tool] NOT FOUND: $TOOL" >&2
        echo "[probe-tool] checked locations:" >&2
        for c in "${CHECKED[@]}"; do
            echo "[probe-tool]   $c" >&2
        done
    fi
fi

[ -n "$found" ] && exit 0 || exit 1
