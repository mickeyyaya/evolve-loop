# Eval: ledger-skip-source-attribution

## Goal
Validate that `phase_skipped` ledger entries carry a `source` field identifying why the phase was skipped (`psmas|router|content`).

## Acceptance Criteria

### AC-1: LedgerEntry struct has Source field [code]
```bash
grep -n "Source\b" /Users/danleemh/ai/claude/evolve-loop/go/internal/core/ledger.go | grep -i "source\|psmas\|router\|content"
```
Expected: at least one match showing the `Source` field in the `LedgerEntry` struct or a typed constant declaration.

### AC-2: phase_skipped entries carry source in orchestrator [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -run TestPhaseSkipped_SourceAttribution -v
```
Expected: PASS. Test must verify that a `phase_skipped` ledger entry appended by the router path carries `"source":"router"` and a psmas-gated skip carries `"source":"psmas"`.

### AC-NEGATIVE: Missing source fails test (not silently empty) [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -run TestPhaseSkipped_EmptySourceIsError -v
```
Expected: PASS. A `phase_skipped` entry with an empty `Source` must be detected by the ACS predicate test (verifying the field is mandatory, not optional).

### AC-3: ACS cycle-98/005 still passes [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./acs/cycle98/... -v
```
Expected: All cycle-98 ACS tests PASS. The new Source field must not break the existing `phase_skipped` predicate tests.

### AC-FULL-BUILD: No regressions [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go build ./... 2>&1
```
Expected: exits 0 with no errors.
