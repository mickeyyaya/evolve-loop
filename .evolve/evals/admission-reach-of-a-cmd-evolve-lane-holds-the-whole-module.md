---
score_cap:
  - criterion: "A running lane that edits only a leaf package (go/cmd/evolve) holds no Go todo in another package, in either direction"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1972_001' ./acs/cycle1972"
  - criterion: "A todo that edits a leaf and a non-leaf package keeps the full reach of the non-leaf package"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1972_002' ./acs/cycle1972"
  - criterion: "A non-leaf lane still holds a candidate in a package of its reach"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1972_003' ./acs/cycle1972"
  - criterion: "Two todos that edit go/cmd/evolve never run at once and the hub rule is unchanged"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1972_004' ./acs/cycle1972"
  - criterion: "ModuleGraph.IsLeaf is true for go/cmd/evolve and false for go/internal/router on the fixture"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1972_005' ./acs/cycle1972"
  - criterion: "A mixed cmd/evolve+router todo keeps the reach of cmd/evolve: it holds a clihealth-only candidate, and a clihealth lane holds it"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1972_007' ./acs/cycle1972"
  - criterion: "The wave-orchestrator plan and spec record approach (b), the rejected options and the wave 131 evidence"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1972_009' ./acs/cycle1972"
---

# Eval: admission reach of a leaf lane

> Pins operator approach (b) of 2026-10-19: a todo whose edited packages are all leaves of the module graph (zero importers, such as go/cmd/evolve) has an empty reach, so it holds no other Go todo out of the pool. Source incident: wave 131, where the pool fell to 3 of 4 lanes while 18 items were lane-ready, because cmd/evolve reaches 266 internal packages through go list Deps.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| leaf-no-reach | a cmd/evolve lane admits a router todo | 4/10 | `go test -tags acs -run TestC1972_001 ./acs/cycle1972` |
| mixed-keeps-reach | leaf plus non-leaf keeps the non-leaf reach | 5/10 | `go test -tags acs -run TestC1972_002 ./acs/cycle1972` |
| non-leaf-holds | a non-leaf lane still holds its reach | 5/10 | `go test -tags acs -run TestC1972_003 ./acs/cycle1972` |
| same-cell-and-hub | same package and hub rules unchanged | 5/10 | `go test -tags acs -run TestC1972_004 ./acs/cycle1972` |
| is-leaf | IsLeaf on the fixture | 6/10 | `go test -tags acs -run TestC1972_005 ./acs/cycle1972` |
| mixed-full-reach | a mixed todo holds the cmd/evolve reach too | 4/10 | `go test -tags acs -run TestC1972_007 ./acs/cycle1972` |
| design-recorded | plan and spec record approach (b) | 6/10 | `go test -tags acs -run TestC1972_009 ./acs/cycle1972` |
