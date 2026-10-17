---
score_cap:
  - criterion: "evolve gc --only cache trims the go build cache and the pipeline temp sweep and runs no other sweep"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1937_004 ./acs/cycle1937"
  - criterion: "evolve gc --only with an unknown scope is refused with a usage error that names the value"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1937_005 ./acs/cycle1937"
  - criterion: "evolve gc --only cache --dry-run previews both steps, mutates nothing and keeps main; the cache scope trims nothing while disk free is above gc.cache_trim_free_gb"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1937_00[78]' ./acs/cycle1937"
  - criterion: "gc.cache_trim_free_gb decodes as a typed policy key and a non-numeric value is refused"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC1937_006 ./acs/cycle1937"
  - criterion: "a landed, clean dev worktree with no open pull request is archived inside the idle limit"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1937_001 ./acs/cycle1937"
  - criterion: "a landed dev worktree that is dirty, has an open pull request or has a commit beyond main is kept, and a new worktree with no own work is kept"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1937_00[23]' ./acs/cycle1937"
  - criterion: "every gc scope test in cmd/evolve calls t.Parallel first or is listed serial, and none of the amplified tests is deleted"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1937_009 ./acs/cycle1937"
---

# Eval: evolve gc cache scope and the archive of merged dev worktrees

> Pins two gc gaps that the console measured on 2026-10-17 (inbox item
> gc-cache-scope-and-merged-dev-worktrees, cycle 1935). The go build cache grew from 21.95 GB to
> 66 GB in one hour, and `evolve gc` had no scope that was safe while lanes ran. The dev worktree
> archive kept 95 dev worktrees with merged pull requests and clean trees, for the full 14 days.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| cache-scope | `--only cache` runs only the cache trim and the temp sweep | 7/10 | `TestC1937_004` |
| scope-refusal | an unknown scope is a usage error | 6/10 | `TestC1937_005` |
| policy-key | `gc.cache_trim_free_gb` is typed | 5/10 | `TestC1937_006` |
| parallel-default | the gc scope tests obey the cmd/evolve parallel gate (cycle 1937 audit) | 6/10 | `TestC1937_009` |
| landed-archive | a landed clean tree is archived early | 8/10 | `TestC1937_001` |
| landed-keep | dirty, open-PR, ahead-of-main and new trees stay | 8/10 | `TestC1937_002`, `TestC1937_003` |
