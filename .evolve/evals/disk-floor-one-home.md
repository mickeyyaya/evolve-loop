---
score_cap:
  - criterion: "The new disk.floor_gb key alone sets one floor for evolve gc, the loop preflight and the percent cycle-start guard"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestDiskFloorOneHome_NewKeyAloneIsOneFloorForEveryReader$' ./cmd/evolve"
  - criterion: "An old key (gc.disk_floor_gb, gc.disk_warn_gb, preflight.min_free_gib) still sets the floor and prints a WARN naming the new key"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^TestDiskFloorOneHome_AnOldKeyStillSetsTheFloorAndWarnsNamingTheNewKey$' ./cmd/evolve"
  - criterion: "Different values for an old key and its new key are refused by every reader with an error that names both keys"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^TestDiskFloorOneHome_ConflictingOldAndNewValues' ./cmd/evolve"
  - criterion: "A guard test fails when a second reader of a disk floor appears outside the resolver package"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestSingleReader_' ./internal/policy"
---

# Eval: One policy home for the disk floor

> Pins inbox item disk-floor-one-home (operator decision D35, plan section 13).
> Three places held a free-disk floor in three units: gc.disk_floor_gb and
> gc.disk_warn_gb, preflight.min_free_gib, and a fixed 5% percent guard. In wave
> 186 the percent guard paused cycle starts 20 times (HOST_DISK_LOW) while the
> other floors showed no warning. Cycle 2609 moves the floor to one policy block,
> `disk`, with one resolver in the leaf package `go/internal/diskfloor`. The whole-tree guard sits in `go/internal/policy`, a package of the repo-contract pack (cycle 2609 put it in the leaf and ship refused it).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| one-floor | The new key alone sets one floor for every reader | 4/10 | `TestDiskFloorOneHome_NewKeyAloneIsOneFloorForEveryReader` |
| alias-warn | Old keys alias the new key with a WARN | 5/10 | `TestDiskFloorOneHome_AnOldKeyStillSetsTheFloorAndWarnsNamingTheNewKey` |
| conflict | Conflicting old and new values are refused | 5/10 | `TestDiskFloorOneHome_ConflictingOldAndNewValues*` |
| single-reader | A second reader outside the resolver fails a guard that lives in a repo-contract pack package | 6/10 | `go test -run '^TestSingleReader_' ./internal/policy` |
