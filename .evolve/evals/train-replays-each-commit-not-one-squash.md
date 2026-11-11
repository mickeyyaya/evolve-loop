---
score_cap:
  - criterion: "the wave operations guide states that the train lands a PR of many commits as many commits and never squashes"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2292_00[12]_' ./acs/cycle2292"
  - criterion: "a train test pins that a PR with two commits lands as two integrate commits"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestConsoleTwoCommitPRLandsAsTwoIntegrateCommits$' ./internal/integration"
  - criterion: "the two-commit train test exists, runs and is not skipped"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2292_003_' ./acs/cycle2292"
---

# Eval: The train replays each commit of a PR

> Pins the landing rule of the console PR train. A PR with N commits lands as N integrate commits, each titled `evolve-pr N: integrate`, and the train never squashes them. The wave operations guide said "one commit on main", which misled a reader. Source incident: inbox item `train-replays-each-commit-not-one-squash`, cycle 2292.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| docs-state-replay | The guide says no squash and names the many-commit case | 7/10 | `go test -tags acs -run 'TestC2292_00[12]_' ./acs/cycle2292` |
| test-pins-replay | A two-commit PR lands as two integrate commits | 6/10 | `go test -run '^TestConsoleTwoCommitPRLandsAsTwoIntegrateCommits$' ./internal/integration` |
| test-exists | The test exists and is not skipped | 6/10 | `go test -tags acs -run 'TestC2292_003_' ./acs/cycle2292` |
