---
score_cap:
  - criterion: "A named helper, internal/gittest.Fixture, builds a committable git work tree (no ambient identity needed) and the tree is gone once the test that built it ends"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1706_001_' ./acs/cycle1706/"
  - criterion: "Fixture and Repo.Git fail the test loudly, with the failing arguments, on an unwritable root or a failing git command (never a skip or a silent zero Repo)"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1706_002_' ./acs/cycle1706/"
  - criterion: "No fixture repo can spawn a detached auto-maintenance git child, through the helper or through a raw git in the repo, even when the global config asks for one"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1706_003_' ./acs/cycle1706/"
  - criterion: "The sighted tests build their repos through gittest: every commit/fetch made by TestDefaultBuildFloorChecks_IncludesPersonaBudgetCheck, TestLaneStartRef_IntegrationHeadAuthority and TestEngine_RunAllExecutesPureAndCycleSpecs (fixture and production code alike) runs in a repo that cannot detach a maintenance child"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1706_004_' ./acs/cycle1706/"
  - criterion: "Teardown absorbs a writer that outlives its test body (bounded retry): the test passes and the fixture tree is gone"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1706_005_' ./acs/cycle1706/"
  - criterion: "When teardown exhausts its retries the failure names the holding process by PID alongside the fixture path"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1706_006_' ./acs/cycle1706/"
  - criterion: "With no lsof on PATH the teardown failure states 'diagnostic unavailable' next to the path instead of omitting the holder silently"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1706_007_' ./acs/cycle1706/"
  - criterion: "A -race -count=20 stress run of the affected tests is green (a TempDir cleanup race fails the run)"
    max_if_missing: 8
    evidence: "cd go && go test -race -count=20 -run '^(TestDefaultBuildFloorChecks_IncludesPersonaBudgetCheck|TestLaneStartRef_IntegrationHeadAuthority)$' ./internal/core && go test -race -count=20 -run '^TestEngine_RunAllExecutesPureAndCycleSpecs$' ./internal/routingtest && go test -race -count=20 ./internal/gittest"
  - criterion: "internal/gittest is enrolled in go/.apicover-enforce with every export named by a test, executed, and documented"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1706_009_' ./acs/cycle1706/"
---

# Eval: One git-fixture helper owns creation and teardown (tempdir-cleanup-vs-git-flake)

> Triage slug `gittest-fixture-centralize`. A recurring CI flake (PR #545
> ubuntu routingtest, PR #548 macOS core, recurred in #638 and #643): a
> git-backed test's assertions pass, then `t.TempDir()`'s single `RemoveAll`
> fails with `unlinkat …/.git/objects: directory not empty` because a git
> process is still writing in the fixture. Cycle 1706 reproduced it at 25%
> (`.evolve/runs/cycle-1706/bug-reproduction-report.md`) with a detached
> background packer, and its TDD phase found that on git ≥ 2.47 every `commit`
> and `fetch` spawns `git maintenance run --auto --detach` — which `gc.auto=0`
> does not stop; only `maintenance.auto=false` (or autoDetach=false) does.
>
> The fix centralizes every git fixture in `internal/gittest`: repos whose own
> config keeps maintenance in the foreground (so production code committing
> inside a fixture inherits it), a teardown that retries a bounded number of
> times, and a failure that names who holds the tree. This eval pins those
> behaviors; the Linux half of the stress criterion is the PR's CI run.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| helper-positive | Fixture builds a committable tree that is gone at test end | 7/10 | `TestC1706_001_*` |
| helper-negative | unwritable root / failing git fail loudly with context | 6/10 | `TestC1706_002_*` |
| root-cause | no detached maintenance child from any fixture repo | 8/10 | `TestC1706_003_*` |
| caller-proof | the sighted tests' commits/fetches all run in quiet repos | 9/10 | `TestC1706_004_*` |
| the-race | retry outlasts a writer that outlives its test | 9/10 | `TestC1706_005_*` |
| diagnostic | exhaustion names the holder by PID | 7/10 | `TestC1706_006_*` |
| diagnostic-fallback | "diagnostic unavailable" when lsof is absent | 5/10 | `TestC1706_007_*` |
| stress | -race -count=20 of the affected tests green | 8/10 | `go test -race -count=20 …` |
| graduation | apicover-enforced, every export named/executed/documented | 6/10 | `TestC1706_009_*` |
