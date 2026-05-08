#!/usr/bin/env bash
#
# team-context.sh — Shared narrative bus for the evolve-loop swarm pipeline.
#
# The bus is a human-readable Markdown document at
# .evolve/runs/cycle-N/team-context.md that every pipeline agent appends a
# section to before exiting. The next agent reads the bus before starting.
# Replaces fragile JSON handoffs with a single canonical narrative.
#
# Subcommands:
#   init   <cycle> <workspace_dir>                    create stub bus
#   append <cycle> <workspace_dir> <role> <body_file> set role's section body
#   verify <cycle> <workspace_dir> --require <roles>  check required sections
#                                                     populated (comma-sep)
#
# Role → section mapping:
#   scout         → "Scout Findings"
#   tdd-engineer  → "TDD Contract"
#   builder       → "Build Report"
#   auditor       → "Audit Verdict"
#   (no role)     → "Goal" (set out-of-band during init)
#
# Idempotency: append replaces a role's section body each call. The header
# itself (the `## ...` line) is never duplicated. Init is also idempotent —
# repeated calls re-stub if no sections have been populated, otherwise leave
# existing content untouched (Section content beats stub).
#
# Format example:
#   # Cycle <N> Team Context
#
#   ## Goal
#   _pending_
#
#   ## Scout Findings
#   <scout body or _pending_>
#
#   ## TDD Contract
#   ...
#
# Bash 3.2 compatible (macOS default). Uses awk for section-aware editing.
# Atomic writes via tmp + mv.
#
# Exit:
#   0  — success
#   1  — usage / runtime error
#   2  — verify check failed (missing or empty required section)

set -uo pipefail

log()  { echo "[team-context] $*" >&2; }
fail() { log "FAIL: $*"; exit 1; }

# Sections in canonical order.
SECTIONS_ORDERED="Goal|Scout Findings|TDD Contract|Build Report|Audit Verdict"

# Map role name → section heading.
role_to_section() {
    case "$1" in
        scout)        echo "Scout Findings" ;;
        tdd-engineer) echo "TDD Contract" ;;
        builder)      echo "Build Report" ;;
        auditor)      echo "Audit Verdict" ;;
        goal)         echo "Goal" ;;
        *)            echo "" ;;
    esac
}

bus_path_for() {
    # arg1: workspace_dir
    echo "$1/team-context.md"
}

cmd_init() {
    local cycle="$1"
    local ws="$2"
    [ -d "$ws" ] || fail "workspace_dir not a directory: $ws"
    local bus
    bus=$(bus_path_for "$ws")
    if [ -f "$bus" ]; then
        log "init: bus already exists at $bus (no-op)"
        return 0
    fi
    local tmp="$bus.tmp.$$"
    {
        echo "# Cycle $cycle Team Context"
        echo
        echo "Each pipeline agent appends its section before exiting; the next agent reads the bus before starting."
        echo
        local IFS='|'
        for s in $SECTIONS_ORDERED; do
            echo "## $s"
            echo "_pending_"
            echo
        done
    } > "$tmp" && mv -f "$tmp" "$bus"
    log "init: $bus"
}

cmd_append() {
    local cycle="$1"
    local ws="$2"
    local role="$3"
    local body_file="$4"
    [ -f "$body_file" ] || fail "body_file not found: $body_file"
    local section
    section=$(role_to_section "$role")
    [ -n "$section" ] || fail "unknown role: $role (valid: scout|tdd-engineer|builder|auditor|goal)"
    local bus
    bus=$(bus_path_for "$ws")
    [ -f "$bus" ] || cmd_init "$cycle" "$ws"
    # Replace section body via awk: keep lines outside [section, next-section)
    # untouched; replace inside-region with new body content.
    local tmp="$bus.tmp.$$"
    awk -v section="$section" -v body_file="$body_file" '
        BEGIN {
            in_section = 0
            replaced = 0
        }
        /^## / {
            if (in_section) {
                # Closing previous section we were replacing — emit body.
                while ((getline line < body_file) > 0) print line
                close(body_file)
                print ""
                in_section = 0
                replaced = 1
            }
            if ($0 == "## " section) {
                print
                in_section = 1
                next
            }
        }
        in_section { next }      # skip old body
        { print }
        END {
            if (in_section && !replaced) {
                # Last section in file — emit body now.
                while ((getline line < body_file) > 0) print line
                close(body_file)
            }
        }
    ' "$bus" > "$tmp" && mv -f "$tmp" "$bus"
    log "append: role=$role section='$section' bus=$bus"
}

cmd_verify() {
    local cycle="$1"
    local ws="$2"
    shift 2
    local require_csv=""
    while [ $# -gt 0 ]; do
        case "$1" in
            --require) require_csv="$2"; shift 2 ;;
            *) fail "unknown verify flag: $1" ;;
        esac
    done
    [ -n "$require_csv" ] || fail "verify requires --require <csv-roles>"
    local bus
    bus=$(bus_path_for "$ws")
    [ -f "$bus" ] || { log "verify FAIL: bus not found ($bus)"; exit 2; }
    local missing=""
    local IFS=','
    for role in $require_csv; do
        unset IFS
        local section
        section=$(role_to_section "$role")
        if [ -z "$section" ]; then
            log "verify FAIL: unknown role '$role'"
            exit 2
        fi
        # Extract the section body (lines between `## $section` and next `## `
        # or EOF), then check it's not just `_pending_` / whitespace.
        local body
        body=$(awk -v s="$section" '
            $0 == "## " s { in_s = 1; next }
            /^## / && in_s { exit }
            in_s { print }
        ' "$bus" | tr -d '[:space:]')
        if [ -z "$body" ] || [ "$body" = "_pending_" ]; then
            missing="$missing $role"
        fi
        IFS=','
    done
    unset IFS
    if [ -n "$missing" ]; then
        log "verify FAIL: required sections empty:$missing"
        exit 2
    fi
    log "verify OK: all required sections populated"
}

usage() {
    cat <<USAGE >&2
team-context.sh — shared narrative bus for the evolve-loop swarm

Usage:
  team-context.sh init   <cycle> <workspace_dir>
  team-context.sh append <cycle> <workspace_dir> <role> <body_file>
  team-context.sh verify <cycle> <workspace_dir> --require <csv-roles>

Roles: scout | tdd-engineer | builder | auditor | goal
USAGE
    exit 1
}

[ $# -ge 1 ] || usage
case "$1" in
    init)   shift; [ $# -eq 2 ] || usage; cmd_init "$@" ;;
    append) shift; [ $# -eq 4 ] || usage; cmd_append "$@" ;;
    verify) shift; [ $# -ge 3 ] || usage; cmd_verify "$@" ;;
    *)      usage ;;
esac
