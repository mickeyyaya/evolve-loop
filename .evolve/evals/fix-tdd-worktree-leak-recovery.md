# Eval: fix-tdd-worktree-leak-recovery

**Slug:** fix-tdd-worktree-leak-recovery
**Phase target:** tdd + build
**Grader mix:** [code] primary

## Acceptance Criteria

### AC-1 [code] — Tests pass with no regression
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -count=1 -timeout 120s 2>&1 | tail -10
# EXIT 0 required; all packages report "ok"
```

### AC-2 [code] — recoverBuildLeak runs for TDD phase (not just build)
```bash
grep -n "WorktreePhase(next)" /Users/danleemh/ai/claude/evolve-loop/go/internal/core/orchestrator.go
# Must match at least one line; the condition no longer hardcodes PhaseBuild only
```

### AC-3 [code] — cycle-243 scenario: TDD-phase tracked-file leak is recovered, not abort
```bash
# A test covering the pattern: TDD phase leaks a tracked file → recoverBuildLeak runs → cycle continues
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -run "TestTDD" -v 2>&1 | grep -E "PASS|FAIL|tdd"
# Must show PASS for TDD-related tests; no FAIL
```

### AC-4 [code] — Negative: non-worktree phase leak still aborts
```bash
# Confirm non-worktree phases (e.g. audit) still abort on tracked-file leak
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -run "TestAuditLeak" -v 2>&1 | grep -E "PASS|FAIL"
# Must not regress: audit-leak tests still PASS (they verify the abort path)
```

### AC-5 [code] — No regression in existing build-leak recovery tests
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -run "TestBuildLeak\|TestRecoverBuild" -v 2>&1 | grep -E "PASS|FAIL|ok"
# All must PASS
```

## Gaming detection
Trivial fake: always returning "recovered" regardless of leak type. This is refuted by AC-4 (non-worktree phases must still abort) and AC-5 (existing build-leak tests use real path logic).
