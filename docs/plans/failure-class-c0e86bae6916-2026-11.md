# Plan: one hung predicate costs one named red, not the whole Go lane

- **Status:** approved after review round 3, built 2026-11-21, rebuilt on main `eaa0cf56a` 2026-11-23. The holder budget of PR 1656 and the scope concurrency of PR 1614 stay. The bound sits on top of them.
- **Design:** [internal-acssuite](../architecture/packages/internal-acssuite.md), [pkg-acsassert](../architecture/packages/pkg-acsassert.md), [internal-phases-audit-ciparitygate](../architecture/packages/internal-phases-audit-ciparitygate.md).
- **Item:** `failure-class-c0e86bae6916`, rank 66, recurrence 3 (cycles 2547, 2548, 2551).

> **Terms used throughout:**
> - **Go lane:** the part of the ACS suite that runs `go test -tags acs` for each predicate scope.
> - **Scope:** one `go test` pattern of the lane.
> - **Predicate subprocess:** each process that a predicate starts, for example an inner `go test`. A predicate starts one through `acsassert.SubprocessOutput`, through `acsassert.GoTests` or through raw `exec.Command`.
> - **Bound:** the deadline of one predicate subprocess. It is the earliest of the caller deadline, the lane end and `now + subprocessCap` (300 s).
> - **Harness red:** a synthetic red result that the harness makes when a predicate cannot run. It is not a defect of the code.
> - **Lane budget:** the one deadline of the lane. The setting is `acs.go_timeout_s` in `.evolve/policy.json`.

## 1. Request

The console filed this item from the retro autofiler: "Stop recurring failure class audit|gate-block|class-c0e86bae6916 (3 occurrences)". The defect survived earlier fixes, so the blocked-over-three rule applies. Rounds 1 and 2 named a slow host and a lane budget as the cause. The round-2 review found the real cause. This plan redesigns at that cause.

**Acceptance:** a single hung predicate no longer produces `go-lane-incomplete` and `go-lane-scope-failed` harness reds for the other scopes.

This plan does not claim that the class key changes. See F2.6.

## 2. Facts

**F2.1** In cycle 2551, the first red is `egps/go-lane-incomplete/.../cycle1270/TestC1270_003_CmdEvolveNoLongerPaysTheRetryLadder` ("predicate started without a terminal result"). The same test is the first red in cycles 2547 and 2548.

The 89 green results of cycle 2551 sum to 4270 ms (`runtime/.evolve/runs/cycle-2551/acs-verdict.json`). `TestC1270_003` started about 600 s before the verdict, and used about 590 s of the 600 s lane budget. The same test passes in 1.6 to 9.5 s in more than 30 other cycles (2530 to 2569). Source: the round-2 review (`r2-findings.md`, finding 1), 2026-11-21.
*Implication:* one predicate hung. The other 39 reds are a result of it, not a second cause.

**F2.2** `TestC1270_003` calls `go test -run TestLoop_MaxCyclesExit_ClearsCompletedMarker ./cmd/evolve` through `acsassert.SubprocessOutput` (`go/acs/regression/cycle1270/predicates_test.go:23,33`). `SubprocessOutput` runs the command under `context.Background()` and passes no `-timeout` (`go/pkg/acsassert/assertions.go:146-153`). Of the 578 files that call it, none can bound the call.

`acsassert.GoTests` runs `go test` under `callerDeadline(tb)` (`go/pkg/acsassert/go_tests.go:37,55-62`). That deadline is the go test deadline of the scope.

`defaultGoExec` passes no `-timeout` (`go/internal/acssuite/acssuite.go:270-277`), so the scope runs with the Go default of 10 min. This is longer than the 600 s lane.

In the recent current-cycle packages (`go/acs/cycle2400` and later, 107 packages), 69 use `SubprocessOutput`, 13 use `GoTests` and 29 use raw `exec`. 24 of them run `go`.

