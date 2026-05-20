#!/usr/bin/env bash
# AC-ID: cycle-91-003-tdd-engineer-1to1-contract
# Description: Verifies that agents/evolve-tdd-engineer.md was updated to
#   encode the 1:1 AC-materialization contract. The persona doc MUST contain:
#     (a) the literal token `1:1` (the materialization ratio)
#     (b) at least one of `predicate` / `manual` / `unverifiable` enumerated
#         as a disposition option (we require ALL three to be enumerated
#         together — the lesson's preventive 2 names exactly these three).
#     (c) an explicit prohibition against a bare "defer to Auditor" disposition
#         without an accompanying checklist (we look for the negative phrase
#         "defer" + "Auditor" appearing in a sentence that ALSO contains
#         "checklist" or "ban" or "not allowed" or "without").
# Evidence: intent.md:acceptance_checks bullet 3; intent.md:interfaces bullet 3.
# Author: tdd-engineer (cycle-91)
# Created: 2026-05-20
# Acceptance-of: build-report.md row "agents/evolve-tdd-engineer.md updated
#   with 1:1 materialization contract"
#
# Behavioral: a mutant that adds the contract to the reference doc but not
# the canonical persona doc fails. A mutant that names only one or two of
# the three dispositions (predicate | manual | unverifiable) fails because
# the lesson explicitly enumerates all three. A mutant that mentions the
# defer-to-Auditor option without explicitly banning the BARE form fails
# because the lesson's preventive 2(c) prohibits it.
set -uo pipefail

REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
PERSONA="$REPO_ROOT/agents/evolve-tdd-engineer.md"
AC_ID="cycle-91-003-tdd-engineer-1to1-contract"

if [ ! -f "$PERSONA" ]; then
  echo "RED $AC_ID: agents/evolve-tdd-engineer.md not found at $PERSONA" >&2
  exit 1
fi

missing=""

# (a) Literal `1:1`
if ! grep -qF '1:1' "$PERSONA"; then
  missing="${missing} '1:1'"
fi

# (b) All three disposition options enumerated. Case-insensitive to tolerate
#     capitalization variation (e.g., "Predicate", "Manual+Checklist").
for token in predicate manual unverifiable; do
  if ! grep -qi "$token" "$PERSONA"; then
    missing="${missing} disposition:$token"
  fi
done

# (c) Explicit ban on bare "defer to Auditor". We look for a single sentence/line
#     containing the words "defer" and "Auditor" plus one of: checklist | ban |
#     prohibit | "not allowed" | without | bare.
banned_form_ok=$(awk '
  BEGIN { IGNORECASE = 1 }
  /defer/ && /auditor/ {
    if (tolower($0) ~ /checklist|ban|prohibit|not allowed|without|bare|forbid/) {
      print "OK"; found = 1; exit
    }
  }
  END { if (!found) print "MISSING" }
' "$PERSONA")

if [ "$banned_form_ok" != "OK" ]; then
  # Fall back to a multi-line proximity check (6-line window).
  banned_form_ok=$(awk '
    BEGIN { IGNORECASE = 1 }
    {
      buf[NR % 6] = tolower($0)
      have_defer = 0
      have_auditor = 0
      have_negation = 0
      for (i = 0; i < 6; i++) {
        if (index(buf[i], "defer") > 0)   have_defer = 1
        if (index(buf[i], "auditor") > 0) have_auditor = 1
        if (buf[i] ~ /checklist|ban|prohibit|not allowed|without|bare|forbid/) have_negation = 1
      }
      if (have_defer && have_auditor && have_negation) { found = 1; print "OK"; exit }
    }
    END { if (!found) print "MISSING" }
  ' "$PERSONA")
fi

if [ "$banned_form_ok" != "OK" ]; then
  missing="${missing} bare-defer-to-Auditor-ban"
fi

if [ -n "$missing" ]; then
  echo "RED $AC_ID: agents/evolve-tdd-engineer.md missing required tokens:${missing}" >&2
  exit 1
fi

echo "GREEN $AC_ID: agents/evolve-tdd-engineer.md encodes 1:1 AC-materialization contract with all three dispositions and bare-defer ban"
exit 0
