# Eval: evalgate-floorbinding-workspace-coverage

## Task
Add unit tests for uncovered branches in `go/internal/evalgate`:
1. `cycleNumFromWorkspace` with a non-matching workspace path → returns 0
2. `NewReviewer` logf closure body exercised by a real violation (currently 50% — logf body never called)

## Acceptance Criteria

### AC1: `cycleNumFromWorkspace` non-matching path returns 0
```bash
cd go && go test -v -run TestCycleNumFromWorkspace_NonMatchingPath ./internal/evalgate/...
```
[code] must exit 0 and print `PASS`

### AC2: `NewReviewer` logf body exercised via real violation
```bash
cd go && go test -v -run TestNewReviewer_LogfBodyExecutedOnViolation ./internal/evalgate/...
```
[code] must exit 0 and print `PASS`

### AC3: `NewReviewer` coverage improves
```bash
cd go && go test -coverprofile=/tmp/cover_eg.out ./internal/evalgate/... && go tool cover -func=/tmp/cover_eg.out | grep "reviewer.go"
```
[code] `NewReviewer` coverage must be ≥ 80% (up from 50%)

### AC4: `cycleNumFromWorkspace` coverage improves
```bash
cd go && go test -coverprofile=/tmp/cover_eg.out ./internal/evalgate/... && go tool cover -func=/tmp/cover_eg.out | grep "cycleNumFromWorkspace"
```
[code] coverage must be ≥ 85% (up from 71.4%)

### AC5 (negative): `cycleNumFromWorkspace` with "cycle-abc" (non-numeric) returns 0
```bash
cd go && go test -v -run TestCycleNumFromWorkspace_NonNumericSuffix ./internal/evalgate/...
```
[code] must exit 0 and print `PASS` — the regex requires digits, so this path returns 0 (regex nil-match branch)

### AC6 (edge): `cycleNumFromWorkspace` with valid workspace returns correct cycle number
```bash
cd go && go test -v -run TestCycleNumFromWorkspace_ValidPath ./internal/evalgate/...
```
[code] must exit 0 and print `PASS` — regression guard for the happy path

### AC7: full evalgate suite remains green
```bash
cd go && go test ./internal/evalgate/...
```
[code] must exit 0
