#!/usr/bin/env bash
# AC-ID: cycle-100-005-incident-doc-resolved
# AC-source: cycle-100/intent.md AC "docs/incidents/cycle-94-98-watchdog-overfiring.md, ... updated and internally consistent"
# acs-predicate: config-check
#
# Predicate: the watchdog overfire incident doc must be extended to
# cover cycle-99 AND marked RESOLVED by the cycle-100 migration.
# Concretely:
#   - The file MUST exist (either at the original name or renamed to
#     cycle-94-99-watchdog-overfiring.md).
#   - It MUST reference cycle 99 (timeline extension).
#   - It MUST contain a RESOLVED status marker AND a Resolution section
#     referencing cycle-100 / v10.18.0 / observer default flip.
#
# Doc-content predicate per acs/AGENTS.md waiver policy.
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

# Accept either original or renamed file.
DOC_ORIG="docs/incidents/cycle-94-98-watchdog-overfiring.md"
DOC_RENAMED="docs/incidents/cycle-94-99-watchdog-overfiring.md"

DOC=""
if [ -f "$DOC_RENAMED" ]; then
  DOC="$DOC_RENAMED"
elif [ -f "$DOC_ORIG" ]; then
  DOC="$DOC_ORIG"
else
  echo "RED: neither $DOC_ORIG nor $DOC_RENAMED exists on disk" >&2
  exit 1
fi
if ! git ls-files --error-unmatch "$DOC" >/dev/null 2>&1; then
  echo "RED: $DOC exists but is not git-tracked" >&2
  exit 1
fi

# (a) Must reference cycle 99 (timeline extension).
if ! grep -Eq 'cycle[ -]?99|cycle 99' "$DOC"; then
  echo "RED: $DOC does not reference cycle 99 (timeline must be extended)" >&2
  exit 1
fi

# (b) Must contain a RESOLVED status marker (case-insensitive).
if ! grep -qi 'RESOLVED' "$DOC"; then
  echo "RED: $DOC does not contain 'RESOLVED' status marker" >&2
  exit 1
fi

# (c) Must contain a Resolution section (## Resolution or similar).
if ! grep -Eqi '^##+ *Resolution' "$DOC"; then
  echo "RED: $DOC does not contain a '## Resolution' section heading" >&2
  exit 1
fi

# (d) Resolution must anchor to cycle-100 OR v10.18.0 OR the observer flip.
if ! grep -Eqi 'cycle[ -]?100|v10\.18\.0|EVOLVE_OBSERVER_ENFORCE=1|observer.*default|default.*observer' "$DOC"; then
  echo "RED: $DOC has RESOLVED marker but no anchor to cycle-100/v10.18.0/observer-flip" >&2
  exit 1
fi

echo "GREEN: $DOC extended to cycle 99 and marked RESOLVED with cycle-100/v10.18.0 anchor"
exit 0
