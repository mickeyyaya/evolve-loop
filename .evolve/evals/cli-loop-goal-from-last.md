---
score_cap:
  - criterion: "A launch with --goal-text T records T atomically in .evolve/last-goal.txt, and --goal-from-last --dry-run resolves the same goal hash"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestLoopGoalSource_(LaunchRecordsGoalTextThatGoalFromLastReplays|LaunchReplacesTheRecordAtomically|DryRunRecordsNothing)$' ./cmd/evolve"
  - criterion: "Two goal sources together exit 10, and an unusable --goal-file exits 10"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestLoopGoalSource_(TwoGoalSourcesExit10|UnreadableGoalFileExits10)$' ./cmd/evolve"
  - criterion: "--goal-from-last with nothing recorded exits 1"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestLoopGoalSource_GoalFromLastWithNothingRecordedExits1$' ./cmd/evolve"
  - criterion: "--goal-file resolves the file text and the printed run config names every goal source"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestLoopGoalSource_(GoalFileResolvesTheFileText|PrintedConfigNamesEveryGoalSource)$' ./cmd/evolve"
  - criterion: "The built CLI and runtime-reference.md's launch step carry both flags"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1886_00[2-6]_' ./acs/cycle1886"
---

# Eval: evolve loop records its goal and replays it with --goal-from-last / --goal-file

> Pins the goal-source contract of `evolve loop`. A real launch records its goal
> text atomically in `.evolve/last-goal.txt`; `--goal-from-last` replays it with the
> same goal hash, and `--goal-file F` reads it from a file. Two goal sources together
> exit 10, and `--goal-from-last` with nothing recorded exits 1. The printed run config
> names the goal source. Source incident: inbox item cli-loop-goal-from-last
> (2026-09-30): operators retyped each wave's goal by hand, and a quoting slip changed
> the goal hash without warning. Contract authored in cycle 1886.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| record-and-replay | launch records atomically; --goal-from-last replays the hash | 8/10 | `go test -run TestLoopGoalSource_LaunchRecords...` |
| conflict-negatives | two goal sources exit 10 | 7/10 | `go test -run TestLoopGoalSource_TwoGoalSourcesExit10` |
| empty-record-negative | nothing recorded exits 1 | 7/10 | `go test -run TestLoopGoalSource_GoalFromLastWithNothingRecordedExits1` |
| source-naming | goal_source in the run config | 6/10 | `go test -run TestLoopGoalSource_PrintedConfigNamesEveryGoalSource` |
| cli-and-docs | binary help and launch step list both flags | 5/10 | `go test -tags acs -run TestC1886_00[2-6]_` |
