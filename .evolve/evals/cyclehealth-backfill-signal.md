# Eval: cyclehealth-backfill-signal

## Goal
Add signal 13 `backfill_events` to cyclehealth so that backfill recoveries
(ledger kind="backfill") surface in cycle-health.json anomalies instead of
being silently absent from health monitoring.

## Acceptance Criteria

### AC1 — signalNames returns 13 entries [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/cyclehealth/... -run TestSignalNames -v 2>&1 | grep -q "PASS"
```

### AC2 — backfill_events present in signal names [code]
```bash
grep -q "backfill_events" /Users/danleemh/ai/claude/evolve-loop/go/internal/cyclehealth/cyclehealth.go
```

### AC3 — All cyclehealth tests pass [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/cyclehealth/... 2>&1 | grep -q "^ok"
```

### AC4 — ledgerEntry struct has Kind field [code]
```bash
grep -A10 "type ledgerEntry struct" /Users/danleemh/ai/claude/evolve-loop/go/internal/cyclehealth/cyclehealth.go | grep -q "Kind"
```

### AC5 — artifact-backfill.md mentions backfill_events signal [code]
```bash
grep -q "backfill_events" /Users/danleemh/ai/claude/evolve-loop/docs/architecture/artifact-backfill.md
```

### AC6 — Negative: old 12-count test must be gone or updated [code]
```bash
# The test must not claim count == 12 if it now expects 13
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/cyclehealth/... -v 2>&1 | grep -v "FAIL"
# Verify no test fails due to count mismatch
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/cyclehealth/... 2>&1 | grep -qv "FAIL"
```
