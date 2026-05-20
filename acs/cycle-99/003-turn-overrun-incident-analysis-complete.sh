#!/usr/bin/env bash
# AC-ID: cycle-99-003-turn-overrun-incident-analysis-complete
# Description: Cycle-99 resolves the abnormal-turn-overrun-c95 HIGH-priority carryover by producing a git-tracked 6-part incident analysis citing the carryover id, abnormal-events.jsonl, cycle-95, and either a root cause or an honest inconclusive verdict with recommended follow-up.
# Evidence: docs/operations/incidents/cycle-95-turn-overrun.md OR knowledge-base/research/{cycle-95-turn-overrun.md,turn-overrun-cycle-95.md}
# Author: tdd-engineer (cycle-99)
# Created: 2026-05-20T14:00:00Z
# Acceptance-of: cycle-99/scout-report.md §3 T5 ; cycle-99/triage-decision.md scope[T5] ; .evolve/state.json carryoverTodos[abnormal-turn-overrun-c95]
# AC-source: cycle-99/scout-report.md T5 ; triage-decision.md scope[T5] ;
#            state.json:carryoverTodos[abnormal-turn-overrun-c95]
# Behavioral predicate:
#   Cycle-99 must resolve the HIGH-priority carryover `abnormal-turn-overrun-c95`
#   by producing a git-tracked, 6-part incident analysis covering the cycle-95
#   turn-overrun event. The analysis must live in a persistent location
#   (NOT only `.evolve/runs/`, which is gitignored and ephemeral). Accepted
#   destinations:
#     - docs/operations/incidents/cycle-95-turn-overrun.md
#     - knowledge-base/research/cycle-95-turn-overrun.md
#     - knowledge-base/research/turn-overrun-cycle-95.md
#   The analysis must include the 6 sections required by repo policy
#   (feedback_detailed_incident_reports.md):
#     1. What happened           2. Research
#     3. Reasoning               4. Fix (or "inconclusive — recommended path")
#     5. Lessons                 6. References
#   AND must reference: the carryover id, abnormal-events.jsonl, and either
#   a root cause statement or an explicit "inconclusive" verdict with
#   recommended follow-up.
#
# RED until Builder writes the incident analysis to a persistent location;
# GREEN once the doc is present, git-tracked, and structurally complete.
#
# Bash 3.2 compatible. No GNU-only flags.
#
# Exit codes:
#   0 = GREEN (incident analysis present, tracked, 6 sections, required tokens)
#   1 = RED   (missing, untracked, or structurally incomplete)
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
if [ -z "$REPO_ROOT" ]; then
  echo "RED: not inside a git work tree" >&2
  exit 1
fi
cd "$REPO_ROOT" || { echo "RED: cd failed" >&2; exit 1; }

# (1) Locate the incident analysis at one of the accepted persistent paths.
CANDIDATES="
docs/operations/incidents/cycle-95-turn-overrun.md
knowledge-base/research/cycle-95-turn-overrun.md
knowledge-base/research/turn-overrun-cycle-95.md
"

REPORT=""
for path in $CANDIDATES; do
  if [ -f "$path" ] && git ls-files --error-unmatch "$path" >/dev/null 2>&1; then
    REPORT="$path"
    break
  fi
done

if [ -z "$REPORT" ]; then
  echo "RED: no git-tracked turn-overrun incident analysis found at any of:" >&2
  for path in $CANDIDATES; do
    echo "RED:   - $path" >&2
  done
  exit 1
fi

fail_count=0

# (2) 6-part structure check. We accept either numbered headings
#     ("## 1. What happened", "## 2. Research", …) OR plain section
#     headings with the part name as a substring. We tally distinct
#     part labels found.
parts_found=0
for label in 'happened' 'research' 'reasoning' 'fix' 'lessons' 'references'; do
  if grep -Eqi "^#{1,6}.*${label}" "$REPORT"; then
    parts_found=$(( parts_found + 1 ))
  fi
done
if [ "$parts_found" -lt 6 ]; then
  echo "RED: $REPORT missing required 6-part incident structure (found $parts_found/6 section labels)" >&2
  fail_count=$(( fail_count + 1 ))
fi

# (3) Carryover ID must be cited so audit can trace resolution.
if ! grep -q 'abnormal-turn-overrun-c95' "$REPORT"; then
  echo "RED: $REPORT does not cite carryover id 'abnormal-turn-overrun-c95'" >&2
  fail_count=$(( fail_count + 1 ))
fi

# (4) Evidence source reference.
if ! grep -q 'abnormal-events.jsonl' "$REPORT"; then
  echo "RED: $REPORT does not reference abnormal-events.jsonl (evidence source)" >&2
  fail_count=$(( fail_count + 1 ))
fi

# (5) Cycle 95 referenced explicitly (the incident's anchor cycle).
if ! grep -Eq '(cycle[- ]?95|c95)' "$REPORT"; then
  echo "RED: $REPORT does not reference cycle-95 (anchor cycle of incident)" >&2
  fail_count=$(( fail_count + 1 ))
fi

# (6) Either a root cause statement OR an explicit "inconclusive" verdict
#     with recommended follow-up. We accept either signal.
if grep -Eqi 'root[[:space:]]+cause' "$REPORT"; then
  : # root cause stated
elif grep -Eqi 'inconclusive' "$REPORT" && grep -Eqi 'recommend|follow[- ]?up|next[[:space:]]+step' "$REPORT"; then
  : # honest inconclusive with follow-up path
else
  echo "RED: $REPORT lacks both a root-cause statement AND an 'inconclusive + recommended follow-up' verdict" >&2
  fail_count=$(( fail_count + 1 ))
fi

# (7) Non-trivial size — guard against a stub doc that mechanically passes
#     headings but contains no substance. Require at least 30 non-blank
#     lines as a minimum-substance threshold for a "1-page" incident report.
nonblank_lines=$(grep -cE '[^[:space:]]' "$REPORT" 2>/dev/null || echo 0)
if [ "$nonblank_lines" -lt 30 ]; then
  echo "RED: $REPORT has only $nonblank_lines non-blank lines (<30) — likely stub, not substantive analysis" >&2
  fail_count=$(( fail_count + 1 ))
fi

if [ "$fail_count" -ne 0 ]; then
  echo "RED: turn-overrun incident analysis incomplete ($fail_count issue[s]) at $REPORT" >&2
  exit 1
fi

echo "GREEN: $REPORT is a git-tracked 6-part incident analysis citing carryover id, abnormal-events.jsonl, cycle-95, and a root-cause-or-inconclusive verdict"
exit 0
