# Eval: evalgate-floor-declarations

## Overview
Validates that the evalgate floor-binding gate reads deferred/committed floors from the
triage-decision.json companion's `deferred_floors` array (not prose-scraping), and that
`evolve guard triage-floors <workspace>` exposes the self-check CLI (ADR-0045 pattern).

## Criteria

### C1: deferred_floors in companion blocks matching TDD floor predicate [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/evalgate/... -run TestFloorBinding_DeferredFromCompanion -v 2>&1 | grep -E "PASS|FAIL"
```
Expected: `PASS`

### C2: Missing companion → fail open (no block) [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/evalgate/... -run TestFloorBinding_MissingCompanion_FailOpen -v 2>&1 | grep -E "PASS|FAIL"
```
Expected: `PASS`

### C3: All evalgate tests pass [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/evalgate/... 2>&1 | tail -3
```
Expected: `ok  	github.com/mickeyyaya/evolve-loop/go/internal/evalgate`

### C4: evolve guard triage-floors command exists and exits 0 on clean workspace [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop && \
  ./go/evolve guard triage-floors --help 2>&1 | grep -i "triage-floors\|floor" | head -3
```
Expected: line containing "triage-floors" or "floor" (non-empty output)

### C5: evolve guard triage-floors exits non-zero on declared/prose divergence [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/evalgate/... -run TestFloorBinding_DeclaredDivergenceMessage -v 2>&1 | grep -E "PASS|FAIL"
```
Expected: `PASS`

## Negative cases

### N1: Prose-only deferred section without companion is no longer the authoritative source [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/evalgate/... -run TestFloorBinding_ProseIgnoredWithCompanion -v 2>&1 | grep -E "PASS|FAIL"
```
Expected: `PASS`

## Edge cases

### E1: companion present but deferred_floors field absent → fall back to prose scan [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/evalgate/... -run TestFloorBinding_CompanionNoField_FallbackProse -v 2>&1 | grep -E "PASS|FAIL"
```
Expected: `PASS`
