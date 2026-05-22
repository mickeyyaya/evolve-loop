#!/usr/bin/env bash
# AC-ID: cycle-104-004-auditor-plan-adherence-advisory
# AC-source: scout-report.md AC EG-104-04 (lines 225-228); intent.md acceptance_checks[3]
# Behavioral predicate (AUDITOR Plan Adherence section):
#   agents/evolve-auditor.md must contain the literal heading
#   "## Plan Adherence (advisory)" documenting an OPTIONAL,
#   NON-BLOCKING report section the Auditor fills in when
#   workspace/build-plan.md exists. Adherence must be marked as
#   advisory: it does NOT affect acs-verdict.json, red_count, or
#   the EGPS verdict. The non-blocking semantics must be explicit
#   so future readers understand it is informational only.
#
# Mutation spec (cycle-104-004-MUT):
#   Mutant: section heading absent                                     -> must FAIL.
#   Mutant: heading present but no non-blocking qualifier nearby       -> must FAIL.
#   Mutant: section marked as a FAIL trigger (contradicts ADR-0019)    -> must FAIL.
#
# Bash 3.2 compatible. No GNU-only flags.
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

AUDITOR="agents/evolve-auditor.md"

if [ ! -f "$AUDITOR" ]; then
  echo "RED: $AUDITOR does not exist" >&2
  exit 1
fi

# Required: literal heading.
HEADING='## Plan Adherence (advisory)'
if ! grep -qF "$HEADING" "$AUDITOR"; then
  echo "RED: $AUDITOR missing literal heading: $HEADING" >&2
  exit 1
fi

# Required: a non-blocking qualifier must appear somewhere in the file. Accept
# "non-blocking", "non blocking", or "informational" (case-insensitive).
if ! grep -qiE 'non[-[:space:]]?blocking|informational' "$AUDITOR"; then
  echo "RED: $AUDITOR has '$HEADING' but no non-blocking/informational qualifier" >&2
  echo "Expected one of: 'non-blocking', 'non blocking', 'informational' (case-insensitive)" >&2
  exit 1
fi

# Forbidden: Plan Adherence must NOT be wired as a FAIL trigger. Heuristic:
# scan for "Plan Adherence" mentioned together with red_count/acs-verdict/FAIL
# on the same line, and only fail if no negation token disambiguates intent.
if grep -qiE 'plan[[:space:]]+adherence.*(red_count|acs-verdict|FAIL|fail)' "$AUDITOR"; then
  offending=$(grep -niE 'plan[[:space:]]+adherence.*(red_count|acs-verdict|FAIL|fail)' "$AUDITOR")
  if ! printf '%s\n' "$offending" | grep -qiE 'not|never|no impact|does not|do not'; then
    echo "RED: $AUDITOR appears to wire Plan Adherence into FAIL/acs-verdict (cycle-105 enforce, not cycle-104 advisory):" >&2
    printf '%s\n' "$offending" >&2
    exit 1
  fi
fi

echo "GREEN: $AUDITOR contains '$HEADING' with non-blocking/informational qualifier"
exit 0
