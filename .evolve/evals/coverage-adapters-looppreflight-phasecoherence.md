# Eval: coverage-adapters-looppreflight-phasecoherence
<!-- challenge-token: 259a5de890f5b389 -->

## Goal
Raise coverage in `internal/adapters/bridge` (82.0%), `internal/looppreflight` (87.6%), `internal/phasecoherence` (83.0%), and `internal/modelcatalog` (87.3%) to ≥98% each by covering the key uncovered paths.

## Acceptance Criteria

### AC1 — adapters/bridge reaches ≥98% [code]
```
go test ./internal/adapters/bridge/... -coverprofile=/tmp/ev_adpbridge.out -count=1 && go tool cover -func=/tmp/ev_adpbridge.out | grep "^total" | awk '{if ($3+0 < 98) {print "FAIL adapters/bridge coverage="$3; exit 1} else print "PASS adapters/bridge coverage="$3}'
```
Expects: exit 0 and `PASS adapters/bridge coverage=9X.X%`

### AC2 — looppreflight reaches ≥98% [code]
```
go test ./internal/looppreflight/... -coverprofile=/tmp/ev_looppreflight.out -count=1 && go tool cover -func=/tmp/ev_looppreflight.out | grep "^total" | awk '{if ($3+0 < 98) {print "FAIL looppreflight coverage="$3; exit 1} else print "PASS looppreflight coverage="$3}'
```
Expects: exit 0 and `PASS looppreflight coverage=9X.X%`

### AC3 — phasecoherence reaches ≥98% [code]
```
go test ./internal/phasecoherence/... -coverprofile=/tmp/ev_phasecoherence.out -count=1 && go tool cover -func=/tmp/ev_phasecoherence.out | grep "^total" | awk '{if ($3+0 < 98) {print "FAIL phasecoherence coverage="$3; exit 1} else print "PASS phasecoherence coverage="$3}'
```
Expects: exit 0 and `PASS phasecoherence coverage=9X.X%`

### AC4 — modelcatalog reaches ≥98% [code]
```
go test ./internal/modelcatalog/... -coverprofile=/tmp/ev_modelcatalog.out -count=1 && go tool cover -func=/tmp/ev_modelcatalog.out | grep "^total" | awk '{if ($3+0 < 98) {print "FAIL modelcatalog coverage="$3; exit 1} else print "PASS modelcatalog coverage="$3}'
```
Expects: exit 0 and `PASS modelcatalog coverage=9X.X%`

### AC5 — SetOnStopReview is covered [code]
```
go test ./internal/adapters/bridge/... -coverprofile=/tmp/ev_sor.out -count=1 && go tool cover -func=/tmp/ev_sor.out | grep "SetOnStopReview" | awk '{if ($3 == "0.0%") {print "FAIL SetOnStopReview uncovered"; exit 1} else print "PASS SetOnStopReview="$3}'
```
Expects: `PASS SetOnStopReview=X%` (not 0.0%)

### AC6 — canonicalRole in phasecoherence is covered [code]
```
go test ./internal/phasecoherence/... -coverprofile=/tmp/ev_canon.out -count=1 && go tool cover -func=/tmp/ev_canon.out | grep "canonicalRole" | awk '{if ($3+0 < 80) {print "FAIL canonicalRole only="$3; exit 1} else print "PASS canonicalRole="$3}'
```
Expects: `PASS canonicalRole=XX.X%` (≥80%)

### AC7 — All four packages pass their test suites [code]
```
go test ./internal/adapters/bridge/... ./internal/looppreflight/... ./internal/phasecoherence/... ./internal/modelcatalog/... -count=1
```
Expects: exit 0, all `ok` lines

## Eval Grader Types
- AC1–AC7: `[code]` graders

## Anti-gaming Notes
- `newDefaultBootTester` at 6.7% must be brought above 80% (it's a factory function for the real boot test — testing only its zero-value path would score 100% falsely; test it with a seam-injected stub that verifies the real factory wires the expected check names).
- `canonicalRole` at 28.6% is tested via negative case: a role name that is not in the canonical set must return the `unknown` sentinel, not panic.
