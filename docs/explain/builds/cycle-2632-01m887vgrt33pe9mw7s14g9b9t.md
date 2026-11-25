# Build Explanation — Cycle 2632

## Build Binding
- Cycle: 2632
- Base SHA: e144bfba9d3fbb79d29d399556aec7b277e1c18f

## Summary
The knowledge-base package now scores recall at k and MRR over a checked-in gold query set of 36 cycle plans and 75 lessons. The `evolve kb recall score` command prints both metrics for `FileKB.Lookup` and reproduces the recorded baseline.

## Rationale
Evaluating future rankers requires a deterministic, reproducible benchmark on a tracked corpus. The live `.evolve/instincts/lessons/` directory is git-ignored, so a checked-in snapshot of 75 real lessons ensures every checkout scores the same corpus.

## Changed Areas
- `go/internal/research/kb.go` — adds JSON field tags to `Query` so gold queries serialize with lowercase keys.
- `go/internal/research/gold.go` — defines `GoldQuery`, `GoldSet`, and `LoadGoldSet` to read and parse JSON query sets.
- `go/internal/research/recall_score.go` — implements `RecallScore` and `ScoreRecall` with rank cutoffs, duplicate suppression, and input validation.
- `go/internal/research/testdata/gold_set.json` — stores 36 queries quoted from 36 distinct cycle dossiers under `knowledge-base/cycles/`, with reviewer-judged multi-entry and cross-cycle labels, labeller ids, and rationales.
- `go/internal/research/testdata/gold_corpus/` — stores 75 real lesson YAML files from past cycles as the tracked benchmark corpus.
- `go/internal/research/testdata/gold_baseline.json` — records the live `FileKB.Lookup` baseline (`recall_at_5`: 0.587963, `mrr`: 0.615741).
- `go/cmd/evolve/cmd_kb.go` — adds the `evolve kb recall score` subcommand with `--gold`, `--corpus`, `--project-root`, and `--json`.
- `go/cmd/evolve/registry.go` — updates the `kb` subcommand summary to include `kb recall score`.
- `go/cmd/evolve/main.go` — updates the top-level usage text for `kb`.

## Design Decisions
The gold set ships its own tracked lesson snapshot under `testdata/gold_corpus/` instead of reading the git-ignored `.evolve/instincts/lessons/` directory. Each query quotes verbatim failure text from its plan dossier instead of copying lesson pattern slugs. Each query marks relevant entries across cycles with a recorded rationale. `ScoreRecall` wraps ranker lookup errors so callers match them with `errors.Is`, and rejects empty sets, unlabelled queries, or non-positive `k` values.

## Verification
All `internal/research` unit tests, `cmd/evolve` `TestRunKB` and `TestCLIContract` tests, and ACS predicates `TestC2632_001` through `TestC2632_015` pass.

## Compatibility
Existing `evolve kb graph` commands and `FileKB.Lookup` behavior remain unchanged.

## Limitations
The checked-in baseline measures `FileKB.Lookup` on 36 dossier-derived queries over the 75-lesson snapshot rather than the full local lessons directory.
