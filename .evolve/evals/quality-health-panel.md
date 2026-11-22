---
score_cap:
  - criterion: "evolve status prints a Quality block with one line per report or gate index, with value, interval, n, window and stage"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2604_001_|TestC2604_005_' ./acs/cycle2604"
  - criterion: "the dashboard Snapshot carries Quality []QualityReading from the root stream, the cycle streams and the quality.reading channel"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2604_002_|TestC2604_003_|TestC2604_004_|TestC2604_013_' ./acs/cycle2604"
  - criterion: "a breach reading carries '!' and no line or field sums or averages indexes"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2604_006_|TestC2604_007_' ./acs/cycle2604"
  - criterion: "an absent stream prints 'no reading yet' for each index and exits 0"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2604_008_|TestC2604_009_' ./acs/cycle2604"
  - criterion: "evolve status --quality all adds the shadow indexes and a bad value is a usage error"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2604_011_|TestC2604_012_' ./acs/cycle2604"
  - criterion: "changed functions readQuality and writeStatusQuality are at 100% coverage"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2604_010_' ./acs/cycle2604"
---

# Eval: Quality panel in evolve status and the dashboard snapshot

> Pins Q11 of the quality indexes plan (ADR-0153, spec `docs/architecture/llm-quality-indexes.md` section 11). The panel prints separate readings and never a total. Source incident: inbox item quality-health-panel, cycle 2604.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| block-lines | one line per report or gate index with value, interval, n, window, stage | 5/10 | `TestC2604_001_`, `TestC2604_005_` |
| snapshot | Snapshot.Quality from root, cycle and channel streams | 6/10 | `TestC2604_002_`, `_003_`, `_004_`, `_013_` |
| no-total-and-breach | no aggregate; '!' on breach only | 6/10 | `TestC2604_006_`, `TestC2604_007_` |
| absent-stream | 'no reading yet' and exit 0 | 6/10 | `TestC2604_008_`, `TestC2604_009_` |
| shadow-flag | --quality all adds shadow; bad value refused | 6/10 | `TestC2604_011_`, `TestC2604_012_` |
| coverage | changed functions at 100% | 7/10 | `TestC2604_010_` |
