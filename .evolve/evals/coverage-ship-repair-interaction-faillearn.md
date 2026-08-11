# Eval: coverage-ship-repair-interaction-faillearn
<!-- challenge-token: 259a5de890f5b389 -->

## Goal
Raise coverage in `internal/phases/ship` (91.2%), `internal/interaction` (90.5%), `internal/faillearn` (91.0%), and `internal/evalgate` (91.2%) to ≥97% each by adding intent-probing tests for the ship repair ladder, interaction ledger, faillearn rendering, and evalgate reviewer paths.

## Acceptance Criteria

### AC1 — phases/ship reaches ≥97% [code]
```
go test ./internal/phases/ship/... -coverprofile=/tmp/ev_ship.out -count=1 -short && go tool cover -func=/tmp/ev_ship.out | grep "^total" | awk '{if ($3+0 < 97) {print "FAIL ship coverage="$3; exit 1} else print "PASS ship coverage="$3}'
```
Expects: exit 0 and `PASS ship coverage=9X.X%` (≥97%)

Note: `phases/ship` target is 97% (not 98%) — the final 1% is integration-gated (real git push paths).

### AC2 — interaction reaches ≥98% [code]
```
go test ./internal/interaction/... -coverprofile=/tmp/ev_interaction.out -count=1 && go tool cover -func=/tmp/ev_interaction.out | grep "^total" | awk '{if ($3+0 < 98) {print "FAIL interaction coverage="$3; exit 1} else print "PASS interaction coverage="$3}'
```
Expects: exit 0 and `PASS interaction coverage=9X.X%`

### AC3 — faillearn reaches ≥98% [code]
```
go test ./internal/faillearn/... -coverprofile=/tmp/ev_faillearn.out -count=1 && go tool cover -func=/tmp/ev_faillearn.out | grep "^total" | awk '{if ($3+0 < 98) {print "FAIL faillearn coverage="$3; exit 1} else print "PASS faillearn coverage="$3}'
```
Expects: exit 0 and `PASS faillearn coverage=9X.X%`

### AC4 — evalgate reaches ≥98% [code]
```
go test ./internal/evalgate/... -coverprofile=/tmp/ev_evalgate.out -count=1 && go tool cover -func=/tmp/ev_evalgate.out | grep "^total" | awk '{if ($3+0 < 98) {print "FAIL evalgate coverage="$3; exit 1} else print "PASS evalgate coverage="$3}'
```
Expects: exit 0 and `PASS evalgate coverage=9X.X%`

### AC5 — ship repair functions covered (negative case: collider behavior) [code]
```
go test ./internal/phases/ship/... -coverprofile=/tmp/ev_repair.out -count=1 -short && go tool cover -func=/tmp/ev_repair.out | grep "repairColliders" | awk '{if ($3+0 < 70) {print "FAIL repairColliders only="$3; exit 1} else print "PASS repairColliders="$3}'
```
Expects: `PASS repairColliders=XX.X%` (≥70%)

### AC6 — writeStateMap in ship/statefile is covered [code]
```
go test ./internal/phases/ship/... -coverprofile=/tmp/ev_statefile.out -count=1 -short && go tool cover -func=/tmp/ev_statefile.out | grep "writeStateMap" | awk '{if ($3+0 < 85) {print "FAIL writeStateMap only="$3; exit 1} else print "PASS writeStateMap="$3}'
```
Expects: `PASS writeStateMap=XX.X%` (≥85%)

### AC7 — All packages pass their test suites [code]
```
go test ./internal/phases/ship/... ./internal/interaction/... ./internal/faillearn/... ./internal/evalgate/... -count=1 -short
```
Expects: exit 0, all `ok` lines

## Eval Grader Types
- AC1–AC7: `[code]` graders

## Anti-gaming Notes
- `addRepairSignals` at 33.3% should be tested with a seam that exercises the repair-signal enrichment (a repaired state with typed repair markers must produce the right Signals entries — not just hit the early-return branch).
- `structuredDefects` in faillearn at 66.7% must be tested with a defects list that exercises both the "has defects" and "empty defects" branches — the latter is the negative case.
- `evalgate.quality.name` at 0.0% (a method) must be called from a test — even a constructor test will trigger it.
