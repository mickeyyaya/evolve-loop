---
score_cap:
  - criterion: "The triage-tdd request for the cycle 1946 fixture lists its two acceptance entries, each with its source and verbatim text"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1974_001_TheTriageTddRequestForCycle1946ListsItsTwoAcceptanceEntriesWithSourceAndVerbatimText ./acs/cycle1974"
  - criterion: "The triage-tdd builder refuses a request it cannot build whole: no committed set, or a committed item with no readable inbox record"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1974_002_TheTriageTddBuilderRefusesARequestItCannotBuildWhole ./acs/cycle1974"
  - criterion: "The tdd-build request lists each predicate of the cycle 1950 fixture package"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1974_003_TheTddBuildRequestListsEachPredicateOfTheCycle1950Package ./acs/cycle1974"
  - criterion: "The scout-triage request asks once per selected slug"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1974_004_TheScoutTriageRequestAsksOncePerSelectedSlug ./acs/cycle1974"
  - criterion: "go/internal/handoff/request does not import go/internal/core, directly or through its dependencies"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1974_005_ThePackageDoesNotImportCoreDirectlyOrThroughItsDependencies ./acs/cycle1974"
  - criterion: "The package note lists every exported builder of internal/handoff/request and still links the spec"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1974_006_ThePackageNoteListsEveryBuilderAndStillLinksTheSpec ./acs/cycle1974"
  - criterion: "./internal/handoff/request is at 100 in go/.cover-strict, enrolled in .apicover-enforce, and passes apicover -enforce"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1974_007_TheRequestPackageIsAtOneHundredAndEnrolledInBothGates ./acs/cycle1974"
  - criterion: "The request package carries no comments"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC1974_008_TheRequestPackageCarriesNoComments ./acs/cycle1974"
  - criterion: "The fixtures hold no Go file the build floor can run, and the cycle 1950 fixture still lists its predicates"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1974_009_TheFixturesHideNoGoFileFromTheBuildFloor ./acs/cycle1974"
---

# Eval: Phase handoff H2, the request builders for each pair source

> Pins the second component of ADR-0141: the package `go/internal/handoff/request` builds the `handoff.Request` of the pairs scout-triage, triage-tdd and tdd-build, each from one source. The triage-tdd builder reads the committed set (`committedset`) and the inbox acceptance (`inboxbatch`), and keeps each acceptance entry as one ask with its source and its text. The tdd-build builder reads the predicates of an ACS cycle package with `go/parser`. The package must not reach `go/internal/core`. The fixtures are the inbox item of cycle 1946 and the predicates of cycle 1950. Source: cycle 1974, spec `docs/architecture/phase-handoff-contract.md` section 3.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| triage-tdd-fixture | the two acceptance entries of cycle 1946, with source and verbatim text | 8/10 | `TestC1974_001` |
| triage-tdd-refusal | no silent partial request | 7/10 | `TestC1974_002` |
| tdd-build-fixture | each predicate of cycle 1950 | 8/10 | `TestC1974_003` |
| scout-triage | one ask per selected slug | 6/10 | `TestC1974_004` |
| core-boundary | no import of core, direct or transitive | 8/10 | `TestC1974_005` |
| package-note | the note lists every builder | 6/10 | `TestC1974_006` |
| gates | cover-strict 100, apicover-enforce enrollment | 7/10 | `TestC1974_007` |
| comment-zero | no comment in the package | 5/10 | `TestC1974_008` |
| fixture-isolation | no `.go` file under testdata, cycle 1950 fixture stored as `.go.fixture` | 7/10 | `TestC1974_009` |
