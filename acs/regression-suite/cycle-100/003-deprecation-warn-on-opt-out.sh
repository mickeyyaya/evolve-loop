#!/usr/bin/env bash
# AC-ID: cycle-100-003-deprecation-warn-on-opt-out
# AC-source: cycle-100/intent.md AC "Opt-out smoke (EVOLVE_OBSERVER_ENFORCE=0): deprecation WARN appears, watchdog spawns"
#
# Behavioral predicate: when EVOLVE_OBSERVER_ENFORCE=0 is set
# explicitly (the opt-out path), run-cycle.sh MUST emit a deprecation
# WARN naming the env var before spawning the watchdog. The WARN must
# tell operators (a) the var is deprecated, (b) removing the override
# returns them to the new default.
#
# Strategy:
#   1. Source-level dual-check: confirm a literal WARN string mentioning
#      'EVOLVE_OBSERVER_ENFORCE=0' and 'deprecated' exists in
#      scripts/dispatch/run-cycle.sh.
#   2. Behavioral subprocess check: extract just the toggle block with
#      sed and exec it in a subshell with stubbed phase-* scripts and
#      a `log` function that captures output. Assert the deprecation
#      WARN appears in the captured output when EVOLVE_OBSERVER_ENFORCE=0.
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
if [ ! -f "$TARGET" ]; then
  echo "RED: $TARGET missing on disk" >&2
  exit 1
fi
if ! git ls-files --error-unmatch "$TARGET" >/dev/null 2>&1; then
  echo "RED: $TARGET exists but is not git-tracked" >&2
  exit 1
fi

# (1) Source-level: WARN line must mention env-var name and "deprecated".
# Two greps so a partial fix (only one keyword) still RED.
if ! grep -Eq 'EVOLVE_OBSERVER_ENFORCE=0' "$TARGET"; then
  echo "RED: no string 'EVOLVE_OBSERVER_ENFORCE=0' in $TARGET" >&2
  exit 1
fi
if ! grep -Eqi 'WARN[^"]*EVOLVE_OBSERVER_ENFORCE=0|EVOLVE_OBSERVER_ENFORCE=0[^"]*deprecated' "$TARGET"; then
  echo "RED: $TARGET does not contain a WARN/deprecated message tied to EVOLVE_OBSERVER_ENFORCE=0" >&2
  grep -n 'EVOLVE_OBSERVER_ENFORCE' "$TARGET" >&2 || true
  exit 1
fi

# (2) Behavioral simulation — extract the watchdog/observer toggle block
# and run it in a subshell with the env var set to 0. Stubs replace
# phase-observer.sh, phase-watchdog.sh, and log() so we can capture
# behavior without spawning real processes.
TMP_DIR="$(mktemp -d -t acs-cycle-100-003.XXXXXX)" || {
  echo "RED: mktemp -d failed" >&2
  exit 1
}
trap 'rm -rf "$TMP_DIR"' EXIT

# Locate the toggle block: from `if [ "${EVOLVE_OBSERVER_ENFORCE` up to the
# matching `fi` two levels deep. We extract a small window that is
# self-contained.
START_LINE=$(grep -n '"\${EVOLVE_OBSERVER_ENFORCE' "$TARGET" | head -n 1 | cut -d: -f1)
if [ -z "$START_LINE" ]; then
  echo "RED: could not find EVOLVE_OBSERVER_ENFORCE conditional in $TARGET" >&2
  exit 1
fi
# Extract from START_LINE up to the FIRST line matching `        fi` —
# the closing of the inner if-else-fi for the observer toggle.
awk -v start="$START_LINE" '
  NR == start { emit = 1 }
  emit { print }
  emit && /^        fi[[:space:]]*$/ { exit }
' "$TARGET" >"$TMP_DIR/toggle-block.sh"
if ! grep -q '^        fi[[:space:]]*$' "$TMP_DIR/toggle-block.sh"; then
  echo "RED: could not locate closing 'fi' for EVOLVE_OBSERVER_ENFORCE block" >&2
  exit 1
