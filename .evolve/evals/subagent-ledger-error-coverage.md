# Eval: subagent-ledger-error-coverage

## Task Slug
subagent-ledger-error-coverage

## Description
WriteFanoutLedgerEntry (77.8% coverage) and writeSubprocessLedger (69.6% coverage)
have multiple uncovered error branches: MkdirAll failure, readChainLink failure,
open ledger failure, write-line failure, close failure, write-tip failure, and
rename failure. These uncovered paths represent real failure modes that could
silently corrupt the audit chain. This task adds targeted tests for each error
branch to bring both functions to ≥90% coverage.

## Acceptance Criteria

### AC1 — New tests exist for ledger error branches [code]

```bash
grep -c "TestWriteFanoutLedgerEntry_\|TestWriteSubprocessLedger_" \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/subagent/subagent_coverage_test.go \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/subagent/helpers_test.go 2>/dev/null
```

Expected: output ≥ 4 (at least 4 new test functions across the two files)

### AC2 — WriteFanoutLedgerEntry error tests pass [code]

```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/subagent/... \
    -run "TestWriteFanoutLedgerEntry" \
    -count=1 -v 2>&1 | grep -E "^--- (PASS|FAIL)"
```

Expected: all matched tests report PASS; at least 3 test functions matched

### AC3 — writeSubprocessLedger coverage increases from 69.6% baseline [code]

```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/subagent/... -count=1 -coverprofile=/tmp/sa_cov328.out 2>&1 | grep "ok\|FAIL" && \
  go tool cover -func=/tmp/sa_cov328.out 2>&1 | grep "writeSubprocessLedger\|WriteFanoutLedgerEntry"
```

Expected: writeSubprocessLedger ≥ 85%, WriteFanoutLedgerEntry ≥ 88%

### AC4 — Full subagent suite passes with no regression [code]

```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/subagent/... -count=1 2>&1 | tail -3
```

Expected: `ok github.com/mickeyyaya/evolve-loop/go/internal/subagent`

### AC5 — Overall subagent package coverage does not regress [code]

```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/subagent/... -count=1 -cover 2>&1 | grep "coverage:"
```

Expected: coverage ≥ 97% (up from 96.7% baseline)

### Negative case — MkdirAll failure in WriteFanoutLedgerEntry returns an error, not panic [code]

```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/subagent/... \
    -run "TestWriteFanoutLedgerEntry_MkdirFails" \
    -count=1 -v 2>&1 | grep -E "^--- (PASS|FAIL)"
```

Expected: `--- PASS: TestWriteFanoutLedgerEntry_MkdirFails`

### Edge case — WriteFanoutLedgerEntry with empty AggregatePath skips SHA computation [code]

```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/subagent/... \
    -run "TestWriteFanoutLedgerEntry_EmptyAggregatePath" \
    -count=1 -v 2>&1 | grep -E "^--- (PASS|FAIL)"
```

Expected: `--- PASS: TestWriteFanoutLedgerEntry_EmptyAggregatePath`
