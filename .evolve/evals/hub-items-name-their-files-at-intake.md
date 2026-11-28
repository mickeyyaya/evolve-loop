---
score_cap:
  - criterion: "Filing an item whose files name a whole hub dir warns with the typed INBOX_HUB_FILES_UNNAMED signal in shadow and still files the item"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2687_001 ./acs/cycle2687"
  - criterion: "The warning names the hub and suggests the cmd_<verb>.go file family"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2687_002 ./acs/cycle2687"
  - criterion: "Items with named files or a non-hub whole dir draw no hub warning; a hub dir beside named files still warns"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2687_003|TestC2687_004' ./acs/cycle2687"
---

# Eval: Items that touch a hub package name their files at intake

> Pins the intake contract of `evolve inbox add` for items that declare a whole hub directory (`go/cmd/evolve`, `go/internal/core`) in `files`. Shadow warns with a typed signal and files the item. The enforce flip needs a new stage key in the protected `go/internal/policy/stage_keys*.go` registry, so it is a console follow-up and no predicate pins it. Source incident: cycle 2682 failed in the TDD handoff floor because its predicate copied a test helper from another package.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| shadow-warn | typed warning, item still filed | 6/10 | `go test -tags acs -run TestC2687_001 ./acs/cycle2687` |
| names-hub-and-family | hub and cmd_ family in the warning | 6/10 | `go test -tags acs -run TestC2687_002 ./acs/cycle2687` |
| negatives | no warning for named files or non-hub dirs | 7/10 | `go test -tags acs -run 'TestC2687_003|TestC2687_004' ./acs/cycle2687` |