Three files in the regression scopes start raw processes: `acs/regression/apicover/predicates_test.go:84`, `acs/regression/cycle1515/predicates_test.go:43,85` and `acs/regression/cycle49/predicates_test.go:34`. The current-cycle scope runs first (`goLanePatterns`, `acssuite.go:300-305`). A hang there spends the lane of all 44 regression scopes.
*Implication:* any inner test that blocks holds the whole lane budget. This is the missing bound, and it is the root cause that this plan removes.

**F2.3** The inner test blocked on the disk guard. At the wave-182 base `f25d49e4a`, the cycle-start guard read the real host disk and paused with no bound. `loop.log` shows the host below the 5% pause line while the three audits ran: "free disk space ... is 2.6% (below 5% pause threshold)" (`runtime/.evolve/logs/20261119T213950Z/loop.log:1550`, and lines up to 1829).

Commit `203d65ccc` (PR 1599, 2026-11-21) already removed this trigger, and `origin/main` (`bca876212`) contains it. It stubs `cycleStartDiskUsageFn` at 90% free in the `cmd/evolve` `TestMain` (`go/cmd/evolve/main_test.go:37`). 

It bounds the pause (`defaultDiskGuardMaxPauseSeconds = 1800`, `go/internal/policy/preflight_config.go:12`; `go/internal/looppreflight/cyclestart.go:66`). It adds `TestLoop_MaxCyclesExit_ClearsCompletedMarker_WhenHostDiskIsLow` (`go/cmd/evolve/cmd_cycle_start_disk_test.go:155`).
*Implication:* this plan does not touch the disk guard. The trigger of 2026-11-19 no longer reproduces. The gap that is left is that nothing bounds the next trigger.

**F2.4** The lane budget is one deadline for every scope. `lane.budget = newLaneBudget(...)` (`go/internal/acssuite/golane.go:39`) and the loop at `golane.go:42-44` share it. `TestRun_TheGoLaneBudgetIsOneDeadlineSharedByEveryScope` pins it (`go/internal/acssuite/verdict_always_written_test.go:182`).

[internal-acssuite](../architecture/packages/internal-acssuite.md) states why. The suite holds the host-wide single-flight lock for the whole run. A budget for each scope can hold it far past `maxLockWait`.

A hung predicate used the shared budget, so 38 later scopes got "not run" reds (`golane.go:83`).
*Implication:* the shared budget is correct. The defect is that one predicate can spend all of it. Keep the invariant.

**F2.5** The `acs-durable` gate reports passing lines as offenders. The gate runs `go test ... ./acs/regression/...` under an 8 minute limit (`go/internal/phases/audit/ciparitygate/ciparitygate.go:69`). When the limit kills it, the output has no `FAIL` or `panic:` marker.

`offenderLines` then keeps the last six lines (`offenders.go:41-70`), and these lines are `ok` lines. The offenders of cycle 2551 stop at `regression/cycle103`, the package before `cycle1270`, so the gate hung on the same package.
*Implication:* the diagnostic is wrong, but it is a symptom of F2.2. A bound on the predicate (D1) also ends the hang of this gate. D3 fixes the text.

**F2.6** The class fingerprint is a hash of phase, pre-class, checks and layer (`go/internal/core/failure_class.go:56-59`). It does not include a red id. Every infra-layer audit gate-block without floor checks has the fingerprint `c0e86bae6916`.

The layer comes from the `root_cause` that the retro writes in `disposition.json` (`go/internal/core/disposition_gate.go:20`).
*Implication:* no code change of the lane can make this class stop appearing for other causes. The acceptance is therefore the one in section 1. The class key is a separate question (section 10).

## 3. Goals and non-goals

| Goal | Measure |
|---|---|
| G1: a hung predicate subprocess ends inside a bound and becomes one named red that is not retried | A test with a blocking fake `go` returns inside the bound with a nonzero code, for `SubprocessOutput` and for `GoTests`. |
| G2: the bound comes from the shared lane budget | The suite exports the lane end to the predicate. The bound never passes it. |
| G3: the acs-durable offenders never list passing lines | 0 `ok` or `?` lines in the offenders of a markerless failure. |
| G4: no new setting, no flag, no operator value from the environment | `git diff` of `.evolve/policy.json` is empty. |

