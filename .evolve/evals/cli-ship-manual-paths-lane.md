---
score_cap:
  - criterion: "evolve ship --class manual --paths a.go commits a.go only while an untracked b.go stays out of the commit"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2028_001' ./acs/cycle2028"
  - criterion: "an attestation bound to a different staged set is refused as stale under --paths and without it"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2028_002|TestC2028_005' ./acs/cycle2028"
  - criterion: "evolve ship --class manual --dry-run --paths a.go lists a.go as the staged set and leaves HEAD, index and working tree unchanged"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2028_003' ./acs/cycle2028"
  - criterion: "a ship without --paths still stages the whole tree, and the lane diff changes no protected control-plane path"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2028_004|TestC2028_006' ./acs/cycle2028"
---

# Eval: evolve ship --paths for the manual class

> Pins the `--paths` flag of `evolve ship --class manual`. Before it, the manual
> class ran `git add -A` and shipped the whole tree, so an operator with
> unrelated untracked files could not commit one file. The predicates drive the
> built `evolve` binary against a real git repo with a bare origin. Source:
> inbox item cli-ship-manual-paths-lane; cycle 2022 failed the build floor
> because its diff changed protected `native.go` and `gitops.go`, so cycle 2028
> pins that fence too.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| named-only | Only the named file is committed | 3/10 | `go test -tags acs -run TestC2028_001 ./acs/cycle2028` |
| stale-binding | Attestation of another staged set is refused | 4/10 | `go test -tags acs -run 'TestC2028_002\|TestC2028_005' ./acs/cycle2028` |
| dry-run | Dry run prints the named staged set, changes nothing | 4/10 | `go test -tags acs -run TestC2028_003 ./acs/cycle2028` |
| default and fence | Whole-tree default kept; no protected path in the lane diff | 5/10 | `go test -tags acs -run 'TestC2028_004\|TestC2028_006' ./acs/cycle2028` |
