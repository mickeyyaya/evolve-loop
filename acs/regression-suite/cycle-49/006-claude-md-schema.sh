#!/usr/bin/env bash
# ACS predicate: CLAUDE.md contains researchCache schema section
# metadata: cycle=49 slug=claude-md-schema

set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
CLAUDE_MD="$WORKTREE/CLAUDE.md"
[ -f "$CLAUDE_MD" ] || { echo "FAIL: CLAUDE.md not found"; exit 1; }

grep -q "Shared Agent Values" "$CLAUDE_MD" || { echo "FAIL: 'Shared Agent Values' section missing"; exit 1; }
grep -q "researchCache" "$CLAUDE_MD"       || { echo "FAIL: 'researchCache' schema missing"; exit 1; }
grep -q "research_fingerprint" "$CLAUDE_MD" || { echo "FAIL: 'research_fingerprint' field missing"; exit 1; }
grep -q "research-cache.sh" "$CLAUDE_MD"   || { echo "FAIL: 'research-cache.sh' utility reference missing"; exit 1; }
echo "GREEN: CLAUDE.md contains researchCache schema section with all required fields"
