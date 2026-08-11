# Eval: evalgate-error-branch-coverage

## Task
Add unit tests for uncovered branches in `go/internal/evalgate`:
1. `cycleNumFromWorkspace` with a non-matching (non-`cycle-<N>`) workspace path → returns 0
2. `fencedAfterHeading` when the opening fence line has no trailing newline → returns `("", false)`
3. `NewReviewer` logf closure body exercised by a review call that triggers a violation (currently 50%)

## Acceptance Criteria

### AC1: `cycleNumFromWorkspace` non-matching path returns 0
```bash
cd go && go test -v -run TestCycleNumFromWorkspace_NonMatchingPath ./internal/evalgate/...
```
[code] must exit 0 and print `PASS`

### AC2: `fencedAfterHeading` fence-without-newline returns empty+false
```bash
cd go && go test -v -run TestFencedAfterHeading_FenceWithoutTrailingNewline ./internal/evalgate/...
```
[code] must exit 0 and print `PASS`

### AC3: `NewReviewer` logf body exercised (≥80% coverage)
```bash
cd go && go test -coverprofile=/tmp/cover_eg_332.out ./internal/evalgate/... && go tool cover -func=/tmp/cover_eg_332.out | grep "reviewer.go"
```
[code] `NewReviewer` line must report ≥ 80.0%

### AC4: overall evalgate coverage improves
```bash
cd go && go test -coverprofile=/tmp/cover_eg_332.out ./internal/evalgate/... && go tool cover -func=/tmp/cover_eg_332.out | grep "^total"
```
[code] total must report ≥ 93.0%

### AC5 (negative): `cycleNumFromWorkspace` with "workspace" (no cycle prefix) returns 0
```bash
cd go && go test -v -run TestCycleNumFromWorkspace_PlainDir ./internal/evalgate/...
```
[code] must exit 0 and print `PASS`