Non-goals: no change of the lane budget, of the single-flight lock, of the scope list or of the disk guard. No new result id. A real red still blocks the ship.

## 4. Candidate designs

| Id | Design | Why it fails or wins |
|---|---|---|
| A | Raise `acs.go_timeout_s`. | Rejected: a hung predicate still spends all of it. This is the "bigger constant" fix that failed before. |
| B | A budget for each scope. | Rejected: it reverses the lock-hold invariant of F2.4. Alternating slow and passing scopes never trip a breaker. |
| C | Stub the disk guard in more tests. | Rejected as the main fix: it removes one trigger. The next blocking test hangs the lane again. Kept as a follow-up check (section 10). |
| D | Bound each predicate subprocess in `acsassert`. The bound is the earliest of the caller deadline, the lane end and a fixed cap. The inner `go test` gets a matching `-timeout`. | Wins. One home, no new setting, the lane budget stays shared, and a hang becomes one red with the Go goroutine dump. |

| Criterion | A | B | C | D |
|---|---|---|---|---|
| Removes the root cause (F2.2) | 1 | 1 | 1 | 3 |
| Keeps the shared-budget invariant | 3 | 1 | 3 | 3 |
| One home for the rule | 3 | 2 | 1 | 3 |
| Diff size | 3 | 1 | 2 | 3 |

*Implication:* design D is the only one that bounds the cause without a new lock-hold risk.

## 5. Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | One bound helper in `go/pkg/acsassert` returns `min(caller deadline if set, lane end, now + subprocessCap)`. `SubprocessOutput` and `GoTests` both call it, and in `GoTests` it replaces `callerDeadline` at `go_tests.go:37`. `subprocessCap` is a variable seam with the value 300 s. A bound hit returns `ErrSubprocessBound` and prints a line with `acsverdict.SubprocessBoundMarker` to stderr. `isRetryable` returns false for a scope that holds that marker. A bound that is already past starts no process. | F2.1, F2.2 | Environment value for the cap: the house rule forbids a control value from the environment. A policy key: a new setting that the evidence does not need (section 10). A cap of 120 s: the largest green predicate subprocess took 158 s (cycle 2492), so a 120 s cap gives a false red. A cap above 300 s: see the arithmetic below. A retry of a bound hit: the hang repeats, so the retry spends the lane again. |
| D2 | When the command is `go test` and has no `-timeout` argument, `SubprocessOutput` inserts `-timeout <bound minus one tenth>` after `test`. The helper finds the `test` subcommand after the global flags of `go`, such as `go -C dir test` (`go/acs/cycle2299/predicates_test.go:45`). It reuses `isGoTest` and `hasFlag` of `go/internal/lanebudget/lanebudget.go`, or a shared helper that both packages call, so `go test` arguments have one rewriter. Go then prints "panic: test timed out" with the goroutine dump before the context kills the process. | F2.2 | Only the context kill: the process dies with no dump, so the red names no cause. |
| D3 | The suite exports the lane end to predicates, as one more harness export. The key is declared once in `go/internal/ipcenv/ipcenv.go` with the `// SSOT IPC-protocol-allowed` marker. It joins `ProtocolKeys()` and the markers list, so `forwardableKeys` refuses it as an operator key. `ipcenv.Scrub` drops a stale host copy, so the key needs no row in `suiteExportKeys`; `acssuite` and `acsassert` both use the `ipcenv` constant. The guards `TestNoControlFlagReadOutsideTheMarkerList` (`go/internal/guards/control_flags_test.go:357`), `TestEveryGoReaderHasRegistryRow` and `flagreaders` then pass. `runGoTest` creates the budget before `predicateEnv`, so the value is the real deadline. | F2.4 | A second deadline in `acsassert`: the lane then has two clocks. |
| D4 | `offenderLines` does not fall back to lines that start with `ok` or `?`. When no line is left, it returns one fixed line: "no failure marker in the output; the command exited non-zero". | F2.5 | A typed deadline cause in the gate vocabulary: more surface, and the cause is not needed once D1 removes the hang. See section 10. |
| D5 | The lane budget, the scope list and the verdict ids do not change. | F2.4 | Any per-scope budget: see design B. |

