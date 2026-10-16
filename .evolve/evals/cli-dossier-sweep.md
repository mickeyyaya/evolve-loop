---
score_cap:
  - criterion: "`evolve dossier sweep --dry-run` lists a fixture's dirty complete pair, exits 0 and changes neither HEAD nor the tree"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1908_001_' ./acs/cycle1908"
  - criterion: "`evolve dossier sweep` commits the complete orphan pair as one closeout commit, leaves a half pair uncommitted and names both cycles"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1908_002_' ./acs/cycle1908"
  - criterion: "A live run lease exits 1 naming the run and commits nothing"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1908_003_' ./acs/cycle1908"
  - criterion: "A clean corpus exits 0 without a commit and a stray argument exits 10 from the sweep verb"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1908_004_' ./acs/cycle1908"
  - criterion: "The dossier usage names sweep and runtime-reference.md documents the verb"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1908_005_' ./acs/cycle1908"
  - criterion: "The cmd-level sweep tests pass in-process"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^(TestRunDossierSweep_|TestRunDossier_UsageNamesSweep)' ./cmd/evolve"
---

# Eval: `evolve dossier sweep` gives SweepOrphans a CLI caller

> Pins the operator verb that recommits orphaned dossier pairs in `knowledge-base/cycles`. `dossier.SweepOrphans` had no CLI caller at HEAD 2652b4323 (`go/internal/dossier/sweep.go:26`). The verb reuses the publish lock and live-run check of `dossier publish`, and `--dry-run` plans without a commit.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| dry-run | lists, commits nothing | 6/10 | `TestC1908_001_` |
| commit | pair committed, half pair left | 7/10 | `TestC1908_002_` |
| hold | live lease exits 1 | 7/10 | `TestC1908_003_` |
| edges | clean corpus, usage error | 5/10 | `TestC1908_004_` |
| docs | usage and runtime-reference | 4/10 | `TestC1908_005_` |
