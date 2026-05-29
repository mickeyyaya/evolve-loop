#!/usr/bin/env bash
# AC-ID: cycle-97-002-role-context-builder-honors-profile-context-mode
# AC-source: cycle-97/intent.md acceptance_checks[2] ; scout-report.md T1 Fixture A
# Behavioral predicate: when role-context-builder.sh runs for the
#   orchestrator role with the profile's context_mode=digest AND
#   EVOLVE_CONTEXT_DIGEST is NOT explicitly set, the loader MUST set
#   EVOLVE_CONTEXT_DIGEST=1 so that emit_digest_summary fires and the
#   output contains the "## Cycle Digest" marker.
#
# This is Fixture A from scout-report: profile says digest, env unset
#   → digest mode active.
#
# RED until Builder adds _load_profile_context_mode() (or equivalent
# loader-side bridge) and orchestrator.json carries context_mode=digest.
# GREEN once the bridge is wired up AND the orchestrator role assembly
# actually fires emit_digest_summary on this path.
#
# Bash 3.2 compatible. No GNU-only flags. Avoids `set -e` because we
# need to inspect the script's exit code as well as its stdout.
#
# Exit codes:
#   0 = GREEN (predicate satisfied)
#   1 = RED   (predicate violated)
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
if [ -z "$REPO_ROOT" ]; then
  echo "RED: not inside a git work tree" >&2
  exit 1
fi
cd "$REPO_ROOT" || { echo "RED: cd failed" >&2; exit 1; }

BUILDER_SCRIPT="scripts/lifecycle/role-context-builder.sh"
PROFILE=".evolve/profiles/orchestrator.json"
if [ ! -x "$BUILDER_SCRIPT" ]; then
  echo "RED: $BUILDER_SCRIPT missing or not executable" >&2
  exit 1
fi
if [ ! -f "$PROFILE" ]; then
  echo "RED: $PROFILE missing" >&2
  exit 1
fi
if ! command -v jq >/dev/null 2>&1; then
  echo "RED: jq required" >&2
  exit 1
fi

# Precondition: profile.context_mode=="digest" (predicate 001 covers
# this assertion at the AC level; we re-check here so this predicate
# does not falsely pass if 001 fails for an unrelated reason).
if ! jq -e '.context_mode == "digest"' "$PROFILE" >/dev/null 2>&1; then
  echo "RED: precondition — $PROFILE lacks .context_mode=\"digest\"" >&2
  exit 1
fi

# Mint a clean workspace with a minimal intent.md so emit_intent_compact
# has *something* to operate on. The orchestrator-role assembly does not
# require scout/build/audit artifacts to be present.
WORK="$(mktemp -d -t cycle97-002.XXXXXX)" || { echo "RED: mktemp failed" >&2; exit 1; }
trap 'rm -rf "$WORK"' EXIT
cat > "$WORK/intent.md" <<'INTENT'
<!-- ANCHOR:acceptance_criteria -->
## Acceptance Criteria
- predicate-002 fixture intent
INTENT

# Fixture A: env UNSET, profile context_mode=digest → digest must fire.
# We explicitly `unset` the variable so any parent leak doesn't pollute.
OUTPUT="$(unset EVOLVE_CONTEXT_DIGEST; \
          bash "$BUILDER_SCRIPT" orchestrator 97 "$WORK" 2>/dev/null)"
rc=$?
if [ "$rc" -ne 0 ]; then
  echo "RED: role-context-builder.sh exited rc=$rc on Fixture A" >&2
  exit 1
fi

# The digest emitter prints the literal heading "## Cycle Digest".
# Match the heading prefix portably with bash regex (avoid pipe+grep
# under set -o pipefail; see feedback_test_pipe_pipefail_sigpipe).
if ! [[ "$OUTPUT" =~ \#\#[[:space:]]+Cycle[[:space:]]+Digest ]]; then
  echo "RED: Fixture A — profile context_mode=digest + env unset did NOT emit '## Cycle Digest'" >&2
  echo "first 40 lines of output:" >&2
  printf '%s\n' "$OUTPUT" | head -40 >&2
  exit 1
fi

echo "GREEN: profile context_mode=digest + env unset → '## Cycle Digest' emitted"
exit 0
