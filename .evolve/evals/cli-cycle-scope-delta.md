---
score_cap:
  - criterion: "evolve cycle scope-delta is a subcommand of evolve cycle and exits 0 on an in-scope fixture"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1879_001_' ./acs/cycle1879"
  - criterion: "An undeclared changed path is listed and the command exits 1"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1879_002_' ./acs/cycle1879"
  - criterion: "A path in a same-wave sibling lane's declared scope is cross-lane; a lane of another wave is not"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1879_003_' ./acs/cycle1879"
  - criterion: "In-scope plus computed-closure changes exit 0 with carved=0 refused=0"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1879_004_' ./acs/cycle1879"
  - criterion: "--json emits one {cycle, summary, entries} object whose summary is scopedelta.Summary"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1879_005_' ./acs/cycle1879"
  - criterion: "A missing or corrupt cycle run exits 2"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1879_006_' ./acs/cycle1879"
  - criterion: "A missing, non-numeric, zero or negative cycle and an unknown flag exit 10 from scope-delta itself"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1879_007_|TestC1879_008_' ./acs/cycle1879"
  - criterion: "runtime-reference.md documents evolve cycle scope-delta"
    max_if_missing: 8
    evidence: "grep -q 'evolve cycle scope-delta' docs/operations/runtime-reference.md"
  - criterion: "cmd_cycle_scope_delta.go has no comments and cmd_cycle.go gains none"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1879_010_' ./acs/cycle1879"
---

# Eval: evolve cycle scope-delta

> Pins `evolve cycle scope-delta --cycle N [--json]`. The command reads the cycle's declared files from its claimed inbox record (`.evolve/inbox/processing/cycle-N/*-<id>.json` for each `lane-scope.json` `todo_ids` id), diffs its `active_worktree` against `worktree_base_sha`, and classifies each changed path with `scopedelta.Classify`. Sibling scopes come from the other `runs/cycle-*` lanes that share the cycle's `goal_hash`. Exit 0 = all kept, 1 = any carved or refused, 2 = I/O, 10 = usage. Source: inbox item `cli-cycle-scope-delta` (2026-09-30), cycle 1879; motivated by the cycle 1718/1719 doc conflict and undeclared `offenders.json` edits by size-ratchet lanes.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| wiring | subcommand reachable through the built binary | 4/10 | `TestC1879_001_` |
| undeclared | undeclared path listed, exit 1 | 6/10 | `TestC1879_002_` |
| cross-lane | same-wave sibling path is cross-lane, other wave is not | 7/10 | `TestC1879_003_` |
| clean | in-scope + closure exit 0 | 8/10 | `TestC1879_004_` |
| json | Summary + entries JSON | 8/10 | `TestC1879_005_` |
| negative | missing or corrupt run exits 2 | 7/10 | `TestC1879_006_` |
| edge | usage errors exit 10; empty diff exits 0 | 6/10 | `TestC1879_007_`, `TestC1879_008_` |
| docs | runtime-reference documents the command | 8/10 | `grep` |
| comments | zero comments in new Go code | 8/10 | `TestC1879_010_` |
