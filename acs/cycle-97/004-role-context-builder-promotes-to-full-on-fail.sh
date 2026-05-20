#!/usr/bin/env bash
# AC-ID: cycle-97-004-role-context-builder-promotes-to-full-on-fail
# AC-source: cycle-97/intent.md challenged_premises[1] proposed_alternative ; scout-report.md T1 Fixture C
# Behavioral predicate (FAIL-path safeguard / regression gate):
#   When the profile says context_mode=digest, env is UNSET, but
#   state.json shows a recent failedApproaches entry with classification
#   "code-audit-fail" or "code-build-fail", role-context-builder.sh MUST
#   promote back to full mode (digest OFF). The orchestrator persona
#   needs full audit-report defect bodies + commit SHA verbatim on
#   FAIL/WARN-NO-AUDIT cycles for correct remediation routing.
#
# Why this matters: a silent under-feed on FAIL cycles would cause the
# orchestrator to compose lossy commit messages, mis-route to ship vs
# retrospective, and miss BLOCK-RECURRING-AUDIT-FAIL pattern detection.
# Scout flagged this as the MEDIUM-risk regression to guard against and
# scout-report calls Fixture C "the regression gate".
#
# RED until Builder implements the FAIL-path promotion branch.
# GREEN once the loader checks state.failedApproaches and clears the
# digest flag when the most-recent classification is code-{audit,build}-fail.
#
# Test isolation: this predicate redirects EVOLVE_PROJECT_ROOT to a
# scratch dir so it can inject a faked state.json without touching the
# real repo state. EVOLVE_PLUGIN_ROOT stays pointing at the live repo
# so profile-loading still resolves to orchestrator.json.
#
# Bash 3.2 compatible. No GNU-only flags.
#
# Exit codes:
#   0 = GREEN (promotion to full honored on FAIL cycle)
#   1 = RED   (loader stayed in digest mode despite FAIL signal)
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
if ! jq -e '.context_mode == "digest"' "$PROFILE" >/dev/null 2>&1; then
  echo "RED: precondition — $PROFILE lacks .context_mode=\"digest\"" >&2
  exit 1
fi

# Scratch project root with .evolve/state.json containing a recent
# code-audit-fail signal. The plugin root (where the profile lives) stays
# on the real repo so the loader still finds orchestrator.json.
SCRATCH="$(mktemp -d -t cycle97-004.XXXXXX)" || { echo "RED: mktemp failed" >&2; exit 1; }
trap 'rm -rf "$SCRATCH"' EXIT
mkdir -p "$SCRATCH/.evolve" "$SCRATCH/workspace"

# Inject a minimal state.json. We include two failedApproaches entries
# so any "look at last/most-recent" implementation has a deterministic
# answer regardless of array-index direction. Both entries are FAIL-class
# so either choice MUST promote.
cat > "$SCRATCH/.evolve/state.json" <<'STATE'
{
  "lastCycleNumber": 96,
  "failedApproaches": [
    {
      "classification": "code-audit-fail",
      "summary": "predicate-004 fixture: recent audit fail",
      "verdict": "FAIL"
    },
    {
      "classification": "code-audit-fail",
      "summary": "predicate-004 fixture: most-recent audit fail",
      "verdict": "FAIL"
    }
  ]
}
STATE

cat > "$SCRATCH/workspace/intent.md" <<'INTENT'
<!-- ANCHOR:acceptance_criteria -->
## Acceptance Criteria
- predicate-004 fixture intent
INTENT

# Fixture C: env unset, profile=digest, state.json shows FAIL → must promote.
OUTPUT="$(unset EVOLVE_CONTEXT_DIGEST; \
          EVOLVE_PROJECT_ROOT="$SCRATCH" \
          bash "$BUILDER_SCRIPT" orchestrator 97 "$SCRATCH/workspace" 2>/dev/null)"
rc=$?
if [ "$rc" -ne 0 ]; then
  echo "RED: role-context-builder.sh exited rc=$rc on Fixture C" >&2
  exit 1
fi

# When promoted to full, the digest heading MUST be absent.
if [[ "$OUTPUT" =~ \#\#[[:space:]]+Cycle[[:space:]]+Digest ]]; then
  echo "RED: Fixture C — recent code-audit-fail did NOT promote to full; '## Cycle Digest' still emitted" >&2
  echo "first 60 lines of output:" >&2
  printf '%s\n' "$OUTPUT" | head -60 >&2
  exit 1
fi

echo "GREEN: recent code-audit-fail in state.json promoted orchestrator context back to full (digest OFF)"
exit 0
