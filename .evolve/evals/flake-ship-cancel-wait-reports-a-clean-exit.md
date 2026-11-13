---
score_cap:
  - criterion: "describeWaitResult names exit status, signal and elapsed time, pinned with a fake process state"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestDescribeWaitResult_' ./internal/phases/ship/"
  - criterion: "Cancel still kills the whole process group (grandchild reaped)"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -tags integration -run '^TestCancelKillsTheProcessGroup_' ./internal/phases/ship/"
  - criterion: "The package note records the cause and the macOS -race -count=200 result"
    max_if_missing: 5
    evidence: "grep -q '^### Process-group cancel test' docs/architecture/packages/internal-phases-ship.md && grep -q -- '-count=200' docs/architecture/packages/internal-phases-ship.md"
---

# Eval: TestCancelKillsTheProcessGroup flake reports a clean exit

> Pins the diagnosis contract for the macOS flake of `TestCancelKillsTheProcessGroup_CancelReapsTheGrandchildToo`,
> where `Wait` returned `context canceled` (a success exit status after cancel). The failure message must name
> the exit status, the signal and the elapsed time, so the next red names its cause. Source: census F4.3 in
> `docs/research/flaky-tests-2026-10.md`; cycle 2344.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| fake-state message | exit, signal, elapsed are named | 6/10 | `go test -run '^TestDescribeWaitResult_'` |
| group-kill preserved | grandchild is gone after cancel | 7/10 | `go test -tags integration -run '^TestCancelKillsTheProcessGroup_'` |
| recorded evidence | note section with Cause, Run, Result | 5/10 | `grep` of the section heading and `-count=200` |
