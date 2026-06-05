#!/usr/bin/env bash
# AC-ID:         cycle-227-003
# Description:   profiles.Loader.Resolve(name, role) fallback chain: name.json -> default-<role>.json -> zero Profile (nil error)
# Evidence:      go/internal/profiles/profiles.go:163 + go/internal/profiles/resolve_test.go
# Author:        tester
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: build-report.md AC#3 — Loader.Resolve fallback chain (Mode 3 fix)

set -uo pipefail

WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"

PROFILES="$WORKTREE/go/internal/profiles/profiles.go"
[ -f "$PROFILES" ] || { echo "RED: $PROFILES not found" >&2; exit 1; }

# Structural: Resolve must exist with (name, role string) signature.
grep -q "func (l \*Loader) Resolve(name, role string)" "$PROFILES" \
  || { echo "RED: Resolve(name, role string) not defined in $PROFILES" >&2; exit 1; }

# Behavioral: run all four contract cases — name hit, role fallback, zero default, malformed error.
cd "$WORKTREE/go" || { echo "RED: cannot cd to $WORKTREE/go" >&2; exit 1; }

if ! go test ./internal/profiles/... \
  -run "TestResolve_NameHit|TestResolve_RoleFallback|TestResolve_ZeroProfileDefault|TestResolve_MalformedNameFileErrors" \
  -timeout 60s 2>&1; then
  echo "RED: profiles.Resolve fallback-chain test suite FAILED" >&2
  exit 1
fi

echo "GREEN: profiles.Loader.Resolve fallback chain verified (name->role->zero)" >&2
exit 0
