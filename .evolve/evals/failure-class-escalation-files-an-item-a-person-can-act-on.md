---
score_cap:
  - criterion: "A console escalation item names each occurrence (cycle, phase, checks) and the class that must stop recurring"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestFailureClassItem_TheConsoleItemNamesEachOccurrenceAndTheClassToStop$' ./internal/core"
  - criterion: "A further occurrence of the same class updates the same item and files no new one"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestFailureClassItem_AFurtherOccurrenceUpdatesTheSameItem$' ./internal/core"
  - criterion: "The replay of the nine failure classes files one actionable item per class through the real entry point"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestFailureClassItem_TheNineClassReplay$' ./internal/core"
  - criterion: "Inbox lint warns on the nine items as filed on 2026-11-19 and accepts the replayed items"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2534_00[34]' ./acs/cycle2534"
---

# Eval: failure-class escalation files an item a person can act on

> Pins the content of the console item that a failure-class escalation files.
> Source incident: cycle 2534, the nine `failure-class-*` items filed between 2026-11-04 and 2026-11-14.
> Each item carried only a pattern and a recurrence count: no title, no summary, no acceptance.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| occurrence-evidence | Summary names each occurrence and the acceptance names the class | 6/10 | `go test -run TestFailureClassItem_TheConsoleItemNamesEachOccurrenceAndTheClassToStop` |
| same-item-update | A further occurrence updates the one item | 7/10 | `go test -run TestFailureClassItem_AFurtherOccurrenceUpdatesTheSameItem` |
| nine-class-replay | The replay files nine items, one per class | 7/10 | `go test -run TestFailureClassItem_TheNineClassReplay` |
| lint-missing-field | Lint warns on the old shape and accepts the new shape | 6/10 | `go test -tags acs -run TestC2534_00[34] ./acs/cycle2534` |
