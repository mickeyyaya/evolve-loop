#!/usr/bin/env bash
# AC-ID: cycle-97-005-triage-extraction-no-duplication
# AC-source: cycle-97/intent.md acceptance_checks[0] ; scout-report.md T2 ; triage-decision.md T2
# Behavioral predicate (P4 audit, scout-verified no-op):
#   agents/evolve-triage.md MUST be operating-prompt-only — it must
#   reference agents/evolve-triage-reference.md (via pointer), and the
#   reference-shaped sections (Inbox JSON Schema, Ingestion Algorithm,
#   Reconcile-Compatible Schema, Priority + Weight Scoring, Error Codes)
#   MUST live ONLY in the reference file, NOT duplicated into triage.md.
#
# Cycle-97 T2 is scout-verified complete: this predicate is the
# regression guard. If a future cycle re-introduces reference material
# into evolve-triage.md, this predicate goes RED.
#
# Bash 3.2 compatible. No GNU-only flags.
#
# Exit codes:
#   0 = GREEN (extraction integrity preserved)
#   1 = RED   (duplication or missing pointer detected)
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
if [ -z "$REPO_ROOT" ]; then
  echo "RED: not inside a git work tree" >&2
  exit 1
fi
cd "$REPO_ROOT" || { echo "RED: cd failed" >&2; exit 1; }

TRIAGE="agents/evolve-triage.md"
REF="agents/evolve-triage-reference.md"

if [ ! -f "$TRIAGE" ]; then
  echo "RED: $TRIAGE missing" >&2
  exit 1
fi
if [ ! -f "$REF" ]; then
  echo "RED: $REF missing — reference file must exist for extraction to be valid" >&2
  exit 1
fi

# 1) triage.md must point at the reference (Reference Index pattern).
if ! grep -q 'evolve-triage-reference\.md' "$TRIAGE"; then
  echo "RED: $TRIAGE does not reference $REF" >&2
  exit 1
fi

# 2) Reference-shaped section headings MUST NOT appear in triage.md.
# These are the sections enumerated in scout-report.md T2 as belonging
# only to the reference file.
REF_HEADINGS="\
Inbox JSON Schema
Ingestion Algorithm
Reconcile-Compatible Schema
Priority + Weight Scoring
Error Codes"

# Helper: is "$heading" present as a markdown heading line in "$file"?
# Uses grep -F for the literal heading text (avoids ERE meta-chars like +)
# and post-filters to lines that start with 1-6 #'s + whitespace + the
# heading text + optional trailing whitespace + EOL. Portable BSD/GNU.
_has_heading() {
  local file="$1" heading="$2"
  # Pull candidate lines containing the literal heading text, then
  # confirm with a separate ERE that the line is heading-shaped.
  grep -nF -- "$heading" "$file" 2>/dev/null \
    | grep -Eq ':[[:space:]]*#{1,6}[[:space:]]+.*$' || return 1
  # The above approves any heading-shaped line containing the heading
  # text. Tighten further: require the heading text to be the *trailing*
  # content of the line (no extra prose after it).
  local literal_heading="${heading}"
  while IFS= read -r line; do
    case "$line" in
      *"# ${literal_heading}"|*"# ${literal_heading} ")
        return 0 ;;
    esac
  done < <(grep -F -- "$heading" "$file" 2>/dev/null)
  return 1
}

fail_count=0
fail_summary=""
while IFS= read -r h; do
  [ -z "$h" ] && continue
  if _has_heading "$TRIAGE" "$h"; then
    fail_count=$(( fail_count + 1 ))
    fail_summary="$fail_summary  - duplicated heading in $TRIAGE: $h"$'\n'
  fi
  if ! _has_heading "$REF" "$h"; then
    fail_count=$(( fail_count + 1 ))
    fail_summary="$fail_summary  - missing from $REF: $h"$'\n'
  fi
done <<< "$REF_HEADINGS"

if [ "$fail_count" -ne 0 ]; then
  printf 'RED: triage extraction integrity violated (%s issue[s]):\n' "$fail_count" >&2
  printf '%s' "$fail_summary" >&2
  exit 1
fi

echo "GREEN: agents/evolve-triage.md is operating-prompt only; reference-shaped sections live in agents/evolve-triage-reference.md"
exit 0
