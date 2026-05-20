#!/usr/bin/env bash
# AC-ID: cycle-100-001-observer-enforce-default-on
# AC-source: cycle-100/intent.md AC "run-cycle.sh default is :-1 (observer enforced); legacy :-0 form absent."
#
# Behavioral predicate: with EVOLVE_OBSERVER_ENFORCE UNSET (the default
# operator experience), the run-cycle.sh toggle MUST take the
# phase-observer branch, not the phase-watchdog branch.
#
# Strategy:
#   1. Source-level dual-check: confirm `EVOLVE_OBSERVER_ENFORCE:-1`
#      appears in scripts/dispatch/run-cycle.sh AND the legacy `:-0`
#      form does NOT appear.
#   2. Behavioral subprocess check: extract the toggle conditional and
#      execute it in a subshell with the env var unset. Stub
#      `phase-observer.sh` and `phase-watchdog.sh` to write to a marker
#      file. Assert observer-marker exists and watchdog-marker does not.
#
# Bash 3.2 compatible.
#
# Exit codes:
#   0 = GREEN
#   1 = RED
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
if [ -z "$REPO_ROOT" ]; then
  echo "RED: not inside a git work tree" >&2
  exit 1
fi
cd "$REPO_ROOT" || { echo "RED: cd failed" >&2; exit 1; }

TARGET="scripts/dispatch/run-cycle.sh"

# Dual-check existence.
if [ ! -f "$TARGET" ]; then
  echo "RED: $TARGET missing on disk" >&2
  exit 1
fi
if ! git ls-files --error-unmatch "$TARGET" >/dev/null 2>&1; then
  echo "RED: $TARGET exists but is not git-tracked" >&2
  exit 1
fi

# (1a) Confirm flipped default `:-1` is present.
if ! grep -Eq '"\$\{EVOLVE_OBSERVER_ENFORCE:-1\}"' "$TARGET"; then
  echo "RED: $TARGET does not contain '\${EVOLVE_OBSERVER_ENFORCE:-1}' (default must be 1)" >&2
  grep -n 'EVOLVE_OBSERVER_ENFORCE' "$TARGET" >&2 || true
  exit 1
fi

# (1b) Confirm legacy `:-0` form is absent.
if grep -Eq '"\$\{EVOLVE_OBSERVER_ENFORCE:-0\}"' "$TARGET"; then
  echo "RED: legacy '\${EVOLVE_OBSERVER_ENFORCE:-0}' form still present in $TARGET" >&2
  grep -n 'EVOLVE_OBSERVER_ENFORCE:-0' "$TARGET" >&2 || true
  exit 1
fi

# (2) Behavioral subprocess simulation — extract the live toggle block
# from run-cycle.sh and execute it with stubs. This is NOT a re-encoding
# of the contract; it runs the actual source text against a clean env so
# any future regression in the toggle (e.g. someone reverts `:-1`→`:-0`)
# RED-fails behaviorally even if the source-level grep is fooled.
TMP_DIR="$(mktemp -d -t acs-cycle-100-001.XXXXXX)" || {
  echo "RED: mktemp -d failed" >&2
  exit 1
}
trap 'rm -rf "$TMP_DIR"' EXIT

# Locate the toggle: from `if [ "${EVOLVE_OBSERVER_ENFORCE` for ~25 lines.
START_LINE=$(grep -n '"\${EVOLVE_OBSERVER_ENFORCE' "$TARGET" | head -n 1 | cut -d: -f1)
if [ -z "$START_LINE" ]; then
  echo "RED: could not locate EVOLVE_OBSERVER_ENFORCE conditional in $TARGET" >&2
  exit 1
fi
# Extract from START_LINE up to the FIRST line that is exactly `        fi`
# (eight-space-indented fi) — matching the inner if-else-fi block. Anchoring
# on the leading whitespace avoids tripping on `; fi` or differently-nested
# closures.
awk -v start="$START_LINE" '
  NR == start { emit = 1 }
  emit { print }
  emit && /^        fi[[:space:]]*$/ { exit }
' "$TARGET" >"$TMP_DIR/toggle-block.sh"
if ! grep -q '^        fi[[:space:]]*$' "$TMP_DIR/toggle-block.sh"; then
  echo "RED: could not locate closing 'fi' for EVOLVE_OBSERVER_ENFORCE block" >&2
  exit 1
fi

# Harness: stub `bash` so we can intercept phase-observer.sh /
# phase-watchdog.sh invocations without actually spawning them.
cat >"$TMP_DIR/harness.sh" <<'EOF'
#!/usr/bin/env bash
set -u
log() { :; }
EVOLVE_PLUGIN_ROOT="${EVOLVE_PLUGIN_ROOT:-/nonexistent}"
WORKSPACE="${WORKSPACE:-/tmp}"
RUN_PGID="${RUN_PGID:-0}"
CYCLE="${CYCLE:-100}"
CYCLE_STATE_PATH_FOR_WD="${CYCLE_STATE_PATH_FOR_WD:-/dev/null}"
WATCHDOG_PID=""
bash() {
  for a in "$@"; do
    case "$a" in
      *phase-observer.sh) touch "$MARKER_DIR/observer-spawned" ;;
      *phase-watchdog.sh) touch "$MARKER_DIR/watchdog-spawned" ;;
    esac
  done
  return 0
}
. "$TOGGLE_BLOCK"
wait 2>/dev/null || true
EOF
chmod +x "$TMP_DIR/harness.sh"

# Run with EVOLVE_OBSERVER_ENFORCE UNSET (default operator experience).
unset EVOLVE_OBSERVER_ENFORCE
MARKER_DIR="$TMP_DIR" \
TOGGLE_BLOCK="$TMP_DIR/toggle-block.sh" \
  bash "$TMP_DIR/harness.sh" >"$TMP_DIR/harness.out" 2>"$TMP_DIR/harness.err" || true

if [ ! -f "$TMP_DIR/observer-spawned" ]; then
  echo "RED: observer was NOT spawned under default env (toggle did not honor :-1)" >&2
  echo "--- harness stderr ---" >&2
  cat "$TMP_DIR/harness.err" >&2 || true
  exit 1
fi
if [ -f "$TMP_DIR/watchdog-spawned" ]; then
  echo "RED: watchdog was spawned under default env (toggle still defaults to 0)" >&2
  exit 1
fi

echo "GREEN: $TARGET defaults to phase-observer (literal :-1 present, extracted toggle block executed and selected observer branch)"
exit 0
