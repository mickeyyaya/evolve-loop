# Eval: ci-coverage-gate-staged-enforcement

## Purpose
Verify the CI coverage gate advances from warning-only (exit 0 always) toward
staged enforcement where packages below 85% threshold actually fail the gate.

## Code Graders [code]

- `grep -rn "exit 0" .github/workflows/ 2>/dev/null | grep -v "Phase 1 build.*warning-only\|warning.*until task" | grep -c "coverage gate" | grep "^0$"` — the "exit 0 always" bypass is replaced
- `grep -rn "exit.*fail\|exit \$fail" .github/workflows/ 2>/dev/null | grep -q "coverage"` — enforcement path added for coverage gate

## Regression Graders

- `grep -rn "coverage gate" .github/workflows/ 2>/dev/null | grep -qv "^Binary"` — coverage gate section still exists
- `grep -rn "::warning::coverage" .github/workflows/ 2>/dev/null | grep -qv "^Binary"` — warning annotation retained for visibility

## Acceptance Notes
- Staged means: the gate now FAILS (non-zero exit) when packages fall below 85%
- The warning annotation should be retained alongside enforcement
- Comment must be updated to remove the "warning-only" / "Phase 1 build" placeholder text
