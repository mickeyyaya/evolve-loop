---
score_cap:
  - criterion: "No package under go/internal/cli declares a package-level atomic Deps pointer"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2672_001_' ./acs/cycle2672"
  - criterion: "The guard test exists, passes on the tree and fails on an atomic.Pointer[Deps] global"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2672_003_' ./acs/cycle2672"
  - criterion: "A run keeps its own Deps when another run starts or returns first, in landcmd and inboxcmd"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2672_00[45]_' ./acs/cycle2672"
  - criterion: "TestRunCI_DocsTests and the inboxcmd tests run with t.Parallel"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2672_00[67]_' ./acs/cycle2672"
  - criterion: "landcmd and inboxcmd pass under -race and pause their parallel tests at run time"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2672_00[89]_' ./acs/cycle2672"
---

# Eval: CLI verbs pass their Deps down, not through a process-global

> Pins the removal of `var current atomic.Pointer[Deps]` from landcmd and inboxcmd
> (cycle 2672, inbox item cli-verbs-pass-deps-not-a-process-global). The 2026-11-24
> cross-test swap flake ejected PR 1746; a guard test now keeps the shape out of
> every verb package under go/internal/cli.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| global-gone | No atomic Deps pointer under go/internal/cli | 4/10 | `TestC2672_001_` |
| guard | Guard test passes on the tree, fails on an offender | 6/10 | `TestC2672_003_` |
| interleave | A run keeps its Deps when another run overlaps | 5/10 | `TestC2672_004_`, `TestC2672_005_` |
| parallel | DocsTests and inboxcmd tests call t.Parallel | 6/10 | `TestC2672_006_`, `TestC2672_007_` |
| race | landcmd and inboxcmd pass `-race`, tests pause at run time | 6/10 | `TestC2672_008_`, `TestC2672_009_` |
