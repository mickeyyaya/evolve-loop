# Eval: chaos-malformed-artifact-tests

## Acceptance Criteria

### AC-1: New chaos tests exist in triage package [code]
```bash
grep -q "TestTriage_Chaos\|TestRun_Chaos\|TestRun_Malformed\|TestRun_Truncated\|TestRun_EmptyArtifact\|TestRun_ZeroByte\|TestRun_BinaryContent" \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/phases/triage/triage_test.go
```

### AC-2: New chaos tests exist in scout or runner package [code]
```bash
# At least one chaos/malformed-artifact test exists in scout or runner
grep -rq "TestScout_Chaos\|TestRun_Chaos\|TestRun_Malformed\|TestRun_Truncated\|TestRun_EmptyArtifact\|TestRun_ZeroByte" \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/phases/scout/ \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/phases/runner/ 2>/dev/null
```

### AC-3: Chaos tests verify graceful WARN/FAIL (not panic) for zero-byte artifact [code]
```bash
# Tests must assert on verdict, not just compile
grep -rn "VerdictFAIL\|VerdictWARN\|Verdict.*FAIL\|Verdict.*WARN\|want.*FAIL\|want.*WARN\|wantVerdict" \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/phases/triage/triage_test.go | grep -i "chaos\|malform\|truncat\|empty\|zero" | head -5
# Alternative: just check test functions exist AND tests pass
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phases/triage/... -count=1 -v 2>&1 | grep -E "PASS|FAIL" | tail -5
```

### AC-4: Triage chaos tests pass (no panic, no compile error) [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phases/triage/... -count=1 2>&1
```

### AC-5: Scout chaos tests pass [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phases/scout/... -count=1 2>&1
```

### AC-6: Runner tests pass (including any new chaos tests) [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phases/runner/... -count=1 2>&1 | grep -E "ok|FAIL"
```

### AC-7: Negative case — chaos test for 0-byte artifact returns FAIL verdict [code]
```bash
# A 0-byte artifact must produce FAIL, not panic or PASS
# Verify the test name contains an assertion on verdict=FAIL
grep -A 10 "TestTriage_Chaos\|TestRun_.*Empty\|TestRun_.*Zero" \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/phases/triage/triage_test.go 2>/dev/null | grep -i "FAIL\|fail\|error\|Error" | head -5
```

### AC-8: Backfill chaos test: truncated clean.txt yields (false, nil) [code]
```bash
# backfill.TryExtract with content shorter than minLen must return false, nil
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/backfill/... -count=1 -run "TestTryExtract" -v 2>&1 | tail -10
```
