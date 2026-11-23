---
score_cap:
  - criterion: "An executable dir unused past the horizon is trimmed and its bytes count in the report and the cap"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2613_001|TestC2613_002' ./acs/cycle2613"
  - criterion: "A dir with a file in the holder snapshot stays, and unknown liveness spares it"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2613_003|TestC2613_004|TestC2613_005' ./acs/cycle2613"
  - criterion: "Report bytes equal the sum of the trimmed entries"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2613_006' ./acs/cycle2613"
  - criterion: "evolve gc reaches the dir trim and the holder probe through its command"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2613_007|TestC2613_008' ./acs/cycle2613"
---

# Eval: Go cache trim counts executable dirs and spares a held one

> The Go build cache trim skipped every directory entry, so executable dirs named `<hash>-d` escaped the age trim and the byte cap. This eval pins the fix: a dir is one entry sized by its files, aged by its newest inner file, and spared when a live process holds a file inside it. Source: inbox item recycle-x2-gocache-counts-executable-dirs (PR #1585, D12, X2), cycle 2606; cycle 2613 continues it after the build floor refused the skip-hides-error test.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| dir-trimmed-and-counted | Stale dir trimmed, bytes in cap | 4/10 | `TestC2613_001`, `002` |
| held-dir-spared | Holder snapshot spares the dir | 5/10 | `TestC2613_003`, `004`, `005` |
| bytes-equal-sum | Report matches trimmed entries | 6/10 | `TestC2613_006` |
| command-wiring | `evolve gc` reaches the trim and probe | 5/10 | `TestC2613_007`, `008` |
