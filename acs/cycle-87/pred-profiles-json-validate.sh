#!/usr/bin/env bash
# AC-ID: cycle-87-profiles-json-validate
# Verifies all 7 phase-agent profiles after Cycle A edit:
#   1. Each file is valid JSON (jq empty exits 0).
#   2. Each profile contains a top-level `research_quota` field (object).
#   3. Each profile's allowed_tools array contains WebSearch, WebFetch, and a
#      Bash entry referencing kb-search.sh.
#   4. Each profile's pre-Cycle-A allowed_tools entries are PRESERVED (no
#      removals). We check against the HEAD-snapshot pre-Cycle-A allow lists
#      by comparing the post-edit allow list as a superset of the pre-edit one
#      (read from git HEAD via git show).
set -uo pipefail

REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
PROFILES_DIR="$REPO_ROOT/.evolve/profiles"

profiles=(intent scout triage tdd-engineer builder auditor retrospective)

fail=0
errors=""

for p in "${profiles[@]}"; do
  file="$PROFILES_DIR/$p.json"

  if [ ! -f "$file" ]; then
    errors="${errors}\n  MISSING: $p.json"
    fail=$((fail + 1)); continue
  fi

  # (1) Valid JSON.
  if ! jq -e . "$file" >/dev/null 2>&1; then
    errors="${errors}\n  INVALID-JSON: $p.json"
    fail=$((fail + 1)); continue
  fi

  # (2) research_quota field present and is an object with at least one numeric
  # quota value.
  rq_kind=$(jq -r '.research_quota | type' "$file" 2>/dev/null)
  if [ "$rq_kind" != "object" ]; then
    errors="${errors}\n  $p.json: research_quota missing or not object (got $rq_kind)"
    fail=$((fail + 1)); continue
  fi
  rq_numeric_count=$(jq -r '[.research_quota | to_entries[] | select(.value | type == "number")] | length' "$file" 2>/dev/null)
  if [ "${rq_numeric_count:-0}" -lt 1 ] 2>/dev/null; then
    errors="${errors}\n  $p.json: research_quota has no numeric quota values"
    fail=$((fail + 1)); continue
  fi

  # (3) allowed_tools contains WebSearch, WebFetch, and a kb-search.sh Bash entry.
  has_websearch=$(jq -r 'any(.allowed_tools[]?; . == "WebSearch")' "$file" 2>/dev/null)
  has_webfetch=$(jq -r 'any(.allowed_tools[]?; . == "WebFetch")' "$file" 2>/dev/null)
  has_kb=$(jq -r 'any(.allowed_tools[]?; test("kb-search\\.sh"))' "$file" 2>/dev/null)

  if [ "$has_websearch" != "true" ]; then
    errors="${errors}\n  $p.json: allowed_tools missing WebSearch"
    fail=$((fail + 1))
  fi
  if [ "$has_webfetch" != "true" ]; then
    errors="${errors}\n  $p.json: allowed_tools missing WebFetch"
    fail=$((fail + 1))
  fi
  if [ "$has_kb" != "true" ]; then
    errors="${errors}\n  $p.json: allowed_tools missing Bash(kb-search.sh:*) entry"
    fail=$((fail + 1))
  fi

  # (4) Supersedence check vs. git HEAD baseline (pre-Cycle-A allow list).
  # Skip gracefully if git can't show the file (fresh repo / first commit).
  baseline=$(git -C "$REPO_ROOT" show "HEAD:.evolve/profiles/$p.json" 2>/dev/null || true)
  if [ -n "$baseline" ]; then
    missing=$(printf '%s' "$baseline" \
      | jq -r --slurpfile cur "$file" \
          '(.allowed_tools // []) as $old
           | ($cur[0].allowed_tools // []) as $new
           | ($old - $new) | .[]' 2>/dev/null)
    if [ -n "$missing" ]; then
      while IFS= read -r entry; do
        [ -n "$entry" ] || continue
        errors="${errors}\n  $p.json: REMOVED pre-existing allowed_tools entry: $entry"
        fail=$((fail + 1))
      done <<< "$missing"
    fi
  fi
done

if [ $fail -gt 0 ]; then
  echo "RED cycle-87-profiles-json-validate: $fail issues across 7 profiles"
  printf "%b\n" "$errors" >&2
  exit 1
fi
echo "GREEN cycle-87-profiles-json-validate: all 7 profiles valid, research_quota set, allowed_tools widened, no removals"
exit 0
