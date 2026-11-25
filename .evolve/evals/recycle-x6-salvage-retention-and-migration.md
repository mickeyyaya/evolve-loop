---
score_cap:
  - criterion: "A legacy full bundle becomes a thin bundle that verifies, even when its objects left the repository"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2645_001 ./acs/cycle2645"
  - criterion: "A full bundle that cannot be migrated stays byte for byte and leaves no temporary ref"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2645_002 ./acs/cycle2645"
  - criterion: "A leaf with untracked files or an uncommitted patch stays over the cap"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2645_003|TestC2645_004' ./acs/cycle2645"
  - criterion: "Over the cap an unlanded leaf younger than the TTL stays and only landed leaves go, oldest first"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2645_005|TestC2645_006' ./acs/cycle2645"
  - criterion: "A continuation bound leaf stays and a store with no cap evicts nothing"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2645_007|TestC2645_008' ./acs/cycle2645"
---

# Eval: salvage retention and legacy bundle migration

> Pins the K5 size cap and the K6 migration of plan PR #1585. The predicates run the built `evolve gc --only salvage` against a fixture hub, so a seam that no production path reaches cannot pass. Source incident: on 2026-11-20, 206 legacy leaves held about 135 GB.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| migration | full bundle becomes a verified thin bundle | 6/10 | `go test -tags acs -run TestC2645_001 ./acs/cycle2645` |
| migration-failure | the full leaf stays on a failure | 6/10 | `go test -tags acs -run TestC2645_002 ./acs/cycle2645` |
| protected-content | untracked files and patch keep the leaf | 7/10 | `go test -tags acs -run 'TestC2645_003\|TestC2645_004' ./acs/cycle2645` |
| landed-only | unlanded leaf stays, oldest landed goes first | 7/10 | `go test -tags acs -run 'TestC2645_005\|TestC2645_006' ./acs/cycle2645` |
| holds-and-default | continuation hold, no cap means no eviction | 6/10 | `go test -tags acs -run 'TestC2645_007\|TestC2645_008' ./acs/cycle2645` |
