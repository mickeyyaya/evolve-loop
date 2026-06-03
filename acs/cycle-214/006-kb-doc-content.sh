#!/usr/bin/env bash
# ACS cycle-214 — the missing-development-phases KB doc exists with the required
# structure, coverage, and size (Task 3 ACs).
#
# acs-predicate: config-check — this is a pure DOCUMENTATION deliverable; there
# is no code/process to invoke, so content+presence assertions on the markdown
# are the correct verification. The strict judgment "does NOT recommend making
# phases mandatory" (AC3.4) is dispositioned manual+checklist for the Auditor;
# here we assert the positive optional-only-safety affirmation.
set -uo pipefail

ROOT="$(git rev-parse --show-toplevel)"
REL="knowledge-base/research/missing-development-phases-2026-06-03.md"
DOC="$ROOT/$REL"

[ -f "$DOC" ] || { echo "RED: $REL missing on disk"; exit 1; }
git -C "$ROOT" ls-files --error-unmatch "$REL" >/dev/null 2>&1 \
  || { echo "RED: $REL untracked — may be dropped at ship"; exit 1; }

bytes=$(wc -c < "$DOC" | tr -d ' ')
[ "${bytes:-0}" -gt 500 ] || { echo "RED: $REL is ${bytes:-0} bytes, need >500"; exit 1; }

# Required structural themes (AC3.2).
grep -qiE 'research findings' "$DOC" || { echo "RED: missing 'Research Findings' section"; exit 1; }
grep -qiE 'missing (development )?phases' "$DOC" || { echo "RED: missing 'Missing Phases' section"; exit 1; }
grep -qiE 'phase design|how to (write|author).*phase\.json|design guide' "$DOC" \
  || { echo "RED: missing 'Phase Design Guide' section"; exit 1; }

# Coverage of the four named phases (AC3.3).
for p in security-scan dependency-audit performance-bench post-ship-monitor; do
  grep -qiE "$p" "$DOC" || { echo "RED: KB doc does not cover $p"; exit 1; }
done

# AC3.4 positive half: affirms the optional-only safety rule.
grep -qiE 'optional' "$DOC" || { echo "RED: KB doc does not state the optional-only safety rule"; exit 1; }

echo "GREEN: KB doc present, >500B, all sections + 4 phases covered, optional-safety affirmed"
exit 0