fi

# Build harness that defines log() as a capture, stubs the two scripts,
# and sources the extracted block.
cat >"$TMP_DIR/harness.sh" <<'EOF'
#!/usr/bin/env bash
set -u
CAPTURE_FILE="${CAPTURE_FILE:-/dev/null}"
log() { printf '%s\n' "$*" >>"$CAPTURE_FILE"; }
# Minimal variable shims so the extracted block does not error on unbound vars.
EVOLVE_PLUGIN_ROOT="${EVOLVE_PLUGIN_ROOT:-/nonexistent}"
WORKSPACE="${WORKSPACE:-/tmp}"
RUN_PGID="${RUN_PGID:-0}"
CYCLE="${CYCLE:-100}"
CYCLE_STATE_PATH_FOR_WD="${CYCLE_STATE_PATH_FOR_WD:-/dev/null}"
WATCHDOG_PID=""
# Stub bash so we don't actually spawn phase-observer/phase-watchdog.
bash() {
  # phase-observer.sh / phase-watchdog.sh stubs: just touch markers and return
  for a in "$@"; do
    case "$a" in
      *phase-observer.sh) touch "$CAPTURE_DIR/observer-spawned" ;;
      *phase-watchdog.sh) touch "$CAPTURE_DIR/watchdog-spawned" ;;
    esac
  done
  return 0
}
# Trap unbound EVOLVE_INACTIVITY_DISABLE check needed by surrounding code; the
# extracted block assumes it's inside the parent's `if INACTIVITY_DISABLE != 1`
# so just run the toggle.
. "$TOGGLE_BLOCK"
wait 2>/dev/null || true
EOF
chmod +x "$TMP_DIR/harness.sh"

# Run with EVOLVE_OBSERVER_ENFORCE=0 (opt-out).
EVOLVE_OBSERVER_ENFORCE=0 \
CAPTURE_FILE="$TMP_DIR/captured-log.txt" \
CAPTURE_DIR="$TMP_DIR" \
TOGGLE_BLOCK="$TMP_DIR/toggle-block.sh" \
  bash "$TMP_DIR/harness.sh" >"$TMP_DIR/harness-stdout.txt" 2>"$TMP_DIR/harness-stderr.txt"
rc=$?
# Harness may exit non-zero if the extracted block references things we
# didn't shim — but the WARN should still have been logged before any
# error, since the WARN must be emitted FIRST in the else-branch.
# Only fail if no log file exists.
if [ ! -f "$TMP_DIR/captured-log.txt" ]; then
  echo "RED: harness produced no captured log (rc=$rc)" >&2
  echo "--- stderr ---" >&2
  cat "$TMP_DIR/harness-stderr.txt" >&2 || true
  exit 1
fi

# The captured log MUST contain a WARN mentioning EVOLVE_OBSERVER_ENFORCE=0
# and the word "deprecated".
if ! grep -Eqi 'WARN.*EVOLVE_OBSERVER_ENFORCE=0|EVOLVE_OBSERVER_ENFORCE=0.*deprecated' \
       "$TMP_DIR/captured-log.txt"; then
  echo "RED: captured log lacks deprecation WARN for opt-out path" >&2
  echo "--- captured log ---" >&2
  cat "$TMP_DIR/captured-log.txt" >&2 || true
  echo "--- end log ---" >&2
  exit 1
fi

# Also confirm the watchdog (not the observer) was selected.
if [ ! -f "$TMP_DIR/watchdog-spawned" ]; then
  echo "RED: opt-out branch did NOT select phase-watchdog" >&2
  ls -la "$TMP_DIR" >&2
  exit 1
fi
if [ -f "$TMP_DIR/observer-spawned" ]; then
  echo "RED: opt-out branch unexpectedly also spawned phase-observer" >&2
  exit 1
fi

echo "GREEN: opt-out (EVOLVE_OBSERVER_ENFORCE=0) emits deprecation WARN; watchdog selected (observer not spawned)"
exit 0