A signalled process gives code -1, and `cycle1270` reports that as a launch failure. The sentinel keeps the red message true.

Cap sizing: the largest measured green predicate subprocess is 158 s (cycle 2492). The single call `TestC2507_009_GcPackageTestsPass` took 76.5 s (cycle 2507). Other green predicates took 58 to 70 s (cycles 2250, 2252, 2299, 2322). The inner `-timeout` is 0.9 times the remaining bound, so a cap of 300 s gives 270 s and leaves room on a loaded host.

A bound hit is not retried. The marker in the red evidence makes `isRetryable` return false, so `retryFlakyReds` does not run the scope again. A hang therefore costs one cap.

The cap must satisfy `cap + largest summed lane time <= acs.go_timeout_s`. The largest summed lane time in the recent verdicts is 187 s (cycle 2552). With a cap of 300 s, 300 + 187 = 487 s, which is below 600 s. A cap above 413 s breaks this inequality.

Consequence of D1: a predicate that needs more than 300 s for a legitimate subprocess now fails. Condition to change D1: a green predicate subprocess that takes more than 200 s in any `runtime/.evolve/runs/cycle-*/acs-verdict.json` (all cycles, current-cycle predicates included). Then the cap moves to `policy.json` as its own key (section 10), and the inequality is checked again.

Consequence of D1 for a hang: the cost is one cap, 300 s of the lane, not 590 s. A second hang in the same lane still spends the rest of it.

### Worked example

Cycle 2551 shape: the lane has a 600 s budget. `TestC1270_003` starts an inner `go test` that blocks. At 270 s Go panics with "test timed out" (D2). `SubprocessOutput` returns a nonzero code and `ErrSubprocessBound`.

`assertNamedTestsPass` reports `TestLoop_MaxCyclesExit_ClearsCompletedMarker did NOT pass` with the goroutine dump. The red carries the bound marker, so the scope is not retried. The lane has used about 300 s plus the other predicates of the scope. The other 43 scopes run to their own results.

## 6. Components and files

| Id | Component | File |
|---|---|---|
| C1 | One bound helper, inner `-timeout`, `ErrSubprocessBound`, a cap seam (a package var) | `go/pkg/acsassert/assertions.go`, `go/pkg/acsassert/go_tests.go` (new file `go/pkg/acsassert/subprocess_bound.go` for the helpers), `go/internal/lanebudget/lanebudget.go` (`WithTimeout`), `go/internal/acsverdict/acsverdict.go` (the marker), `go/internal/acssuite/golane.go` (`isRetryable`) |
| C2 | Lane end key, budget created before `predicateEnv` | `go/internal/ipcenv/ipcenv.go`, `go/internal/acssuite/golane.go`, `go/internal/acssuite/acssuite.go` |
| C3 | No `ok` fallback and the fixed line | `go/internal/phases/audit/ciparitygate/offenders.go` |
| C4 | Docs: bound, export and offender text | `docs/architecture/packages/pkg-acsassert.md`, `docs/architecture/packages/internal-acssuite.md` (lines 14-15 and 42, the export list), `docs/architecture/acs-predicate-quality-gate.md:210` (one bound rule for both paths), `docs/architecture/packages/internal-phases-audit-ciparitygate.md:46` (the fallback text), the `ipcenv` key documentation |
| C5 | The three raw-exec sites of the regression scopes go through `SubprocessOutput` | `go/acs/regression/apicover/predicates_test.go:84`, `go/acs/regression/cycle1515/predicates_test.go:43,85`, `go/acs/regression/cycle49/predicates_test.go:34` |
| C6 | An advisory (shadow) flaky-shape row "subprocess started outside acsassert" for new cycle predicates. `SubprocessOutput` and `GoTests` are the allowed shapes. | `go/internal/evalqualitycheck/flakylint.go` (`LintFlakyPredicates`) |

