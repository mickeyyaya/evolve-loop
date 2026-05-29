#!/usr/bin/env bash
# AC-ID: cycle-90-001-expected-ship-sha-refreshed
# Description: Verifies that .evolve/state.json:expected_ship_sha matches the
#   SHA-256 of the currently-deployed scripts/lifecycle/ship.sh. Plan §3A
#   demands a refresh against the canonical file so future ship attempts do
#   not emit `ship-refused: tampering` from a stale pin.
# Evidence: intent.md success-criteria row "sha256sum scripts/lifecycle/ship.sh
#   == jq -r .expected_ship_sha .evolve/state.json"; triage-decision.md item 3A.
# Author: tdd-engineer (cycle-90)
# Created: 2026-05-19
# Acceptance-of: build-report.md row "3A: state.json:expected_ship_sha
#   recomputed against current ship.sh and atomically written"
#
# Behavioral: recomputes the SHA-256 from disk every run (does NOT trust the
# value Builder reports) and compares to the value Builder must have written
# to state.json. A mutant that simply pins the existing (stale) value when
# ship.sh has drifted will fail. A mutant that pins a hard-coded literal
# unrelated to the actual file will also fail.
set -uo pipefail

# state.json lives only in the canonical project root (.evolve/ is gitignored
# and never copied into per-cycle worktrees). Prefer EVOLVE_PROJECT_ROOT;
# fall back to git toplevel only if env-var is absent.
REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
SHIP_SCRIPT="$REPO_ROOT/scripts/lifecycle/ship.sh"
STATE_FILE="$REPO_ROOT/.evolve/state.json"
AC_ID="cycle-90-001-expected-ship-sha-refreshed"

if [ ! -f "$SHIP_SCRIPT" ]; then
  echo "RED $AC_ID: ship.sh not found at $SHIP_SCRIPT" >&2
  exit 1
fi

if [ ! -f "$STATE_FILE" ]; then
  echo "RED $AC_ID: state.json not found at $STATE_FILE" >&2
  exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "RED $AC_ID: jq not on PATH" >&2
  exit 1
fi

# Pick whichever sha256 helper exists on the host (Linux: sha256sum, macOS: shasum -a 256).
compute_sha() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    return 2
  fi
}

actual_sha=$(compute_sha "$SHIP_SCRIPT")
rc=$?
if [ $rc -ne 0 ] || [ -z "$actual_sha" ]; then
  echo "RED $AC_ID: unable to compute sha256 of ship.sh (rc=$rc)" >&2
  exit 1
fi

pinned_sha=$(jq -r '.expected_ship_sha // ""' "$STATE_FILE" 2>/dev/null)
if [ -z "$pinned_sha" ]; then
  echo "RED $AC_ID: state.json:expected_ship_sha missing or empty" >&2
  exit 1
fi

# Reject obvious placeholders / partial hashes that pass eyeball-similarity but
# would never satisfy ship-gate's exact-match policy.
if [ "${#pinned_sha}" -ne 64 ]; then
  echo "RED $AC_ID: pinned sha length ${#pinned_sha} != 64 (got '$pinned_sha')" >&2
  exit 1
fi

if [ "$actual_sha" != "$pinned_sha" ]; then
  echo "RED $AC_ID: ship.sh sha mismatch" >&2
  echo "  computed: $actual_sha" >&2
  echo "  pinned:   $pinned_sha" >&2
  exit 1
fi

echo "GREEN $AC_ID: expected_ship_sha == sha256(scripts/lifecycle/ship.sh) == $actual_sha"
exit 0
