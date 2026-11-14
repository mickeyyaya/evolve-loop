---
score_cap:
  - criterion: "evolve gc recycles an orphaned Test* or gittest* temp dir and spares one that a live process holds"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2364_001|TestC2364_002|TestC2364_003' ./acs/cycle2364"
  - criterion: "free disk below the stated floor or warn level raises a named WARN signal and the sweep still runs"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2364_004|TestC2364_005|TestC2364_006|TestC2364_007' ./acs/cycle2364"
  - criterion: "the seat brief forbids deleting shared temp and names evolve gc as the one route"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2364_008|TestC2364_009' ./acs/cycle2364"
---

# Eval: test temp directories no longer fill the host disk

> Pins the three acceptance criteria of `test-temp-dirs-fill-the-host-disk`
> (cycle 2364). `ReapPipelineTemp` matched pipeline names only and had no
> liveness proof, so suite temp dirs (`Test*`, `gittest*`) filled the disk and a
> seat cleared shared temp by hand. Each check drives the real `evolve gc
> --only cache` binary or the real `phaseidentity.Authority()` text.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| journey | orphan recycled, held dir spared | 4/10 | `TestC2364_001`, `002`, `003` |
| disk signal | floor and warn signals, stated floor | 5/10 | `TestC2364_004`..`007` |
| seat brief | forbids deleting shared temp, names the gc route | 6/10 | `TestC2364_008`, `009` |
