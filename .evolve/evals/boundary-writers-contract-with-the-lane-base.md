---
score_cap:
  - criterion: "A BoundaryWriters contract test exists in cmd/evolve and passes on the real tree"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -v -run BoundaryWriters ./cmd/evolve | grep -q -- '--- PASS: .*BoundaryWriters'"
  - criterion: "Each boundary home is bound to the core lane-base list by the contract test"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2325_00[234]' ./acs/cycle2325"
  - criterion: "A publisher home with no core list entry fails the contract test"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2325_00[56]' ./acs/cycle2325"
---

# Eval: contract test ties each boundary writer home to the lane-base prefixes

> Pins `core.boundaryOwnedPrefixes` against the homes that `runLanesThenPublish` commits to
> (`dossier.Home`, `inboxstamps.Pathspec`, `recoveryretro.DocsHome`). Source incident: wave 166
> deferred all 16 lanes because the retrospective home was missing from the list (cycle 2325 inbox item).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| contract-exists | the contract test runs and passes | 5/10 | `go test -run BoundaryWriters ./cmd/evolve` |
| drop-a-home | dropping each home fails the test | 6/10 | `TestC2325_002..004` |
| new-publisher | an unlisted publisher home fails the test | 6/10 | `TestC2325_005..006` |
