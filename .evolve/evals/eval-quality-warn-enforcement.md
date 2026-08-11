# Eval: eval-quality-warn-enforcement

## Purpose
Verify that WARN-level eval quality-check returns non-zero exit code (blocking
callers) and add a negative test confirming a diversity-check WARN on
positive-only suites exits non-zero. This advances the "WARN-only → fail-gate"
promotion path.

## Code Graders [code]

### AC-1: WARN exit-code contract enforced by test [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./cmd/evolve/... -run TestEvalQualityCheck_WarnReturnsNonZero -v 2>&1 | grep -q "PASS"
```

### AC-2: Diversity WARN returns exit 1 (blocks) [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./cmd/evolve/... -run "TestEvalDiversityCheck.*Warn\|TestEvalDiversityCheck.*PositiveOnly" -v 2>&1 | grep -q "PASS"
```

### AC-3: Quality-check HALT returns exit 2 [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./cmd/evolve/... -run TestEvalQualityCheck_HaltReturns2 -v 2>&1 | grep -q "PASS"
```

## Regression Graders

- `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./cmd/evolve/... 2>&1 | grep -c "^--- FAIL" | grep "^0$"` — all cmd/evolve tests pass
- `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/evalqualitycheck/... 2>&1 | grep -c "^--- FAIL" | grep "^0$"` — evalqualitycheck tests pass

## Acceptance Notes
- Tests must be deterministic (use existing fake/stub evals, not real AI calls)
- Tests verify the exit-code contract: WARN=1, HALT=2, PASS=0
- These tests document and pin the current blocking behavior (even if not yet wired into cycle gating)
- Negative case: PASS-only eval suite → diversity-check WARN → exit 1 (not 0)
