---
score_cap:
  - criterion: "The scanner output is teed to the run-dir scan log on BOTH green and red runs"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestRepoContractGate_ScanLogPersistedOnGreenAndRedRuns$' ./internal/phases/ship"
  - criterion: "The CodeRepoContractGate error message names the parsed failing tests"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestRepoContractGate_RedErrorMessageNamesFailingTests$' ./internal/phases/ship"
  - criterion: "The production caller (Phase.runNative) reaches the gate carrying the run workspace"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -run '^TestRunNative_RepoContractGateReceivesRunWorkspace$' ./internal/phases/ship"
---

# Eval: repo-contract scan-log persistence

> Pins the forensic-evidence contract of the ship-time repo-contract scanner
> pack. Before cycle-1409 both `cmd.Stdout` and `cmd.Stderr` were wired only to
> the ship's `stderr io.Writer` — nothing was teed to a run-directory artifact —
> so when the gate fired, the failing test names and the full scanner chatter
> were unrecoverable from `ship-error.json` or any run artifact. That is why the
> cycle-1402/1403 false RED could not be diagnosed in place and needed a manual
> worktree re-run to disprove. The fix writes
> `<runs>/cycle-N/ship-repocontract-scan.log` unconditionally (green runs too —
> the green baseline is half the diagnosis) and threads the parsed failing test
> names into the ship error message.
>
> The third cap is the wiring proof: the log path is derived from the run
> workspace, so a seam that is only reachable from a test is dead code. The
> production caller `ship.Phase.runNative` (`go/internal/phases/ship/ship.go`)
> must reach the gate with `req.Workspace` — the cycle-1064 manifest-gate
> anti-trap (a gate threaded nowhere is permanently off) applied to the new
> parameter.
>
> Source incident: cycles 1402/1403/1405; inbox item
> `repocontract-gate-false-red-swallowed-diag` FIX SHAPE (b).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| unconditional-tee | Scan log written on green AND red paths | 8/10 | `go test -run TestRepoContractGate_ScanLogPersistedOnGreenAndRedRuns ./internal/phases/ship` |
| named-failures | RED ship error message carries the failing test names | 7/10 | `go test -run TestRepoContractGate_RedErrorMessageNamesFailingTests ./internal/phases/ship` |
| caller-reachability | `runNative` threads `req.Workspace` into the gate (wiring proof) | 9/10 | `go test -run TestRunNative_RepoContractGateReceivesRunWorkspace ./internal/phases/ship` |
