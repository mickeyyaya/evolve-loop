---
score_cap:
  - criterion: "A newly added failing test in the shipping diff blocks the ship with CodeRepoContractGate, naming the test"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 ./internal/phases/ship -run '^TestRepoContractGate_NewlyAddedFailingTestBlocksShip$'"
  - criterion: "A newly added test that is explicitly t.Skip-ped does not block the ship"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 ./internal/phases/ship -run '^TestRepoContractGate_NewlyAddedSkippedTestDoesNotBlockShip$'"
  - criterion: "Selection is bounded to newly ADDED test files — a modified test, a non-test addition, and an empty candidate set never trip the gate"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 ./internal/phases/ship -run '^TestRepoContractGate_AddedTestSelectionIgnoresModifiedAndNonTestFiles$'"
  - criterion: "A newly added failing test hidden behind a build tag is not silently reported green (incident 25040cea)"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 ./internal/phases/ship -run '^TestRepoContractGate_NewlyAddedTaggedFailingTestIsNotSilentlyGreen$'"
  - criterion: "A lone tag-guarded added test package that is green under its own tag does not false-RED the gate"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 ./internal/phases/ship -run '^TestRepoContractGate_NewlyAddedTagGuardedGreenPackageIsNotFalseRed$'"
  - criterion: "A textual mention of a build constraint is not treated as a build constraint"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 ./internal/phases/ship -run '^TestBugReproduction_AddedTestLiteralBuildConstraintIsNotExcluded$'"
  - criterion: "A failed staged-file discovery is recorded in the scan log instead of silently disabling the backstop"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./internal/phases/ship -run '^TestRepoContractGate_AddedTestDiscoveryFailureIsRecorded$'"
  - criterion: "The fixed-pack RED message still names the four guard suites, and an added-test RED identifies itself as one"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./internal/phases/ship -run '^TestRepoContractGate_RedMessagesDistinguishFixedPackFromAddedTests$'"
---

# Eval: Consume newly added red tests at the ship gate

> Pins the ship-time consumer for an already-produced signal: a shipping diff
> that adds a genuinely failing `*_test.go`. Three landings — `dcab9337`
> (cycle-1538), `25040cea` (cycle-1547) and the cycle-1550 ordering-inversion
> occurrence — pushed such a test to main because nothing at ship time asked
> "does this diff add a test that fails in the tree being shipped?". The gate
> must fail closed on a real red, stay open for an honest `t.Skip` reproducer,
> and — the half cycle-1559's audit found broken — must be correct about build
> tags in BOTH directions: a tag-guarded failure is still a failure, and a
> tag-guarded healthy package (the `//go:build acs` predicate file every cycle
> mints into its own diff) is not a build break. Source incidents: cycle-1538 /
> 1547 / 1550 red-main landings; cycle-1559 audit defects H1/H2/H3/M1/M2.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| added-red-blocks | A newly added failing test blocks ship, naming it | 9/10 | `go test -run '^TestRepoContractGate_NewlyAddedFailingTestBlocksShip$'` |
| skip-is-honest | An explicit `t.Skip` reproducer ships | 7/10 | `go test -run '^TestRepoContractGate_NewlyAddedSkippedTestDoesNotBlockShip$'` |
| bounded-selection | Only newly ADDED test files are candidates | 7/10 | `go test -run '^TestRepoContractGate_AddedTestSelectionIgnoresModifiedAndNonTestFiles$'` |
| tagged-red-not-green | H1 — a tag-guarded failure is not compiled out into a false green | 8/10 | `go test -run '^TestRepoContractGate_NewlyAddedTaggedFailingTestIsNotSilentlyGreen$'` |
| tagged-green-not-red | H2 — a tag-guarded healthy package is not a false RED | 9/10 | `go test -run '^TestRepoContractGate_NewlyAddedTagGuardedGreenPackageIsNotFalseRed$'` |
| literal-is-not-a-constraint | H3 — a string literal must not launder a real red into an exclusion | 8/10 | `go test -run '^TestBugReproduction_AddedTestLiteralBuildConstraintIsNotExcluded$'` |
| no-silent-disable | M1 — discovery failure is recorded, never invisible | 6/10 | `go test -run '^TestRepoContractGate_AddedTestDiscoveryFailureIsRecorded$'` |
| attributable-red | M2 — the operator can tell which scan went red | 6/10 | `go test -run '^TestRepoContractGate_RedMessagesDistinguishFixedPackFromAddedTests$'` |
