---
score_cap:
  - criterion: "a lane that adds a second verdict.New( call outside its package fails at the build floor, not on main"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1829_003_' ./acs/cycle1829"
  - criterion: "readsTheTreeOutsideThePack holds none of the 13 packages and the pack runs a test of each"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1829_004_' ./acs/cycle1829"
  - criterion: "the slow packages run by test name and the pack finishes green inside the build floor's 120s deadline"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1829_005_' ./acs/cycle1829"
---

# Eval: the pack runs the tree-reading seam tests by name before main

> TestPackages_HoldEveryTestThatReadsTheWholeTree recorded 13 packages (cmd/evolve, bridge, changedpkgs, core, cycleoutcome, inboxmover, inboxmover/lifecycle, phaseobserver, phases/audit, phases/runner, phases/ship, reachabilityprobe, subagent) outside the fixed scanner pack, so a lane touching neither the package nor an importer could break a seam or single-writer test such as TestVerdictEngine_OneConstructionSite and only main's CI found out. Pins that the build floor's production pack runs those tests by name in the lane's own tree: a lane tree with a second verdict.New( site reds the floor naming the real seam test, every one of the 13 packages has a test in the pack, cmd/evolve and core run some but never all of their tests, and the pack finishes green on the lane's tree inside the floor's deadline. Source incident: inbox 2026-09-30 architecture review; cycle 1829 RED run.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| seam-red-first | second verdict.New( site reds the build floor | 8/10 | `go test -tags acs -run '^TestC1829_003_' ./acs/cycle1829` |
| table-emptied | no package waits outside the pack; each of the 13 is run | 7/10 | `go test -tags acs -run '^TestC1829_004_' ./acs/cycle1829` |
| by-name-in-deadline | slow packages by name; live pack green inside 120s | 6/10 | `go test -tags acs -run '^TestC1829_005_' ./acs/cycle1829` |
