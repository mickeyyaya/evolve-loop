---
score_cap:
  - criterion: "Run with an empty Deps does not leave a nil LaneForbidden for a later parallel lint"
    max_if_missing: 4
    evidence: "cd go && go test -count=20 -run '^TestRun_AnEmptyDepsDoesNotLeakIntoALaterParallelLint$' ./internal/cli/inboxcmd"
  - criterion: "Run and RunMover restore the previous package host on return"
    max_if_missing: 5
    evidence: "cd go && go test -count=20 -run '^(TestRun_RestoresThePreviousHostOnReturn|TestRunMover_RestoresThePreviousHostOnReturn)$' ./internal/cli/inboxcmd"
  - criterion: "The inboxcmd package passes under the race detector ten times"
    max_if_missing: 6
    evidence: "cd go && go test -race -count=10 ./internal/cli/inboxcmd"
  - criterion: "No inboxcmd test outside the isolation test calls Run or RunMover with a Deps literal lacking LaneForbidden"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC2662_00[34]_' ./acs/cycle2662"
---

# Eval: inboxcmd tests never leak an empty Deps into the package host

> Pins the fix for the package-global `current` Deps in `go/internal/cli/inboxcmd`.
> `Run` and `RunMover` stored their Deps argument and never restored it, so a test
> that called `Run` with `Deps{}` left a nil `LaneForbidden` behind and a parallel
> `runInboxLint` panicked in `loadPendingInbox`. Source: cycle 2662 scout item
> `inboxcmd-tests-never-leak-an-empty-deps-into-the-package-host`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| leak-then-lint | no panic after a `Deps{}` Run | 4/10 | `go test -count=20 -run TestRun_AnEmptyDepsDoesNotLeakIntoALaterParallelLint` |
| host-restore | previous host restored by Run and RunMover | 5/10 | `go test -count=20 -run TestRun_RestoresThePreviousHostOnReturn` |
| race-clean | package passes `-race -count=10` | 6/10 | `go test -race -count=10 ./internal/cli/inboxcmd` |
| no-empty-deps-callers | no stray `Deps{}` caller | 6/10 | `go test -tags acs -run TestC2662_003` |
