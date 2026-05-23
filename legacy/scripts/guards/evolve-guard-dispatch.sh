#!/usr/bin/env bash
#
# evolve-guard-dispatch.sh — v11.4.0 hook shim that routes PreToolUse
# guard invocations to the native Go binary (`evolve guard <name>`)
# with graceful fallback to the legacy bash scripts.
#
# Invocation contract (used by .claude/settings.json):
#
#   bash $CLAUDE_PROJECT_DIR/legacy/scripts/guards/evolve-guard-dispatch.sh <guard-script-name>
#
# where <guard-script-name> matches the bash script basename minus .sh:
#
#   ship-gate
#   phase-gate-precondition
#   role-gate
#   research-quota-gate         (under legacy/scripts/hooks/)
#   doc-deletion-guard          (under legacy/scripts/hooks/)
#
# Routing precedence:
#
#   1. EVOLVE_NATIVE_GUARDS=0           → bash legacy script (force rollback)
#   2. Resolved binary + probe passes   → exec Go (`evolve guard <name>`)
#   3. Binary missing / probe fails     → fall back to bash legacy script
#
# The Go binary is resolved in this order (PATH lookup REMOVED in the
# #16 hardened-shim — a stale system install of `evolve` would silently
# match and brick every hook on exit 2, exactly the v11.4.0 dev-session
# failure mode):
#
#   $EVOLVE_GO_BIN
#   $CLAUDE_PROJECT_DIR/go/bin/evolve
#
# Before exec, the shim runs `"$evolve_bin" guard 2>&1` (no stdin) and
# greps the output for "evolve guard:". Current builds emit that prefix
# in their usage message (see go/cmd/evolve/cmd_guard.go:runGuard) and
# exit non-zero on missing-arg. Stale builds (Phase 1 task #13-14
# predecessors) list guard in --help but lack the switch case, so they
# print "unknown command 'guard'" + exit 2 — caught here, fallback fires.
#
# stdin/stdout/stderr are forwarded unchanged. Exit code from the
# delegate is passed through (0=allow, 2=deny, others=error).
#
# Audit trail: when the Go path is taken, cmd_guard.go appends to
# .evolve/guards.log mirroring the bash format. When falling back to
# bash, the bash scripts append themselves. Either way, the log line
# format is byte-identical.

set -uo pipefail

guard_short_name="${1:-}"
if [ -z "$guard_short_name" ]; then
    echo "[evolve-guard-dispatch] missing guard name argument" >&2
    exit 10
fi

# Map hook-script basename → Go guard subcommand name.
case "$guard_short_name" in
    ship-gate)                  go_guard="ship";       bash_path="legacy/scripts/guards/ship-gate.sh" ;;
    phase-gate-precondition)    go_guard="phase";      bash_path="legacy/scripts/guards/phase-gate-precondition.sh" ;;
    role-gate)                  go_guard="role";       bash_path="legacy/scripts/guards/role-gate.sh" ;;
    research-quota-gate)        go_guard="quota";      bash_path="legacy/scripts/hooks/research-quota-gate.sh" ;;
    doc-deletion-guard)         go_guard="docdelete";  bash_path="legacy/scripts/hooks/doc-deletion-guard.sh" ;;
    *)
        echo "[evolve-guard-dispatch] unknown guard '$guard_short_name'" >&2
        exit 10
        ;;
esac

repo_root="${CLAUDE_PROJECT_DIR:-}"
if [ -z "$repo_root" ]; then
    # Fall back to walking up from this script's location to find
    # .claude-plugin/. Mirrors resolve-roots.sh-style discovery.
    self_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    candidate="$self_dir"
    while [ "$candidate" != "/" ]; do
        if [ -d "$candidate/.claude-plugin" ]; then
            repo_root="$candidate"
            break
        fi
        candidate="$(dirname "$candidate")"
    done
    if [ -z "$repo_root" ]; then
        # Last resort: use cwd. Hooks usually run from the project root.
        repo_root="$(pwd)"
    fi
fi

bash_full="$repo_root/$bash_path"

fallback_to_bash() {
    local reason="${1:-fallback}"
    if [ -x "$bash_full" ] || [ -r "$bash_full" ]; then
        # Forward stdin to the bash legacy script. The hook protocol
        # is identical so this is a transparent passthrough.
        exec bash "$bash_full"
    fi
    echo "[evolve-guard-dispatch] $reason; legacy bash script not found at $bash_full" >&2
    # Allow on missing legacy script — better than blocking the world.
    # The intentional design: hooks fail-open on missing infrastructure.
    exit 0
}

# Rollback hatch: force bash path.
if [ "${EVOLVE_NATIVE_GUARDS:-1}" = "0" ]; then
    fallback_to_bash "EVOLVE_NATIVE_GUARDS=0"
fi

# Resolve Go binary — explicit paths only. PATH lookup REMOVED in #16
# (see header) because a stale system `evolve` install would silently
# match here and brick every hook on its exit 2.
evolve_bin=""
if [ -n "${EVOLVE_GO_BIN:-}" ] && [ -x "$EVOLVE_GO_BIN" ]; then
    evolve_bin="$EVOLVE_GO_BIN"
elif [ -x "$repo_root/go/bin/evolve" ]; then
    evolve_bin="$repo_root/go/bin/evolve"
fi

if [ -z "$evolve_bin" ]; then
    fallback_to_bash "Go binary not found (EVOLVE_GO_BIN unset; no $repo_root/go/bin/evolve)"
fi

# #16 probe: confirm the binary actually understands `guard`. The
# current build emits "evolve guard:" in its missing-arg usage line;
# a stale Phase-1 build prints "unknown command 'guard'" (exit 2) and
# would brick every Bash tool call once wired into PreToolUse.
probe_output="$("$evolve_bin" guard 2>&1 || true)"
if [[ "$probe_output" != *"evolve guard:"* ]]; then
    fallback_to_bash "binary at $evolve_bin doesn't recognize 'guard' subcommand (likely stale build; rebuild via 'cd go && go build -o bin/evolve ./cmd/evolve')"
fi

# Soft warning if binary is older than main.go. The probe above is the
# hard gate — this is just an early-warning that a rebuild would refresh
# subcommand behavior (e.g., recently added guards).
src_main="$repo_root/go/cmd/evolve/main.go"
if [ -f "$src_main" ] && [ -n "$(find "$src_main" -newer "$evolve_bin" 2>/dev/null)" ]; then
    echo "[evolve-guard-dispatch] WARN: $evolve_bin is older than $src_main; rebuild may be needed (cd go && go build -o bin/evolve ./cmd/evolve)" >&2
fi

# Exec the Go binary. stdin/stdout/stderr forward unchanged.
exec "$evolve_bin" guard "$go_guard" --evolve-dir "$repo_root/.evolve"