`pkg/acsassert`, `ipcenv` and `acssuite/` are protected paths (`ipcenv` is Pillar-1), so the console flow builds this. `.evolve/policy.json` does not change.

## 7. Tests

The tests 1 to 3 and 5 to 7 are red today. The build must record that run.

| Name | Package | What it proves | Red today? | Goal |
|---|---|---|---|---|
| `TestSubprocessOutput_ABlockingCommandReturnsInsideTheBound` | `acsassert` | A fake `go` on `PATH` blocks, and the script uses `exec sleep`, so the signal reaches the process that holds the pipe. A lane end of `now + 1 s` is set in the environment. The call returns inside the bound plus `sysexec.WaitDelay` (5 s) plus slack with a nonzero code and `ErrSubprocessBound`. A second case sets a lane end in the past (bound at most 0): the call returns without starting the process. The test runs the call in a goroutine with a 15 s guard, so a missing bound fails the test and does not hang it. | Yes: the call never returns. | G1, G2 |
| `TestSubprocessOutput_AGoTestGetsATimeoutBelowTheBound` | `acsassert` | The fake `go` records its arguments. They hold `-timeout` after `test`, with a value below the bound. A caller `-timeout` stays unchanged. The case `go -C dir test` also gets the `-timeout`. | Yes: no `-timeout` today. | G1 |
| `TestRunGoTest_PredicatesReceiveTheLaneEnd` | `acssuite` | The env that the fake `GoExec` receives holds the export key. Its value equals the budget deadline of the lane. A host copy of the key does not reach the predicate. | Yes: no export today. | G2 |
| `TestOffenderLines_FallbackSkipsPassingPackages` | `ciparitygate` | Output of six `ok` lines gives the fixed line. A marker line still wins. | Yes: the tail is returned today. | G3 |
| `TestGoTests_ABlockingGoTestReturnsInsideTheBound` | `acsassert` | A fake `go` blocks inside `GoTests`. The call returns inside the bound, and `resolveImportPath` uses the same bound. | Yes: it uses the go test deadline of the scope today. | G1, G2 |
| `TestRunGoTest_AHungPredicateLeavesTheOtherScopesGreen` | `acssuite` | A `runGoTest` test uses a fake `GoExec`. Scope 1 sets the `ipcenv` key to one second ahead with `t.Setenv`, like a cap that is below the lane end. It calls `acsassert.SubprocessOutput` on a blocking fake `go` (lane 10 s). The scope runs once, because a bound hit is not retried. Scope 2 emits a pass. There are no `egps/go-lane-*` reds, scope 2 is green and scope 1 has one named red. | Yes: `SubprocessOutput` blocks until the lane is spent. | G1, G2 |
| `TestSubprocessOutput_ABoundHitNamesTheBound` | `acsassert` | The error wraps `ErrSubprocessBound`, and the stderr carries the marker line. The cases are a deadline kill, an inner Go timeout and the cap. | Yes: the sentinel does not exist. | G1 |
| `TestIsRetryable_ABoundRedBlocksTheRetryOfItsWholeScope` | `acssuite` | A scope with a red that carries the marker is not retried, also beside an ordinary red. | Yes: the marker does not exist. | G1 |

Guard tests, which pass today and stay green: `TestRun_TheGoLaneBudgetIsOneDeadlineSharedByEveryScope` (D5), `TestOffenderLines_MarkersFallbackAndCap`.

Limit of the tests: no single test goes from a hung predicate through `evolve acs run` to the verdict. `cmd_acs.go:163` passes no `GoExec`, and a real lane compiles the whole module. The composition test `TestRunGoTest_AHungPredicateLeavesTheOtherScopesGreen` is the regression test at the CUJ level, through `runGoTest`.

