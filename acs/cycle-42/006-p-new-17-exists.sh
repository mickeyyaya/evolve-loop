#!/usr/bin/env bash
# ACS predicate: verify P-NEW-17 section exists in token-reduction-roadmap.md
# cycle: 42
# ac: AC1 — P-NEW-17 section heading exists; AC2 — P-NEW-17 status table entry exists; AC3 — knowledge-base research file exists
# metadata: {"id":"006","slug":"p-new-17-exists","cycle":42,"author":"builder"}
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || { echo "ERR: not a git repo"; exit 1; }
ROADMAP="$REPO_ROOT/docs/architecture/token-reduction-roadmap.md"
KB_FILE="$REPO_ROOT/knowledge-base/research/cache-ttl-march-2026-impact.md"

[ -f "$ROADMAP" ] || { echo "ERR: $ROADMAP not found"; exit 1; }

rc=0

# AC1: P-NEW-17 section heading exists in roadmap
if ! grep -q 'P-NEW-17' "$ROADMAP"; then
    echo "FAIL AC1: P-NEW-17 section heading not found in roadmap"
    rc=1
else
    echo "PASS AC1: P-NEW-17 section heading found in roadmap"
fi

# AC2: P-NEW-17 status table entry exists
if ! grep -q 'P-NEW-17.*RESEARCH\|P-NEW-17.*PENDING\|P-NEW-17.*cycle 43' "$ROADMAP"; then
    echo "FAIL AC2: P-NEW-17 status table entry with investigation target not found"
    rc=1
else
    echo "PASS AC2: P-NEW-17 status table entry found"
fi

# AC3: knowledge-base research dossier exists
if [ ! -f "$KB_FILE" ]; then
    echo "FAIL AC3: $KB_FILE not found — knowledge-base stewardship requirement not met"
    rc=1
else
    echo "PASS AC3: $KB_FILE exists (knowledge-base research dossier created)"
fi

exit $rc
