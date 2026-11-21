---
score_cap:
  - criterion: "The item that the wave latency check files for a breach passes inbox lint"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2588_001' ./acs/cycle2588"
  - criterion: "The item that the practice watch files for a changed source passes inbox lint"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2588_002' ./acs/cycle2588"
  - criterion: "evolve inbox lint exits 0 on a runtime inbox that holds only these two loop-filed items"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2588_003' ./acs/cycle2588"
  - criterion: "evolve inbox lint still refuses an item with no declared surface"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2588_004' ./acs/cycle2588"
---

# Eval: Loop-filed items declare a surface that inbox lint accepts

> Pins that the two loop producers of inbox items, `evolve wave latency --check` and `evolve practice watch`,
> file items that pass `evolve inbox lint`. Both items had no `files` list and no console route, so each
> boundary inbox PR went red until the console removed them. Source incident: the inbox item
> loop-filed-items-declare-a-surface-that-inbox-lint-accepts (cycle 2588).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| latency-item | Wave latency breach item passes lint | 4/10 | `go test -tags acs -run TestC2588_001 ./acs/cycle2588` |
| practice-item | Practice watch item passes lint | 4/10 | `go test -tags acs -run TestC2588_002 ./acs/cycle2588` |
| lint-exit | Lint exits 0 on both items together | 5/10 | `go test -tags acs -run TestC2588_003 ./acs/cycle2588` |
| lint-still-strict | Lint still refuses a no-surface item | 6/10 | `go test -tags acs -run TestC2588_004 ./acs/cycle2588` |
