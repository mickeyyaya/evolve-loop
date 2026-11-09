---
score_cap:
  - criterion: "The scanner counts the extra copies of equal helper bodies and the ratchet fails on a fixture tree with a new copy"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run 'TestScan_CountsTheExtraCopiesOfEqualHelperBodies|TestRatchet_FailsOnAFixtureTreeWithANewCopyOfAHelperBody|TestCheck_FailsWhenTheCopiesAreAboveTheBaseline' ./internal/testdup/"
  - criterion: "The repocontract structure test passes on the real tree at the count in go/.test-dup-baseline"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestTestDuplicates_TheCountOfCopiesEqualsTheBaseline' ./internal/repocontract/"
  - criterion: "A count below the baseline fails, so the baseline goes down in the same commit as the replaced copies"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestCheck_FailsWhenTheBaselineIsAboveTheCountSoALoweredCountLowersIt|TestRatchet_PassesAgainAfterTheNewCopyIsRemoved' ./internal/testdup/"
  - criterion: "An acs cycle directory older than the baseline date is not scanned, and its age comes from its commit"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestScan_SkipsAnAcsCycleDirectoryOlderThanTheBaselineDate|TestScan_ScansAnAcsCycleDirectoryNewerThanTheBaselineDate|TestScan_DatesAnAcsCycleDirectoryByItsCommitNotByTheMtimeOfAFreshCheckout' ./internal/testdup/"
---

# Eval: A duplicate ratchet for test helpers (test-system plan T5)

> Pins the ratchet of docs/architecture/test-system.md section 6.4 (decision D12). A scanner counts the extra copies of equal non-test function bodies in the runtime test files. `go/.test-dup-baseline` holds the count and the date. A structure test in `internal/repocontract` fails when the count differs from the baseline. Source: cycle 2231, inbox item test-system-efficiency-t5-duplicate-ratchet.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| fixture-red | A new copy in a fixture tree fails the ratchet | 4/10 | `go test -run TestRatchet_FailsOnAFixtureTreeWithANewCopyOfAHelperBody ./internal/testdup/` |
| real-tree-green | The real tree passes at the baseline count | 5/10 | `go test -run TestTestDuplicates_TheCountOfCopiesEqualsTheBaseline ./internal/repocontract/` |
| lower-together | A lowered count forces a lowered baseline | 6/10 | `go test -run TestCheck_FailsWhenTheBaselineIsAboveTheCount... ./internal/testdup/` |
| acs-history | Old acs directories are history | 6/10 | `go test -run TestScan_SkipsAnAcsCycleDirectoryOlderThanTheBaselineDate ./internal/testdup/` |
