# Eval: gc-coverage-boost

## Task
Boost `internal/gc` test coverage from 88.8% to ≥95% by covering uncovered paths
in `Apply`, `nowLive`, `protected`, and `dirEntriesOlderThan`.

## Acceptance Criteria

### AC1 — Coverage threshold met [code]
```bash
cd go && go test -coverprofile=/tmp/gc_cover_new.out ./internal/gc/... 2>&1
go tool cover -func=/tmp/gc_cover_new.out | grep "^total:" | awk '{print $3}'
```
Expected: output ≥ `95.0%`
Grader: `[code]` — numeric threshold check

### AC2 — Apply: protected path refused [code]
```bash
cd go && go test -run TestApply_ProtectedRefused ./internal/gc/ -v 2>&1 | grep -c "PASS"
```
Expected: `1`
Grader: `[code]` — test pass

### AC3 — Apply: becomes-live-after-plan refused [code]
```bash
cd go && go test -run TestApply_BecameLive ./internal/gc/ -v 2>&1 | grep -c "PASS"
```
Expected: `1`
Grader: `[code]` — test pass

### AC4 — nowLive: fresh .lease on parent marks live [code]
```bash
cd go && go test -run TestNowLive_ParentLease ./internal/gc/ -v 2>&1 | grep -c "PASS"
```
Expected: `1`
Grader: `[code]` — test pass

### AC5 — All tests pass (no regression) [code]
```bash
cd go && go test ./internal/gc/... 2>&1 | grep -c "^ok"
```
Expected: `1`
Grader: `[code]` — exit-code check

## Negative Cases

### N1 — Apply does NOT proceed on unreadable run-state [code]
```bash
cd go && go test -run TestNowLive_UnreadableRunState ./internal/gc/ -v 2>&1 | grep -c "PASS"
```
Expected: `1` (fail-closed: returns live=true on state read error)
Grader: `[code]`
Fake that passes: an impl that returns live=false on error would fail.

### N2 — dirEntriesOlderThan skips missing dirs without error [code]
```bash
cd go && go test -run TestDirEntriesOlderThan_MissingDir ./internal/gc/ -v 2>&1 | grep -c "PASS"
```
Expected: `1`
Grader: `[code]`

## Edge Cases

### E1 — Apply with empty manifest is a no-op [code]
```bash
cd go && go test -run TestApply_EmptyManifest ./internal/gc/ -v 2>&1 | grep -c "PASS"
```
Expected: `1`
Grader: `[code]`

### E2 — Apply: archive dst collision disambiguates with numeric suffix [code]
```bash
cd go && go test -run TestApply_ArchiveCollision ./internal/gc/ -v 2>&1 | grep -c "PASS"
```
Expected: `1`
Grader: `[code]`
