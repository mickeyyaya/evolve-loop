# Eval: coverage-swarm-packages
<!-- challenge-token: 259a5de890f5b389 -->

## Goal
Raise coverage in `internal/phases/swarmrunner` (73.4%), `internal/phases/swarmplan` (85.7%), and `internal/swarm` (87.1%) to ≥98% each by adding intent-probing tests for uncovered paths.

## Acceptance Criteria

### AC1 — swarmrunner reaches ≥98% [code]
```
go test ./internal/phases/swarmrunner/... -coverprofile=/tmp/ev_swarmrunner.out -count=1 && go tool cover -func=/tmp/ev_swarmrunner.out | grep "^total" | awk '{if ($3+0 < 98) {print "FAIL swarmrunner coverage="$3; exit 1} else print "PASS swarmrunner coverage="$3}'
```
Expects: `PASS swarmrunner coverage=9X.X%` with value ≥ 98.0

### AC2 — swarmplan reaches ≥98% [code]
```
go test ./internal/phases/swarmplan/... -coverprofile=/tmp/ev_swarmplan.out -count=1 && go tool cover -func=/tmp/ev_swarmplan.out | grep "^total" | awk '{if ($3+0 < 98) {print "FAIL swarmplan coverage="$3; exit 1} else print "PASS swarmplan coverage="$3}'
```
Expects: exit 0 and `PASS swarmplan coverage=9X.X%`

### AC3 — swarm reaches ≥98% [code]
```
go test ./internal/swarm/... -coverprofile=/tmp/ev_swarm.out -count=1 && go tool cover -func=/tmp/ev_swarm.out | grep "^total" | awk '{if ($3+0 < 98) {print "FAIL swarm coverage="$3; exit 1} else print "PASS swarm coverage="$3}'
```
Expects: exit 0 and `PASS swarm coverage=9X.X%`

### AC4 — No test failures across all three packages [code]
```
go test ./internal/phases/swarmrunner/... ./internal/phases/swarmplan/... ./internal/swarm/... -count=1
```
Expects: exit 0, all `ok` lines (no FAIL)

### AC5 — Tests probe intent, not surface lines (negative case) [code]
```
go test ./internal/phases/swarmrunner/... -run TestSwarmRunner -v -count=1 | grep -c "=== RUN"
```
Expects: output is ≥ 5 (at least 5 distinct sub-tests exercising behavior)

### AC6 — branchByID is covered [code]
```
go test ./internal/phases/swarmrunner/... -coverprofile=/tmp/ev_branch.out -count=1 && go tool cover -func=/tmp/ev_branch.out | grep "branchByID" | awk '{if ($3 == "0.0%") {print "FAIL branchByID uncovered"; exit 1} else print "PASS branchByID="$3}'
```
Expects: `PASS branchByID=X%` (not 0.0%)

## Eval Grader Types
- AC1–AC6: `[code]` graders (automated, deterministic)

## Anti-gaming Notes
- Coverage metric is statement-level via `go tool cover -func`, not line count — inserting empty test files does not inflate coverage.
- AC5 requires ≥5 distinct test runs by name, preventing a single catch-all test that reaches multiple lines without probing intent.
