#!/usr/bin/env bash
# AC-ID:         cycle-227-007
# Description:   ResolveRegistryPath(root) prefers .evolve/phase-registry.json (file must exist); falls back to docs/architecture/phase-registry.json
# Evidence:      go/internal/config/config.go:590-592 + go/internal/config/dynamic_routing_default_test.go
# Author:        tester
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: build-report.md AC#7 — ResolveRegistryPath seam (user-phase-pipeline-hardening)

set -uo pipefail

WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"

CFG="$WORKTREE/go/internal/config/config.go"
[ -f "$CFG" ] || { echo "RED: $CFG not found" >&2; exit 1; }

# Structural: ResolveRegistryPath must be defined.
grep -q "func ResolveRegistryPath(root string) string" "$CFG" \
  || { echo "RED: ResolveRegistryPath not defined in $CFG" >&2; exit 1; }

# Structural: the .evolve path preference and file-existence check must be coded.
grep -q '".evolve".*"phase-registry.json"' "$CFG" \
  || { echo "RED: .evolve/phase-registry.json preference not coded in $CFG" >&2; exit 1; }

# Behavioral: prefer override, fallback to docs, empty-dir not a match, neither-file returns docs path.
cd "$WORKTREE/go" || { echo "RED: cannot cd to $WORKTREE/go" >&2; exit 1; }

if ! go test ./internal/config/... \
  -run "TestResolveRegistryPath" \
  -timeout 60s 2>&1; then
  echo "RED: ResolveRegistryPath test suite FAILED" >&2
  exit 1
fi

echo "GREEN: ResolveRegistryPath seam verified — .evolve override wins when file exists, docs fallback otherwise" >&2
exit 0
