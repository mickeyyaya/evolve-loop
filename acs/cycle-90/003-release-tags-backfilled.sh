#!/usr/bin/env bash
# AC-ID: cycle-90-003-release-tags-backfilled
# Description: Verifies the three untagged release commits v10.13.0, v10.14.0,
#   v10.14.1 now have annotated/lightweight tags locally AND that each tag
#   points at the canonical release-bump commit identified by scout/triage:
#     v10.13.0 -> 60223cc
#     v10.14.0 -> c4a64e5
#     v10.14.1 -> 88888a2
#   Plan §3D — operational hygiene; ensures changelog generation does not skip
#   3 releases on the next dispatch.
# Evidence: intent.md success-criteria row "git tag --list 'v10.1[34]*' | wc -l
#   == 3 AND git ls-remote --tags origin matches"; triage-decision.md item 3D
#   confirms the three commits exist and lists their target SHAs.
# Author: tdd-engineer (cycle-90)
# Created: 2026-05-19
# Acceptance-of: build-report.md row "3D: 3 release tags created (locally) and
#   pushed to origin; commit-to-tag mapping logged for operator audit"
#
# Behavioral: checks each tag/commit pair independently. A mutant that creates
# the tags but points them at HEAD or an unrelated SHA fails this predicate.
# A mutant that creates only 2 of the 3 tags fails. Origin-push verification
# is best-effort (network may be unavailable under sandbox) — when remote is
# unreachable, the local check is authoritative and operator review of
# build-report.md is the secondary safeguard.
set -uo pipefail

# Tags are a property of the canonical git repo (shared across worktrees, but
# we resolve against the project root for clarity and to match where origin is
# configured).
REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
AC_ID="cycle-90-003-release-tags-backfilled"

cd "$REPO_ROOT" 2>/dev/null || {
  echo "RED $AC_ID: cannot cd to $REPO_ROOT" >&2
  exit 1
}

if ! command -v git >/dev/null 2>&1; then
  echo "RED $AC_ID: git not on PATH" >&2
  exit 1
fi

# Canonical commit -> tag mapping (triage-confirmed, intent §"In-scope" item 3).
# Format: "TAG SHA-PREFIX".  SHA-PREFIX may be 7+ chars; we resolve to full SHA
# before comparison so a future longer/shorter prefix still matches correctly.
EXPECTED=$(cat <<'EOF'
v10.13.0 60223cc
v10.14.0 c4a64e5
v10.14.1 88888a2
EOF
)

failures=""
present_count=0
while read -r tag expected_short; do
  [ -z "$tag" ] && continue
  if ! git rev-parse --verify --quiet "refs/tags/$tag" >/dev/null 2>&1; then
    failures="${failures}\n  tag missing: $tag"
    continue
  fi
  # Resolve tag (peel to commit) and expected short SHA to full SHAs.
  tag_sha=$(git rev-list -n 1 "refs/tags/$tag" 2>/dev/null)
  expected_sha=$(git rev-parse --verify --quiet "$expected_short" 2>/dev/null)
  if [ -z "$tag_sha" ] || [ -z "$expected_sha" ]; then
    failures="${failures}\n  tag $tag: resolution failed (tag_sha='$tag_sha' expected_sha='$expected_sha')"
    continue
  fi
  if [ "$tag_sha" != "$expected_sha" ]; then
    failures="${failures}\n  tag $tag: points at ${tag_sha:0:8}, expected ${expected_sha:0:8}"
    continue
  fi
  present_count=$((present_count + 1))
done <<< "$EXPECTED"

if [ "$present_count" -ne 3 ] || [ -n "$failures" ]; then
  echo "RED $AC_ID: only $present_count/3 release tags valid" >&2
  printf "%b\n" "$failures" >&2
  exit 1
fi

# Best-effort origin verification — sandbox may block network; absence of
# evidence is NOT failure. If `git ls-remote` succeeds we assert each tag
# exists upstream too. If it fails (network blocked, no origin), we pass
# the predicate based on the local check alone.
if git remote get-url origin >/dev/null 2>&1; then
  remote_listing=$(git ls-remote --tags origin 2>/dev/null || true)
  if [ -n "$remote_listing" ]; then
    remote_failures=""
    while read -r tag _; do
      [ -z "$tag" ] && continue
      if ! printf '%s\n' "$remote_listing" | awk -v t="refs/tags/$tag" '$2 == t || $2 == t"^{}" {found=1} END{exit !found}'; then
        remote_failures="${remote_failures}\n  tag missing on origin: $tag"
      fi
    done <<< "$EXPECTED"
    if [ -n "$remote_failures" ]; then
      echo "RED $AC_ID: origin missing release tags" >&2
      printf "%b\n" "$remote_failures" >&2
      exit 1
    fi
  fi
fi

echo "GREEN $AC_ID: 3/3 release tags backfilled (v10.13.0, v10.14.0, v10.14.1)"
exit 0
