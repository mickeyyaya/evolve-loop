---
score_cap:
  - criterion: "The agent run bridge puts the evolve binary directory first on the PATH that the seat pane inherits"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestAgentRunBridge_EvolveBinaryDirOnPath$' ./cmd/evolve"
  - criterion: "An empty host PATH yields no empty PATH entry, and repeated bridge construction adds the directory once"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestAgentRunBridge_EvolveBinaryDirOnPath(LeavesNoEmptyEntry|IsAddedOnce)$' ./cmd/evolve"
  - criterion: "The PATH at the moment a seat launches through evolve agent run holds the evolve binary directory first"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^TestAgentRunBridge_EvolveBinaryDirOnPathWhenTheSeatLaunches$' ./cmd/evolve"
  - criterion: "The cycle predicate drives the four launch environment tests and demands that each one ran and passed"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2498_001' ./acs/cycle2498"
---

# Eval: Review seats get the evolve binary on their pane PATH

> Pins the launch environment of evolve agent run. Source incident: 27 review reports (cycle 2291 routing) say
> that the evolve binary is not on the PATH of the seat pane, so the reviewers skip evolve code and
> evolve agent capabilities. The pane inherits os.Environ() through the hermetic environment, whose pipeline
> overlay copies only cell markers, so the fix prefixes the process PATH in agentRunBridge.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| prefix | Evolve binary directory first, host PATH kept | 4/10 | `TestAgentRunBridge_EvolveBinaryDirOnPath` |
| no-empty-entry, once | No empty entry; no duplicate directory | 6/10 | `LeavesNoEmptyEntry`, `IsAddedOnce` |
| launch-time | PATH at seat launch through agentcmd | 5/10 | `WhenTheSeatLaunches` |
| predicate | Cycle predicate runs all four | 5/10 | `TestC2498_001` |