The other tests prove the parts of the chain: the bound in `acsassert`, and the export that feeds it. They use a fake `go` on `PATH` as `go/internal/acssuite/lanebudget_test.go:15-31` does. They use a lane end of 1 s and no real long sleep. They use no `t.Context`, so they run on Go 1.23.

## 8. Rollout

Stage: enforce at merge, with no flag. A predicate that finishes inside the bound behaves as before. Evidence before the merge:

1. The red run of the new tests on `origin/main`, saved in the build report.
2. The green run after the fix, with every new test passing and the guard tests green.
3. Predicate durations from all `runtime/.evolve/runs/cycle-*/acs-verdict.json` files, current-cycle predicates included. No green predicate exceeds 200 s, and the inequality of section 5 holds.
4. After the first wave on the merged binary, the console reads the incoming signals. It does not poll `acs-verdict.json`. A predicate that hits the bound shows as a named red, which the audit already reports as a failed gate.

## 9. Risks

| Risk | Control |
|---|---|
| A legitimate slow `go test` hits the 300 s cap and turns red. | Step 3 of the rollout measures every predicate of every cycle. The change condition in section 5 names the next step. |
| A predicate starts a raw process outside `acsassert`. The gap is 24 of 107 recent packages that run `go` through raw `exec`. Gate D is advisory. | C5 moves the three sites of the regression scopes. C6 adds an advisory row for new predicates. Follow-up 6 moves it to enforce. |
| A grandchild such as `go run` or the evolve binary outlives the SIGTERM and `WaitDelay`. | Accepted limit. A process-group kill needs an injected signal sender (the kill-guard rule), so this plan adds none. |
| The 90% inner `-timeout` kills a test that finishes at 95% of the bound. | The red names the test and carries the dump, so a person sees the cause in one read. |
| `offenderLines` has two more callers: `apicover.go:137` and `tierlog.go:44`. They receive the fixed line when the output has only passing lines. | `failed` reads `offenders[0]` (`go/internal/phases/audit/ciparitygate/ciparitygate.go:138`), so the fixed line also keeps the list non-empty. `TestOffenderLines_MarkersFallbackAndCap` stays green. |
| A host copy of the lane-end key reaches a predicate. | `ipcenv.Scrub` drops every `EVOLVE_` key from the host environment, and the suite adds the lane end after it. Test 3 proves it. |
| Another test blocks on something other than a subprocess. | Out of scope: `go test` of the predicate package has its own default 10 minute limit. The lane budget still ends it. |

## 10. Follow-ups (not in this build)

1. **Residual disk-guard hazard.** `NewDiskGuard` falls back to the real `DiskUsageOf` when `usage` is nil (`go/internal/looppreflight/disk_percent.go:48-49`). A test package other than `cmd/evolve` can reach the cycle start without a stub and pause for 1800 s. File an item for a check that lists such packages. D1 limits the cost for a predicate to the cap.
2. **Class key.** Ask the operator if the class fingerprint (F2.6) must include the dominant red id or cause. File a separate item. This plan does not change the fingerprint.
3. **Cap as a setting.** If the change condition of D1 occurs, add a separately named policy key for the cap. Do not change the meaning of `acs.go_timeout_s`.
4. **Gate deadline vocabulary.** If the `acs-durable` timeout needs a named cause, add one typed cause to `vocabulary.go`. Test it with the `killedAtDeadline` fake (`helpers_test.go:152`), which the package doc requires. The rule must match the tier gate and `apicover`.
5. **The 8 minute `ACSDurable` constant** (`ciparitygate.go:69`) is a separate item with its own home.
6. **Flaky-shape row to enforce.** C6 starts as an advisory (shadow) row. File an item to move it to enforce after one wave of evidence.
7. **Fleet load.** The plan [fleet-host-adaptive-window-2026-11](fleet-host-adaptive-window-2026-11.md) owns host load. It is not a cause of this item.
