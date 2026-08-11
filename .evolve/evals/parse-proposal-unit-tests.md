# Eval: parseProposal focused unit tests

## Task slug
`parse-proposal-unit-tests`

## Acceptance predicate

### [code] All new tests pass, no regressions
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test github.com/mickeyyaya/evolve-loop/go/internal/core \
    -run TestParseProposal -v -count=1 2>&1 | \
  grep -E "^(=== RUN|--- PASS|--- FAIL|PASS|FAIL|ok)" | tee /dev/stderr | \
  grep -v FAIL | grep -q "PASS"
```

### [code] Full core package still green
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test github.com/mickeyyaya/evolve-loop/go/internal/core -count=1 2>&1 | \
  tail -1 | grep -q "^ok"
```

### [code] At least 3 table-driven sub-cases covering fenced JSON, leading prose, and empty/malformed degrade
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test github.com/mickeyyaya/evolve-loop/go/internal/core \
    -run TestParseProposal -v -count=1 2>&1 | \
  grep "=== RUN" | wc -l | awk '{exit ($1 < 4) ? 1 : 0}'
```

### [code] New test calls parseProposal directly (not via Propose)
```bash
grep -c "parseProposal(" \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/core/router_proposer_test.go | \
  awk '{exit ($1 < 1) ? 1 : 0}'
```
