# Eval: fix-abort-cleanup-gitignored

**Slug:** fix-abort-cleanup-gitignored
**Phase target:** build
**Grader mix:** [code] primary

## Acceptance Criteria

### AC-1 [code] — Tests pass with no regression
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -count=1 -timeout 120s 2>&1 | tail -10
# EXIT 0 required
```

### AC-2 [code] — discardMainLeak guarded by gitignore check
```bash
grep -n "check-ignore\|isGitignored\|gitIgnored\|gitignore" /Users/danleemh/ai/claude/evolve-loop/go/internal/core/orchestrator.go
# Must match ≥1 line; confirms a gitignore gate exists in the discard path
```

### AC-3 [code] — Negative: gitignored path is NOT passed to git checkout HEAD
```bash
# Verify test exists that confirms gitignored path is skipped by discardMainLeak
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -run "TestGitignored\|TestDiscardGitignore\|TestIgnoredPath" -v 2>&1 | grep -E "PASS|FAIL|RUN"
# Must show at least one gitignore-related test result; must not FAIL
```

### AC-4 [code] — Binary absence loud warning present
```bash
grep -n "go/bin/evolve\|absent\|missing.*binary\|binary.*absent\|guard.binary\|Abnormal\|abnormal" /Users/danleemh/ai/claude/evolve-loop/go/internal/core/orchestrator.go | grep -i "warn\|stderr\|Fprintf\|abnormal\|absent"
# Must match ≥1 line — the orchestrator logs loudly when go/bin/evolve is absent post-abort
```

### AC-5 [code] — relBin discard skips gitignored paths
```bash
# Verify the relBin discard block (lines near 1857-1861 in original) is now guarded
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -run "TestTreeDiff\|TestRelBin" -v 2>&1 | grep -E "PASS|FAIL"
# Must not FAIL
```

## Gaming detection
Trivial fake: removing the discard call entirely. Refuted by AC-1 (existing build-leak tests expect binary churn to be discarded). Edge: gitignored path must be skipped WITHOUT removing the tracked-path discard for go/evolve.
