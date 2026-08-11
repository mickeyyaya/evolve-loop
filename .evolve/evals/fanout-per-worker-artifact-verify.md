# Eval: fanout-per-worker-artifact-verify

## Task Slug
fanout-per-worker-artifact-verify

## Description
Verify that DispatchParallel now performs per-worker artifact verification using
the subagent.Verify SSOT (freshness skipped) before calling the aggregator. Any
worker artifact that is missing, empty, or unreadable must block aggregation and
populate WorkerVerifyFailures in the result.

## Acceptance Criteria

### AC1 — VerifyWorkerArtifact seam present in DispatchParallelOptions [code]

```bash
grep -c "VerifyWorkerArtifact" \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/subagent/dispatchparallel.go
```

Expected: output ≥ 1 (seam field exists)

### AC2 — WorkerVerifyFailures field in DispatchParallelResult [code]

```bash
grep -c "WorkerVerifyFailures" \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/subagent/dispatchparallel.go
```

Expected: output ≥ 2 (declaration + assignment)

### AC3 — Freshness is skipped in the default seam implementation [code]

```bash
grep -n "MaxInt64\|math\.MaxInt\|NoFreshness\|SkipFreshness\|maxDuration\|MaxDuration\|365\*24" \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/subagent/dispatchparallel.go | head -5
```

Expected: at least 1 match showing freshness is effectively infinite

### AC4 — Missing worker artifact produces WorkerVerifyFailures and skips aggregation [code]

```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/subagent/... \
    -run "TestDispatchParallel_WorkerArtifact(Missing|Empty|Unreadable|Verify)" \
    -count=1 -v 2>&1 | grep -E "^--- (PASS|FAIL)"
```

Expected: all matched tests report PASS; at least 3 test cases matched

### AC5 — Happy path: all workers valid → aggregation still runs, no failures [code]

```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/subagent/... \
    -run "TestDispatchParallel_HappyPath" \
    -count=1 -v 2>&1 | grep -E "^--- (PASS|FAIL)"
```

Expected: `--- PASS: TestDispatchParallel_HappyPath`

### AC6 — Full subagent test suite passes with no regression [code]

```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/subagent/... -count=1 2>&1 | tail -3
```

Expected: `ok github.com/mickeyyaya/evolve-loop/go/internal/subagent`

### Negative case — Fanout success but worker artifact has no content MUST NOT reach aggregator [code]

```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/subagent/... \
    -run "TestDispatchParallel_WorkerArtifactEmpty" \
    -count=1 -v 2>&1 | grep -E "aggregator|AggregatorExit|WorkerVerifyFail"
```

Expected: output shows AggregatorExit=0 (aggregator not called) AND WorkerVerifyFailures non-empty

### Edge case — Verification with empty token (no token check) must not false-reject valid artifact [code]

```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/subagent/... \
    -run "TestDispatchParallel_WorkerArtifactVerify(Valid|HappyPath|NoToken)" \
    -count=1 2>&1 | grep -E "PASS|FAIL"
```

Expected: PASS (valid artifacts with non-empty content pass verification regardless of token)
