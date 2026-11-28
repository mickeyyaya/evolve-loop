---
score_cap:
  - criterion: "SyncToOrigin sees the new origin/main when remote.origin.fetch omits main or is unset"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2690_001' ./acs/cycle2690"
  - criterion: "No production Go file outside internal/mainsync runs a bare fetch of origin main (integration/sync.go included)"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2690_004' ./acs/cycle2690"
  - criterion: "Every function of the changed sync files is covered 100 percent"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2690_005' ./acs/cycle2690"
  - criterion: "The integration and mainsync package notes pass STE and drop the until-lands sentence"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2690_007' ./acs/cycle2690"
---

# Eval: SyncToOrigin fetches origin main with the explicit refspec

> Pins the fix of the fourth copy of the fetch rule. `integration.SyncToOrigin` ran a plain
> `git fetch origin main`. With a narrowed or unset `remote.origin.fetch`, git did not update
> `refs/remotes/origin/main`, so the function compared two stale refs and returned `current`.
> Source incident: cycle 2690 bug-reproduction. `cmd_boundary_ratchet.go` and
> `marketplacepoll.go` still hold the same bare shape; they are named in the structure test as
> known remaining sites, outside this lane.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| narrowed-fetch-config | SyncToOrigin returns moved and fast-forwards local main | 3/10 | `go test -tags acs -run TestC2690_001 ./acs/cycle2690` |
| bare-fetch-structure | no bare fetch of origin main outside mainsync | 5/10 | `go test -tags acs -run TestC2690_004 ./acs/cycle2690` |
| coverage | 100 percent of the changed files' functions | 6/10 | `go test -tags acs -run TestC2690_005 ./acs/cycle2690` |
| package-notes | notes in STE, no until-lands sentence | 7/10 | `go test -tags acs -run TestC2690_007 ./acs/cycle2690` |
