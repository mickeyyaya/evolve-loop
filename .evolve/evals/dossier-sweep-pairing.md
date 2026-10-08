---
score_cap:
  - criterion: "Same-numbered cycle files in different directories are never swept as one pair"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1835_001_' ./acs/cycle1835"
  - criterion: "SweepOrphans sweeps only the dossier directory and ignores tracked modifications"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^(TestC1835_002_|TestC1835_003_)' ./acs/cycle1835"
  - criterion: "Write(nil) returns an error without panicking"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1835_005_' ./acs/cycle1835"
  - criterion: "BuildOpts has no dead LedgerPath field"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1835_004_' ./acs/cycle1835"
---

# Eval: dossier SweepOrphans pairing, Write nil guard, dead BuildOpts field

> Pins inbox item dossier-sweep-pairing (comment-reduction batch 11 finding).
> SweepOrphans paired files by cycle number alone anywhere in the repo, and
> picked up tracked modifications; Write(nil) dereferenced before a nil check;
> BuildOpts.LedgerPath was unused. Source incident: cycle 1835 RED contract.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| cross-dir pairing | different directories never paired | 6/10 | `TestC1835_001_` |
| scope + tracked | dossier dir only, tracked ignored | 6/10 | `TestC1835_002_`, `TestC1835_003_` |
| nil guard | Write(nil) errors, no panic | 7/10 | `TestC1835_005_` |
| dead field | LedgerPath removed | 5/10 | `TestC1835_004_` |
