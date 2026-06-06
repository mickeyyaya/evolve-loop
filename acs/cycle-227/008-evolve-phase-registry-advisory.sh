#!/usr/bin/env bash
# AC-ID:         cycle-227-008
# Description:   .evolve/phase-registry.json pins dynamic_routing=advisory; file is git-tracked; .gitignore whitelist entry present
# Evidence:      .evolve/phase-registry.json + .gitignore:!.evolve/phase-registry.json + config.dynamic_routing_default_test.go:TestRealRegistry_EvolveAdvisoryPinned
# Author:        tester
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: build-report.md AC#8 — .evolve/phase-registry.json advisory default (user-phase-pipeline-hardening)

set -uo pipefail

WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"

REGISTRY="$WORKTREE/.evolve/phase-registry.json"
[ -f "$REGISTRY" ] || { echo "RED: $REGISTRY does not exist" >&2; exit 1; }

# Structural: the file must contain dynamic_routing=advisory (not "0" or "off").
grep -q '"dynamic_routing".*"advisory"' "$REGISTRY" \
  || { echo "RED: $REGISTRY does not pin dynamic_routing=advisory" >&2; exit 1; }

# Git-tracking: cycle-92 defect mode — untracked .evolve files are silently dropped at ship.
git -C "$WORKTREE" ls-files --error-unmatch ".evolve/phase-registry.json" >/dev/null 2>&1 \
  || { echo "RED: .evolve/phase-registry.json is not git-tracked in worktree (cycle-92 mode)" >&2; exit 1; }

# .gitignore whitelist: the entry must be present so future git-adds don't silently drop it.
GITIGNORE="$WORKTREE/.gitignore"
grep -q '!.evolve/phase-registry.json' "$GITIGNORE" \
  || { echo "RED: .gitignore whitelist entry '!.evolve/phase-registry.json' not found" >&2; exit 1; }

# Behavioral: the real Load path must return StageAdvisory without any EVOLVE_DYNAMIC_ROUTING env var.
cd "$WORKTREE/go" || { echo "RED: cannot cd to $WORKTREE/go" >&2; exit 1; }

if ! go test ./internal/config/... \
  -run "TestRealRegistry_EvolveAdvisoryPinned" \
  -timeout 60s 2>&1; then
  echo "RED: TestRealRegistry_EvolveAdvisoryPinned FAILED — advisory not pinned end-to-end" >&2
  exit 1
fi

echo "GREEN: .evolve/phase-registry.json pins advisory, is git-tracked, .gitignore whitelisted, Load returns StageAdvisory" >&2
exit 0
