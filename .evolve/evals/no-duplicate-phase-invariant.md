# Eval: no-duplicate-phase-invariant

## Task
Add `no-duplicate-phase` to `invariantChecks` in `go/internal/routingtest/invariants.go`; rename `TestInvariant_DuplicatePhaseRejected` → `TestInvariant_DuplicatePhaseTolerated`; add `TestInvariant_NoDuplicatePhaseEnforcesUniqueness` with both positive and negative sub-cases.

## Acceptance Criteria

### C1: no-duplicate-phase invariant fires on duplicate plan entries [code]
```bash
cd go && go test ./internal/routingtest/... -v -run TestInvariant_NoDuplicatePhaseEnforcesUniqueness 2>&1
```
Expected: `--- PASS: TestInvariant_NoDuplicatePhaseEnforcesUniqueness` in output; exit 0.

### C2: Invariant map contains no-duplicate-phase key [code]
```bash
grep -c '"no-duplicate-phase"' go/internal/routingtest/invariants.go
```
Expected: output `1` (exactly one entry in the map).

### C3: Renamed test passes [code]
```bash
cd go && go test ./internal/routingtest/... -v -run TestInvariant_DuplicatePhaseTolerated 2>&1
```
Expected: `--- PASS: TestInvariant_DuplicatePhaseTolerated` in output; exit 0.

### C4: Old test name no longer exists [code]
```bash
grep -c 'TestInvariant_DuplicatePhaseRejected' go/internal/routingtest/invariants_test.go
```
Expected: output `0` (name removed).

### C5: Coverage maintained ≥80% [code]
```bash
cd go && go test ./internal/routingtest/... -cover 2>&1 | grep -o 'coverage: [0-9.]*%'
```
Expected: percentage value ≥80.0%.

### C6: No regression in full suite [code]
```bash
cd go && go test ./internal/routingtest/... 2>&1 | tail -3
```
Expected: `ok  github.com/mickeyyaya/evolve-loop/go/internal/routingtest` with no `--- FAIL` lines.

### C7: Negative case — invariant reports error for duplicate phases [code]
```bash
cd go && go test ./internal/routingtest/... -v -run TestInvariant_NoDuplicatePhaseEnforcesUniqueness/negative 2>&1
```
Expected: exit 0 (the test itself orchestrates the expected-failure); `--- PASS` line present. Gaming fake: a trivially-named invariant function that never calls `t.Errorf` would satisfy C2 but fail this check because the negative sub-case drives a scenario with duplicate phases and validates the error path.
