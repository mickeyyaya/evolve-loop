---
score_cap:
  - criterion: "the code-index bench has a second truth set that go/types did not produce, with interface-method, function-value and static call sites that each read as a real call in their named caller"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2670_00[123]_' ./acs/cycle2670"
  - criterion: "the independent set holds at least one site that the go/types truth of the same module lacks, so the set is not the index restated"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2670_003_' ./acs/cycle2670"
  - criterion: "each bench row names the truth set it was scored on, scores never bleed between sets, and an empty set still gives one zeroed row per serving backend"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2670_00[456]_' ./acs/cycle2670"
  - criterion: "the bench table prints set, precision, recall, p50 and p95 for every row, and evolve code bench scores every backend on both sets"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestCodeBench_IndependentTruth_' ./cmd/evolve && go test -tags acs -count=1 -run 'TestC2670_007_' ./acs/cycle2670"
---

# Eval: second independent truth set for the code-index bench

> Pins the second truth set of `evolve code bench`. Before this task the only truth came from the go/types index, so a backend that agreed with go/types scored 1.00 by construction (dossier docs/research/code-intelligence-tools-2026-11.md, risk R1). The independent set is hand-checked against fixture source and includes interface-method calls and calls through function values, which go/types cannot resolve statically. Source: cycle 2633 inbox item code-index-second-independent-truth-set.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| hand-checked-sites | each site is a real call in its caller, read with go/ast | 7/10 | `TestC2670_001_` |
| call-kind-coverage | interface, function-value and static sites exist | 7/10 | `TestC2670_002_` |
| non-circular | a site go/types lacks | 7/10 | `TestC2670_003_` |
| per-set-rows | rows carry the set name, no bleed, empty set | 6/10 | `TestC2670_004_` to `006_` |
| cli-output | both sets for every backend with p50 and p95 | 7/10 | `TestCodeBench_IndependentTruth_` |
