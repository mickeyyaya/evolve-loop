---
score_cap:
  - criterion: "RunPool with a nil launcher fails every lane with the Supervisor.Run shape (Index i, ExitCode -1, errNoLaunch) instead of returning zero-value success results"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -v -run '^TestRunPool_NilLaunch_FailsEveryLaneWithErrNoLaunch$' ./internal/fleet | grep -q -- '--- PASS: TestRunPool_NilLaunch_FailsEveryLaneWithErrNoLaunch'"
  - criterion: "globalZoneFiles equals the global zone documented in docs/architecture/packages/internal-fleet.md, pinned by a test"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -v -run '^TestGlobalZoneFiles_MatchesDocumentedGlobalZone$' ./internal/fleet | grep -q -- '--- PASS: TestGlobalZoneFiles_MatchesDocumentedGlobalZone'"
  - criterion: "TestSupervisor_BoundedConcurrency waits on a channel bounded by a timeout and has no busy-wait loop"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1823_003_BoundedConcurrencyTestWaitsOnATimeoutNotASpin$' ./acs/cycle1823 | grep -q -- '--- PASS: TestC1823_003_BoundedConcurrencyTestWaitsOnATimeoutNotASpin'"
  - criterion: "The fleet lane leaves internal/loopwave green: the quota-bench WARN keeps the text loopwave pins under the protected control plane"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 ./internal/loopwave"
  - criterion: "Supervisor checks a nil LaunchFn only in Validate; the dead nil branch in launchOne is gone and Run still fails every spec"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1823_005_SupervisorChecksANilLauncherOnlyInValidate$' ./acs/cycle1823 | grep -q -- '--- PASS: TestC1823_005_SupervisorChecksANilLauncherOnlyInValidate'"
  - criterion: "internal-fleet.md cites only tests that some package declares"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1823_006_FleetPackageDocNamesOnlyTestsThatExist$' ./acs/cycle1823 | grep -q -- '--- PASS: TestC1823_006_FleetPackageDocNamesOnlyTestsThatExist'"
  - criterion: "Any WARN-prefix claim internal-fleet.md makes matches the prefix QuotaAwareCount and FreshenSpecs actually print"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1823_007_FleetDocPrefixClaimMatchesPrintedWarnings$' ./acs/cycle1823 | grep -q -- '--- PASS: TestC1823_007_FleetDocPrefixClaimMatchesPrintedWarnings'"
---

# Eval: RunPool fails a nil launcher; fleet global zone, bounded-concurrency wait and the dead nil-launch guard pinned

> Pins the inbox item `fleet-runpool-silent-success` (comment-reduction batch 10,
> 2026-09-26; worked in cycle 1822, continued in cycle 1823). `RunPool` with a nil `LaunchFn` returned
> `len(backlog)` zero-value `Result`s (ExitCode 0, Err nil), so every lane read as
> `LaneOK`, while `Supervisor.Run` fails the same input with `errNoLaunch`. The
> eval also pins the global-zone file list to its documented set, forbids the
> CPU-spinning wait in `TestSupervisor_BoundedConcurrency`, and removes the dead
> nil-`Launch` branch in `Supervisor.launchOne`. Cycle 1822's first audit rejected a
> WARN-prefix unification: `go/internal/loopwave` pins both fleet WARN texts under
> the protected control plane (`plan_test.go:111`, `testdata/stderr_wave*.golden.txt`),
> so either direction reds loopwave. The unification is console follow-up F2
> (`docs/architecture/decomposition/13-loopwave.md:314`), and this eval instead
> requires loopwave to stay green. Cycle 1823 continues the salvaged worktree: its
> predicates live in `go/acs/cycle1823` (the cycle-1822 package was archived), and it
> adds a check that the package doc's WARN-prefix claim matches the printed text.
> The `-run` evidence commands require a reported
> `--- PASS` line, because `go test -run` exits 0 when no test matches.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| silent-success | nil launcher fails every lane like Supervisor.Run | 8/10 | `go test -run TestRunPool_NilLaunch_FailsEveryLaneWithErrNoLaunch ./internal/fleet` |
| global-zone pin | globalZoneFiles equals the documented zone | 5/10 | `go test -run TestGlobalZoneFiles_MatchesDocumentedGlobalZone ./internal/fleet` |
| no spin | bounded-concurrency test waits on a channel with a timeout | 5/10 | `go test -tags acs -run TestC1823_003 ./acs/cycle1823` |
| no loopwave red | the quota-bench WARN keeps loopwave's pinned text | 7/10 | `go test -count=1 ./internal/loopwave` |
| no dead guard | a nil LaunchFn is checked only in Validate | 4/10 | `go test -tags acs -run TestC1823_005 ./acs/cycle1823` |
| doc pins resolve | internal-fleet.md cites only declared tests | 3/10 | `go test -tags acs -run TestC1823_006 ./acs/cycle1823` |
| doc prefix truthful | the doc's WARN-prefix claim matches the printed WARNs | 3/10 | `go test -tags acs -run TestC1823_007 ./acs/cycle1823` |
