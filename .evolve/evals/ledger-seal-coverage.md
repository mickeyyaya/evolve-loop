# Eval: ledger-seal-coverage

## Goal
Raise `internal/adapters/ledger` coverage from 82.5% to ≥ 90% by covering the lowest-covered `seal.go` functions: `writeSegment` (50%), `rewriteLive` (52.2%), `linesEqual` (66.7%), `readSegment` (71.4%), `anchorFor` (75%), and `sealLocked` (76.5%).

## Acceptance Criteria

### AC1 — writeSegment round-trip [code]
```bash
cd go && go test ./internal/adapters/ledger/... -run "TestWriteSegment" -v -count=1
```
Expected: `--- PASS: TestWriteSegment` (write gzipped segment, read it back, assert bytes match)

### AC2 — rewriteLive atomically replaces ledger content [code]
```bash
cd go && go test ./internal/adapters/ledger/... -run "TestRewriteLive" -v -count=1
```
Expected: `--- PASS: TestRewriteLive` (temp file replaced, original not left behind, content matches)

### AC3 — linesEqual true and false paths [code]
```bash
cd go && go test ./internal/adapters/ledger/... -run "TestLinesEqual" -v -count=1
```
Expected: `--- PASS: TestLinesEqual` (same slices → true; different length → false; same length different content → false)

### AC4 — readSegment round-trip [code]
```bash
cd go && go test ./internal/adapters/ledger/... -run "TestReadSegment" -v -count=1
```
Expected: `--- PASS: TestReadSegment` (write segment with writeSegment, read back with readSegment, lines + SHA match)

### AC5 — sealLocked resume case A (residue segment) [code]
```bash
cd go && go test ./internal/adapters/ledger/... -run "TestSeal_ResumeResidueSegment|TestSeal_Resume" -v -count=1
```
Expected: at least 1 PASS (seal is idempotent when called twice — second call detects the residue segment and resumes correctly)

### AC6 — Coverage ≥ 90% [code]
```bash
cd go && go test ./internal/adapters/ledger/... -count=1 -coverprofile=/tmp/ledger301.out && go tool cover -func=/tmp/ledger301.out | grep "^total:" | awk '{print $3}' | awk -F'%' '{exit ($1 < 90)}'
```
Expected: exit 0 (coverage ≥ 90%)

### AC7 — Existing ledger tests remain green [code]
```bash
cd go && go test ./internal/adapters/ledger/... -count=1 2>&1 | tail -1 | grep -q "^ok"
```
Expected: exit 0 (full package passes)

## Negative Cases

### NC1 — linesEqual rejects length mismatch [code]
```bash
cd go && go test ./internal/adapters/ledger/... -run "TestLinesEqual_LengthMismatch" -v -count=1
```
Expected: `--- PASS` (slices of different lengths → false, not panic)

### NC2 — readSegment fails on corrupt gzip [code]
```bash
cd go && go test ./internal/adapters/ledger/... -run "TestReadSegment_Corrupt" -v -count=1
```
Expected: `--- PASS` (non-zero error returned, not a panic)

## Grader Notes
- `[code]` graders run from repo root; `cd go` changes to the Go module dir.
- AC6 awk exits 0 when coverage passes — correct passing behavior.
- These tests exercise seal internals via unexported functions (same package `package ledger`); test files use `package ledger` (not `_test`).
