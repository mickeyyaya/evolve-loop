#!/usr/bin/env bash
# AC-ID: cycle-97-003-role-context-builder-env-var-wins
# AC-source: cycle-97/intent.md acceptance_checks[2] (fixture b) ; scout-report.md T1 Fixture B
# Behavioral predicate: explicit EVOLVE_CONTEXT_DIGEST=0 in the env MUST
#   override the profile's context_mode=digest. With env=0 the orchestrator
#   role assembly MUST NOT emit "## Cycle Digest" (full legacy mode wins).
#
# This is the precedence half of the L1 contract: the env var is the
# "operator escape hatch" and must always defeat the profile default,
# matching the goal text's "env-var precedence preserved".
#
# RED until the loader correctly preserves env-var precedence.
# GREEN once env=0 produces full-mode output despite profile=digest.
#
# Bash 3.2 compatible. No GNU-only flags.
#
# Exit codes:
#   0 = GREEN (env override honored)
#   1 = RED   (profile silently overrode env)
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
# Precondition: profile.context_mode=="digest". If the profile lacks
# context_mode entirely, this predicate is moot (env-precedence over a
# missing default is vacuous), so we fail RED with a clear message.
if ! jq -e '.context_mode == "digest"' "$PROFILE" >/dev/null 2>&1; then
  echo "RED: precondition — $PROFILE lacks .context_mode=\"digest\"" >&2
  exit 1
fi

WORK="$(mktemp -d -t cycle97-003.XXXXXX)" || { echo "RED: mktemp failed" >&2; exit 1; }
trap 'rm -rf "$WORK"' EXIT
cat > "$WORK/intent.md" <<'INTENT'
<!-- ANCHOR:acceptance_criteria -->
## Acceptance Criteria
- predicate-003 fixture intent
INTENT

# Fixture B: env explicitly set to 0 → env MUST win → digest OFF.
OUTPUT="$(EVOLVE_CONTEXT_DIGEST=0 bash "$BUILDER_SCRIPT" orchestrator 97 "$WORK" 2>/dev/null)"
rc=$?
if [ "$rc" -ne 0 ]; then
  echo "RED: role-context-builder.sh exited rc=$rc on Fixture B" >&2
  exit 1
fi

# Digest heading MUST be absent when env wins.
if [[ "$OUTPUT" =~ \#\#[[:space:]]+Cycle[[:space:]]+Digest ]]; then
  echo "RED: Fixture B — EVOLVE_CONTEXT_DIGEST=0 did NOT override profile; '## Cycle Digest' still emitted" >&2
  echo "first 40 lines of output:" >&2
  printf '%s\n' "$OUTPUT" | head -40 >&2
  exit 1
fi

echo "GREEN: EVOLVE_CONTEXT_DIGEST=0 overrides profile context_mode=digest (env precedence honored)"
exit 0
