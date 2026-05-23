#!/usr/bin/env bash
#
# evolve-guard-dispatch-test.sh — Unit tests for the #16 hardened-shim
# in legacy/scripts/guards/evolve-guard-dispatch.sh.
#
# Covers the four routing branches:
#   1. Working binary → exec Go (allow path)
#   2. Stale binary (no guard support) → fall back to bash
#   3. Missing binary → fall back to bash
#   4. EVOLVE_NATIVE_GUARDS=0 → force bash even if Go binary works
#
# Plus the structural #16 guarantees:
#   5. PATH lookup is NOT consulted (a stale system install must not
#      silently take over)
#   6. Older-binary WARN fires when mtime predates main.go
#
# Usage: bash legacy/scripts/tests/evolve-guard-dispatch-test.sh

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
DISPATCH="$REPO_ROOT/legacy/scripts/guards/evolve-guard-dispatch.sh"

PASS=0; FAIL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; }

cleanup_dirs=()
trap '
    for d in ${cleanup_dirs[@]+"${cleanup_dirs[@]}"}; do rm -rf "$d"; done
' EXIT

# make_fake_repo creates a minimal repo-shaped tmpdir with the layout
# the dispatcher expects: .claude-plugin/ marker, go/cmd/evolve/main.go
# source file, optional go/bin/evolve binary, and the requested bash
# fallback at legacy/scripts/guards/ship-gate.sh.
#
# Args: $1=binary_kind (working|stale|missing), $2=bash_kind (allow|deny|missing)
# Echoes the tmpdir path.
make_fake_repo() {
    local bin_kind="$1"
    local bash_kind="$2"
    local tmp
    tmp="$(mktemp -d 2>/dev/null || mktemp -d -t evolveghtest)"
    cleanup_dirs+=("$tmp")

    mkdir -p "$tmp/.claude-plugin" \
             "$tmp/go/bin" \
             "$tmp/go/cmd/evolve" \
             "$tmp/legacy/scripts/guards" \
             "$tmp/legacy/scripts/hooks"
    echo '{}' >"$tmp/.claude-plugin/plugin.json"
    # main.go source — used by mtime check. Created first so binary can
    # be made newer or older deterministically per test.
    echo "package main" >"$tmp/go/cmd/evolve/main.go"

    case "$bin_kind" in
        working)
            cat >"$tmp/go/bin/evolve" <<'EOF'
#!/usr/bin/env bash
# Fake working evolve binary: emits the canary "evolve guard:" line
# on missing-arg probe; for actual guard invocations, echo Allow JSON
# and exit 0 so dispatcher's exec succeeds.
if [ "${1:-}" = "guard" ] && [ -z "${2:-}" ]; then
    echo "evolve guard: usage: evolve guard <name> [--evolve-dir DIR] < stdin.json" >&2
    exit 10
fi
if [ "${1:-}" = "guard" ]; then
    echo '{"guard":"'$2'","allow":true,"reason":""}'
    exit 0
fi
exit 1
EOF
            chmod +x "$tmp/go/bin/evolve"
            ;;
        stale)
            cat >"$tmp/go/bin/evolve" <<'EOF'
#!/usr/bin/env bash
# Fake stale evolve binary: pre-#13-14 Phase 1 build. Lists 'guard' in
# --help (not exercised) but unknown in dispatch switch — exits 2.
echo "evolve: unknown command \"$1\"" >&2
exit 2
EOF
            chmod +x "$tmp/go/bin/evolve"
            ;;
        missing)
            : # no binary written
            ;;
    esac

    case "$bash_kind" in
        allow)
            cat >"$tmp/legacy/scripts/guards/ship-gate.sh" <<'EOF'
#!/usr/bin/env bash
echo "BASH_FALLBACK_RAN"
exit 0
EOF
            chmod +x "$tmp/legacy/scripts/guards/ship-gate.sh"
            ;;
        deny)
            cat >"$tmp/legacy/scripts/guards/ship-gate.sh" <<'EOF'
#!/usr/bin/env bash
echo "BASH_FALLBACK_DENY" >&2
exit 2
EOF
            chmod +x "$tmp/legacy/scripts/guards/ship-gate.sh"
            ;;
        missing)
            : # no fallback script
            ;;
    esac

    echo "$tmp"
}

# run_dispatch invokes the shim with CLAUDE_PROJECT_DIR set to the fake
# repo and captures both streams + rc. Output is echoed back in three
# bash assignments suitable for `eval`.
run_dispatch() {
    local repo="$1"
    local guard="${2:-ship-gate}"
    local extra_env="${3:-}"
    local stdout_file stderr_file rc
    stdout_file="$(mktemp)"
    stderr_file="$(mktemp)"
    cleanup_dirs+=("$stdout_file" "$stderr_file")
    # shellcheck disable=SC2086
    env CLAUDE_PROJECT_DIR="$repo" $extra_env \
        bash "$DISPATCH" "$guard" \
        >"$stdout_file" 2>"$stderr_file" </dev/null
    rc=$?
    printf 'stdout=%q\nstderr=%q\nrc=%d\n' \
        "$(cat "$stdout_file")" "$(cat "$stderr_file")" "$rc"
}

header "1. working binary → exec Go (allow path)"
repo=$(make_fake_repo working allow)
eval "$(run_dispatch "$repo" ship-gate)"
if [ "$rc" -ne 0 ]; then
    fail_ "expected rc=0 from Go path, got $rc; stderr=$stderr"
