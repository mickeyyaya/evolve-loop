# Eval: backfill-ledger-and-docs

## Metadata
- slug: backfill-ledger-and-docs
- cycle: 173
- task: Add kind=backfill ledger entry on TryExtract success + document phase-timing.json/failure-diag.json in CLAUDE.md and docs/

## Acceptance Criteria

### AC-1: kind=backfill ledger entry emitted on successful TryExtract [code]
When backfill.TryExtract succeeds (EVOLVE_BACKFILL_ENABLED=1), a LedgerEntry with
Kind="backfill" is appended before the WARN verdict continues.
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -run TestOrchestrator_Backfill_LedgerEntry -count=1 -timeout 30s -v 2>&1 | grep -E "--- PASS|--- FAIL|\[no tests to run\]"
```
Expected: `--- PASS: TestOrchestrator_Backfill_LedgerEntry` (no "no tests to run" line)

### AC-2: CLAUDE.md documents phase-timing.json [code]
```bash
grep -c "phase-timing" /Users/danleemh/ai/claude/evolve-loop/CLAUDE.md
```
Expected: at least 1 (entry in the output/diagnostics subsystem table)

### AC-3: CLAUDE.md documents failure-diag.json [code]
```bash
grep -c "failure-diag" /Users/danleemh/ai/claude/evolve-loop/CLAUDE.md
```
Expected: at least 1

### AC-4: docs/architecture/phase-timing-and-diagnostics.md exists [code]
```bash
test -f /Users/danleemh/ai/claude/evolve-loop/docs/architecture/phase-timing-and-diagnostics.md && echo "OK"
```
Expected: OK

### AC-5: phase-timing-and-diagnostics.md covers phase-timing.json format [code]
```bash
grep -c "duration_ms" /Users/danleemh/ai/claude/evolve-loop/docs/architecture/phase-timing-and-diagnostics.md
```
Expected: at least 1

### AC-6: phase-timing-and-diagnostics.md covers failure-diag.json format [code]
```bash
grep -c "failure-diag" /Users/danleemh/ai/claude/evolve-loop/docs/architecture/phase-timing-and-diagnostics.md
```
Expected: at least 2 (file name + format section)

### AC-7: phase-timing-and-diagnostics.md covers phase_retry and backfill ledger kinds [code]
```bash
grep -cE "phase_retry|kind.*backfill|backfill.*kind" /Users/danleemh/ai/claude/evolve-loop/docs/architecture/phase-timing-and-diagnostics.md
```
Expected: at least 2

### AC-8: No backfill ledger entry when EVOLVE_BACKFILL_ENABLED is not set [code]
No kind=backfill entry should appear in a normal (non-backfill) cycle.
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -run TestOrchestrator_Backfill_NoLedgerEntryWhenDisabled -count=1 -timeout 30s -v 2>&1 | grep -E "--- PASS|--- FAIL|\[no tests to run\]"
```
Expected: `--- PASS: TestOrchestrator_Backfill_NoLedgerEntryWhenDisabled` (no "no tests to run" line)

### AC-9: All existing core tests still pass [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... ./internal/backfill/... -timeout 90s -count=1 2>&1 | grep -E "^ok |^FAIL "
```
Expected: lines starting with `ok` only (no FAIL lines)

## Negative Cases

### NC-1: backfill ledger entry has correct Role = phase name [code]
The ledger entry must carry the phase name (e.g. "scout") in the Role field, not a generic value.
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -run TestOrchestrator_Backfill_LedgerRole -count=1 -timeout 30s -v 2>&1 | grep -E "--- PASS|--- FAIL|\[no tests to run\]"
```
Expected: `--- PASS: TestOrchestrator_Backfill_LedgerRole` (no "no tests to run" line)
