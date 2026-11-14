---
score_cap:
  - criterion: "A FIFO test file does not hang the tdd-build request"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC2390_001 ./acs/cycle2390"
  - criterion: "A symlinked test file adds none of its bytes to the build prompt"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run TestPlainFiles_SymlinkedTestFile ./internal/core"
  - criterion: "A ledger larger than the cap is refused"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2390_003 ./acs/cycle2390"
  - criterion: "The tdd-build request opens each test file once through the shared plain-file reader, so a swap after the check is refused"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run TestReadWith_AFileSwappedAfterTheCheckIsRefused ./internal/plainfile && go test -tags acs -count=1 -run TestC2390_007 ./acs/cycle2390"
  - criterion: "The hard-link test has one home"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC2390_009 ./acs/cycle2390"
---

# Eval: the tdd-build request reads plain files only

> Pins that the tdd-build handoff request lists predicates only from plain test files, and that the ledger read refuses a file above a size cap. A FIFO blocks the read, a symlink or hard link reads bytes from outside the package, and an unbounded ledger read allocates the whole file. Audit repair of cycle 2368: the first build checked an entry with Lstat and let the parser reopen it by path (M1), and copied the hard-link test of inputindex.HardLinked (M2). Source: cycle 2368 inbox item handoff-request-reads-plain-files-only.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| fifo | FIFO test file returns | 7/10 | `go test -tags acs -run TestC2390_001 ./acs/cycle2390` |
| symlink | symlink adds no bytes | 7/10 | `go test -run TestPlainFiles_SymlinkedTestFile ./internal/core` |
| cap | oversize ledger refused | 6/10 | `go test -tags acs -run TestC2390_003 ./acs/cycle2390` |
| swap | file swapped after the check is refused | 6/10 | `go test -run TestReadWith_AFileSwappedAfterTheCheckIsRefused ./internal/plainfile` |
| single-home | one hard-link test | 5/10 | `go test -tags acs -run TestC2390_009 ./acs/cycle2390` |
