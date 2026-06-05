#!/usr/bin/env bash
# ACS predicate: researchCache schema section lives in a canonical agent doc
# metadata: cycle=49 slug=claude-md-schema
#
# Re-baselined 2026-06-05: the CLAUDE.md split (commit d8ac721) intentionally
# moved runtime detail to docs/operations/runtime-reference.md. The predicate's
# intent — the Shared Agent Values / researchCache schema is documented in a
# canonical, always-discoverable doc — is preserved by accepting EITHER file.

set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
CLAUDE_MD="$WORKTREE/CLAUDE.md"
RUNTIME_REF="$WORKTREE/docs/operations/runtime-reference.md"
[ -f "$CLAUDE_MD" ] || { echo "FAIL: CLAUDE.md not found"; exit 1; }

# Token present in CLAUDE.md OR the runtime reference it links to.
in_canonical() {
  grep -q "$1" "$CLAUDE_MD" && return 0
  [ -f "$RUNTIME_REF" ] && grep -q "$1" "$RUNTIME_REF" && return 0
  return 1
}

in_canonical "Shared Agent Values"  || { echo "FAIL: 'Shared Agent Values' section missing from CLAUDE.md and runtime-reference.md"; exit 1; }
in_canonical "researchCache"        || { echo "FAIL: 'researchCache' schema missing from canonical docs"; exit 1; }
in_canonical "research_fingerprint" || { echo "FAIL: 'research_fingerprint' field missing from canonical docs"; exit 1; }
in_canonical "research-cache.sh"    || { echo "FAIL: 'research-cache.sh' utility reference missing from canonical docs"; exit 1; }
echo "GREEN: researchCache schema section present in canonical docs (CLAUDE.md or runtime-reference.md)"
