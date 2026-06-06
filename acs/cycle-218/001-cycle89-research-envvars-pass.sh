#!/usr/bin/env bash
# ACS cycle-218 / task fix-stale-acs-predicates AC1 — cycle89 env-var
# predicate passes again after relocating its check from CLAUDE.md to
# docs/operations/runtime-reference.md (the d8ac721 split moved the env-var
# table there; the test went stale).
#
# Behavioral: asserts on `go test` EXIT CODE via the shared assert lib
# (cycle-131/137 lessons — never scrape PASS/ok strings). The grep is an
# auxiliary anti-deletion guard: the fix must RELOCATE the
# EVOLVE_RESEARCH_CACHE_ENABLED assertion, not delete it.
set -uo pipefail
TOP=$(git rev-parse --show-toplevel) || { echo "RED: not a git repo" >&2; exit 1; }
. "$TOP/acs/lib/assert.sh"

assert_go_test_pass ./acs/cycle89/... 'TestC89_ClaudeMdResearchEnvVars' || exit 1

# auxiliary (not load-bearing): the assertion moved, it did not vanish
if ! grep -qF 'EVOLVE_RESEARCH_CACHE_ENABLED' "$TOP/go/acs/cycle89/predicates_test.go"; then
  echo "RED: cycle89 test no longer asserts EVOLVE_RESEARCH_CACHE_ENABLED — check was deleted, not relocated" >&2
  exit 1
fi
if ! grep -qF 'runtime-reference.md' "$TOP/go/acs/cycle89/predicates_test.go"; then
  echo "RED: cycle89 test does not target docs/operations/runtime-reference.md" >&2
  exit 1
fi
echo "GREEN: cycle89 env-var predicate passes and targets runtime-reference.md" >&2
exit 0
