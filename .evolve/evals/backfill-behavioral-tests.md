# Eval: backfill-behavioral-tests

## Objective

Add meaningful behavioral tests to `go/internal/backfill/backfill_test.go` covering three
uncovered behavioral axes:

1. **Last-occurrence semantics**: when stdout.clean.txt contains the header more than
   once, `TryExtract` must extract from the LAST occurrence (verified by `strings.LastIndex`
   in the implementation but never exercised with two real header occurrences in existing tests).

2. **minLen boundary**: content whose length is exactly `minLen` must be accepted
   (`extracted=true`); content of length `minLen-1` must be rejected (`extracted=false, err==nil`).

3. **Remaining phase headers**: phases `build`, `audit`, `tdd`, `intent`, `triage` appear in
   `phaseHeaders` but are never positively exercised by existing tests.

## Scope

- **Target file**: `go/internal/backfill/backfill_test.go` (extend only, no new files)
- **No production file changes**: `backfill.go`, `atomicwrite`, or any other `.go` source
  outside `*_test.go` must remain unmodified.
- **No new files**: append tests to the existing test file only.

## Acceptance Criteria

```bash
# [code] Suite must pass with all new tests included
cd go && go test ./internal/backfill/... -v -run 'TestTryExtract' 2>&1 | grep -E 'PASS|FAIL'
```

Expected: every `--- PASS:` line visible, no `--- FAIL:` lines.

```bash
# [code] New test TestTryExtract_MultipleHeaders_LastWins must exist and pass
cd go && go test ./internal/backfill/... -run TestTryExtract_MultipleHeaders_LastWins -v 2>&1 | grep -E 'PASS|FAIL|ok'
```

Expected output must include `PASS`.

```bash
# [code] New test TestTryExtract_MinLen_Boundary must exist and pass
cd go && go test ./internal/backfill/... -run TestTryExtract_MinLen_Boundary -v 2>&1 | grep -E 'PASS|FAIL|ok'
```

Expected output must include `PASS`.

```bash
# [code] New test TestTryExtract_KnownPhases_BuildAuditTDDIntentTriage must exist and pass
cd go && go test ./internal/backfill/... -run TestTryExtract_KnownPhases_BuildAuditTDDIntentTriage -v 2>&1 | grep -E 'PASS|FAIL|ok'
```

Expected output must include `PASS`.

```bash
# [code] No production source files modified
cd go && git diff --name-only HEAD -- internal/backfill/ | grep -v '_test\.go' | grep '\.go$'
```

Expected: empty output (no production .go files changed).

```bash
# [code] Full suite still green
cd go && go test ./internal/backfill/... 2>&1 | tail -1
```

Expected: line matches `^ok\s+.*backfill`.
