#!/usr/bin/env bash
# AC-ID:         cycle-227-004
# Description:   phaseregistrar.Register wires PersonaArtifactDrift: conflict = hard error naming artifact; absent = WARN to stderr, still registers
# Evidence:      go/internal/phaseregistrar/registrar.go:75 + go/internal/phaseregistrar/artifact_drift_test.go
# Author:        tester
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: build-report.md AC#4 — PersonaArtifactDrift check wired in registrar (Mode 4 fix)

set -uo pipefail

WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"

REG="$WORKTREE/go/internal/phaseregistrar/registrar.go"
[ -f "$REG" ] || { echo "RED: $REG not found" >&2; exit 1; }

# Structural: PersonaArtifactDrift must be called inside Register.
grep -q "phasecontract.PersonaArtifactDrift" "$REG" \
  || { echo "RED: PersonaArtifactDrift not referenced in $REG" >&2; exit 1; }

# Behavioral: conflict case = hard error naming the bad artifact;
# absent case = WARN to stderr + registration proceeds (runner not nil).
cd "$WORKTREE/go" || { echo "RED: cannot cd to $WORKTREE/go" >&2; exit 1; }

if ! go test ./internal/phaseregistrar/... \
  -run "TestRegister_ArtifactNameConflictRejected|TestRegister_ArtifactNameAbsentWarns" \
  -timeout 60s 2>&1; then
  echo "RED: phaseregistrar artifact drift test suite FAILED" >&2
  exit 1
fi

echo "GREEN: PersonaArtifactDrift wired — conflict hard-errors, absent warns and registers" >&2
exit 0