elif [[ "$stdout" != *"\"allow\":true"* ]]; then
    fail_ "Go binary not invoked (stdout missing allow JSON): $stdout"
elif [[ "$stdout" == *"BASH_FALLBACK_RAN"* ]]; then
    fail_ "bash fallback ran instead of Go: $stdout"
else
    pass "Go path taken, allow JSON returned"
fi

header "2. stale binary (no guard support) → fall back to bash"
repo=$(make_fake_repo stale allow)
eval "$(run_dispatch "$repo" ship-gate)"
if [ "$rc" -ne 0 ]; then
    fail_ "expected rc=0 (allow via bash fallback), got $rc"
elif [[ "$stdout" != *"BASH_FALLBACK_RAN"* ]]; then
    fail_ "bash fallback did not execute: stdout=$stdout stderr=$stderr"
else
    pass "stale binary → bash fallback fired"
fi

header "3. missing binary → fall back to bash"
repo=$(make_fake_repo missing allow)
eval "$(run_dispatch "$repo" ship-gate)"
if [ "$rc" -ne 0 ]; then
    fail_ "expected rc=0 (allow via bash fallback), got $rc"
elif [[ "$stdout" != *"BASH_FALLBACK_RAN"* ]]; then
    fail_ "bash fallback did not execute: stdout=$stdout stderr=$stderr"
else
    pass "missing binary → bash fallback fired"
fi

header "4. EVOLVE_NATIVE_GUARDS=0 → force bash even with working binary"
repo=$(make_fake_repo working allow)
eval "$(run_dispatch "$repo" ship-gate EVOLVE_NATIVE_GUARDS=0)"
if [ "$rc" -ne 0 ]; then
    fail_ "expected rc=0, got $rc"
elif [[ "$stdout" != *"BASH_FALLBACK_RAN"* ]]; then
    fail_ "EVOLVE_NATIVE_GUARDS=0 did not force bash path: $stdout"
elif [[ "$stdout" == *"\"allow\":true"* ]]; then
    fail_ "Go binary ran despite EVOLVE_NATIVE_GUARDS=0: $stdout"
else
    pass "rollback hatch works"
fi

header "5. PATH lookup is NOT consulted (#16 invariant)"
# Put a working binary ONLY on PATH (not at go/bin/evolve, not in
# EVOLVE_GO_BIN). The shim must NOT find it — must fall back to bash.
repo=$(make_fake_repo missing allow)
path_dir="$(mktemp -d)"
cleanup_dirs+=("$path_dir")
cat >"$path_dir/evolve" <<'EOF'
#!/usr/bin/env bash
# This binary works perfectly — but it lives on PATH, so #16 must ignore it.
if [ "${1:-}" = "guard" ] && [ -z "${2:-}" ]; then
    echo "evolve guard: usage" >&2
    exit 10
fi
echo '{"allow":true,"hijacked":true}'
exit 0
EOF
chmod +x "$path_dir/evolve"
eval "$(run_dispatch "$repo" ship-gate "PATH=$path_dir:$PATH")"
if [ "$rc" -ne 0 ]; then
    fail_ "expected rc=0 (bash fallback), got $rc"
elif [[ "$stdout" == *"hijacked"* ]]; then
    fail_ "#16 violated: PATH-installed evolve binary was used"
elif [[ "$stdout" != *"BASH_FALLBACK_RAN"* ]]; then
    fail_ "bash fallback didn't fire: $stdout"
else
    pass "PATH binary correctly ignored"
fi

header "6. WARN emitted when binary mtime < main.go mtime"
repo=$(make_fake_repo working allow)
# Backdate the binary to 1 hour ago, leave main.go at now.
touch -t "$(date -v-1H '+%Y%m%d%H%M' 2>/dev/null || date -d '1 hour ago' '+%Y%m%d%H%M')" \
      "$repo/go/bin/evolve"
eval "$(run_dispatch "$repo" ship-gate)"
if [ "$rc" -ne 0 ]; then
    fail_ "expected rc=0, got $rc"
elif [[ "$stderr" != *"older than"* ]]; then
    fail_ "WARN line missing from stderr: $stderr"
else
    pass "mtime WARN fires for stale binary"
fi

header "7. EVOLVE_GO_BIN explicit override is honored"
repo=$(make_fake_repo missing allow)
override_dir="$(mktemp -d)"
cleanup_dirs+=("$override_dir")
cat >"$override_dir/evolve-custom" <<'EOF'
#!/usr/bin/env bash
if [ "${1:-}" = "guard" ] && [ -z "${2:-}" ]; then
    echo "evolve guard: usage" >&2; exit 10
fi
echo '{"allow":true,"via":"custom"}'
exit 0
EOF
chmod +x "$override_dir/evolve-custom"
eval "$(run_dispatch "$repo" ship-gate "EVOLVE_GO_BIN=$override_dir/evolve-custom")"
if [ "$rc" -ne 0 ]; then
    fail_ "expected rc=0, got $rc; stderr=$stderr"
elif [[ "$stdout" != *"\"via\":\"custom\""* ]]; then
    fail_ "EVOLVE_GO_BIN override not honored: $stdout"
else
    pass "EVOLVE_GO_BIN routes to custom binary"
fi

echo
echo "==================================================="
echo "evolve-guard-dispatch-test.sh: $PASS PASS, $FAIL FAIL"
echo "==================================================="
if [ "$FAIL" -gt 0 ]; then exit 1; fi
exit 0
