# Plan: the main-red stop opens only on a key that every prover of the stop can run

- **Status:** built, 2026-11-21 (revision 4, after the third design review). This plan returns PR #1524 (the linear rebuild of #1488) to design.
- **Spec of record:** [gate-contract-g4-base-red-attribution](../architecture/gate-contract-g4-base-red-attribution.md), sections 7.5 and 7.6. **Package notes:** [internal-ciparity](../architecture/packages/internal-ciparity.md), [internal-phases-ship](../architecture/packages/internal-phases-ship.md), [internal-mainstop](../architecture/packages/internal-mainstop.md).
- **Related:** [train-red-batch-attribution-2026-11](train-red-batch-attribution-2026-11.md) (the train refuses on a red main) and [shadow-stage-evidence-2026-11](shadow-stage-evidence-2026-11.md) (the shadow evidence rules).
- **Spec to rewrite:** [shadow-stage-contract](../architecture/shadow-stage-contract.md), section F3 and incident I3.

## Table of contents

- [1. Request](#1-request)
- [2. Terms used throughout](#2-terms-used-throughout)
- [3. Facts](#3-facts)
- [4. Root cause](#4-root-cause)
- [5. Candidate designs](#5-candidate-designs)
- [6. Decisions](#6-decisions)
- [7. Files to change](#7-files-to-change)
- [8. Red tests](#8-red-tests)
- [9. Rollout](#9-rollout)
- [10. Risks and limits](#10-risks-and-limits)
- [11. Questions](#11-questions)
- [12. Follow-up items](#12-follow-up-items)

## 1. Request

The inbox item says: "The main-red stop opens on a coverage-gate key that no lane can prove (PR #1524)." Its fix is: "give each ciparity.GateStep one home for whether a lane can prove it ... provable and GateStepRunner both read it, so a coverage-gate red emits SHIP_MAIN_RED_CI_UNPROVABLE and opens no stop."

The three acceptance criteria of the item, as this plan words them (section 7, pre-build step 2, edits the item to match):

- **A1.** `provable` has one home. It is true only for a key that every prover of the stop can run. The test is red before the change.
- **A2.** A main red at a coverage gate emits `SHIP_MAIN_RED_CI_UNPROVABLE` and opens no stop.
- **A3.** The stop and the signal use the one `Gate` vocabulary that `ciwatch` writes.

PR #1524 was blocked more than three times. The rule is: redesign from the architecture at its source, with a guard.

**Revision 2** kept step keys out of the stop, because the train cannot run them (F3.9). **Revision 3** applies the same rule to a second kind of bad key: a test key that no prover can run (F3.10, F3.11). It also cuts the step-key code that nothing reads after revision 2 (D3).

## 2. Terms used throughout

- **Step row:** a `ciwatch.FailingTest` with an empty `Test`. Its `Step` field holds the `Gate` name of a red CI step, for example `make cover-changed`. It names a red CI step, not a red test.
- **Gate row:** one row of `ciparity.GateSteps`. It joins a CI job and step to the `make` command that runs it.
- **Prover of the stop:** code that decides if a tree turns the keys of a stop green. There are two. The ship lane runs `stopCheck.proof`. The train runs `selfheal.StopHold.Admit`.
- **Provable key:** a key that every prover of the stop can run. It must also be red at main when run as the provers run it (D1).
- **Main-red stop:** the record `main-red-stop.json`. While it holds, a lane or a batch must prove that it turns each key green.

## 3. Facts

**F3.1** On `origin/main` the files `gate_step_runner.go` and `main_red_ci.go` do not exist. The identifiers `provable` and `GateStepOfKey` have no match in the `.go` files under `go/`. Command: `grep -rn "GateStepOfKey" go --include=*.go` in `dev/cl-b027`, 2026-11-21. (The word also matches data files in `go/internal/inboxcurator/testdata/wave178/bounced/`; they are not code.)

The PR code lives in two commits, `f1f84af64` and `35b0a885b`, on the branch `console/f3-shadow-contract-r3` in the worktree `dev/cl-1488r3` (`git log` there, 2026-11-21). The worktree `dev/cl-1524bind` is a detached `HEAD` at `2752dfd0f`. Its `Gate`-only `GateStepOfKey` exists only as a staged index that no commit holds (`git status` there shows staged files). A recycle or `gc` step can lose an uncommitted index.

*Implication:* the defect lives only in the unmerged PR #1524. The design must fix it before the build, not patch main. The build carries a part of the two commits (section 7). This design does not need `GateStepOfKey`, so the staged index is a safety copy only (pre-build step 1).

**F3.2** Main proves a stop through `attr.RunIn(ctx, tree, mainstop.ProofReds(c.stop.Keys))` at `go/internal/phases/ship/main_red_stop.go:58`. `RunIn` runs `go test` for the keys in the lane tree (`go/internal/baseattr/attribute.go:94`). `ProofReds` passes `covchanged.TagFlags(covchanged.DefaultTags)` only (`go/internal/mainstop/proof.go:9`).

*Implication:* today every stop key is a test key, and `go test` can run it. A step key has no `go test` form.

**F3.3** Main has five `GateSteps` rows (`go/internal/ciparity/gatesteps.go:15-21`): `make cover-changed`, `make apicover-check`, `make cover-strict`, `make cover-changed-darwin` and `make cover-strict-darwin`. Only `ciwatch` reads them (`go/internal/ciwatch/gatejoin.go:40`). `gateRow` writes `FailingTest.Step = gate.Gate`, and `Detail` with the hint `gh run view --job ... --log-failed | tail -n <Bound>` (`gatejoin.go:51-56`).

*Implication:* the key vocabulary that CI produces is the `Gate` string, and the row already holds the hint for the signal. No second lookup is needed.

**F3.4** In `go/Makefile`, the target `test-integration` (line 133) removes `coverage.txt` first (line 134). It writes the file only after a green run. Four recipes read that file through the `evolve cover` verb, and none of them names the file:

- `cover-strict` (recipe at line 241) runs `$(GO) run ./cmd/evolve cover strict --root .`.
- `cover-changed` (recipe at line 247) runs `... cover changed --base ...`.
- `cover-changed-darwin` (recipe at line 250) runs `... cover changed ... --packages ...`.
- `cover-strict-darwin` (recipe at line 253) runs `... cover strict ... --packages ...`.

Only the `##` help text of each target names `coverage.txt`. `apicover-check` (line 269) reads the same file through `go tool cover`.

*Implication:* a ship lane never makes that file. The five rows fail with exit 2 in a lane, with or without the fix. The gate cannot tell a fixed tree from a broken tree.

**F3.5** PR #1524 defines `provable` as "`GateStepOfKey(k.Step)` finds a row" (`gate_step_runner.go`, `provable`). `GateStepRunner` runs `strings.Fields(row.Gate)` in `<tree>/go` for every row.

*Implication:* a row is "provable" when it exists, not when a prover can run it.

**F3.6** The PR added a sixth row, `{"golangci-lint run", "golangci-lint", "Lint", 200}`. The workflow `.github/workflows/required.yml:107-122` runs `golangci-lint run` as its own job. It installs `golangci-lint@v$(cat go/.golangci-version)` (line 119; the pin is `2.14.0`) on ubuntu.

*Implication:* a lane can run the command, but not as CI runs it. Section 10 lists the five gaps. They are the reason that lint proof needs its own item.

**F3.7** Failure path in the PR: main goes red at a key that no prover can run and the stop opens. The fix lane runs the gate and gets exit 2. `Admit` refuses the fix as "not a fix". At shadow, each lane gets a false `WOULD_REFUSE`. At enforce, every lane parks until `MainRedStopBound` (default 120 minutes, `go/internal/policy/routing_gates.go:200`).

*Implication:* a false stop is a fleet-wide deadlock. A missed stop costs less, because the train already refuses on a red main (see [train plan](train-red-batch-attribution-2026-11.md), F1 and D3; `go/internal/integration/mainci.go:35-51`).

**F3.8** Outside source. The tool `act` runs a workflow locally. It does not support every feature. Steps that call back to GitHub need extra setup ([Better Stack guide to act](https://betterstack.com/community/guides/scaling-docker/act-github-actions-tutorial/)). The same search found no standard way to prove a CI step outside CI. Synthesis: the only sound proof is a command that the repository declares for that step, with its inputs.

*Implication:* we do not guess a proof command from the workflow file.

**F3.9** The train is a second prover, and it runs only `go test`. `selfheal.StopHold.Admit` (`go/internal/selfheal/main_red_hold.go:50-54`) runs `h.Run(ctx, tip, mainstop.ProofReds(stop.Keys))`. The command `cmd/evolve` wires `Run` to `core.BaseRedAttributor(...).RunAt` (`go/cmd/evolve/cmd_integration_stop.go:27-31` and `79-81`). A held lint key makes the `--main-red-fix` batch get `Green=[]`, and `Admit` returns `RefuseNotAFix`. Main stays red until `MainRedStopBound`.

*Implication:* "provable" must hold for every prover of the stop. A key that only the ship lane can run is still a false stop.

**F3.10** A CI test row can have an empty `Package`. `addPending` emits `FailingTest{Test: t}` (`go/internal/ciwatch/classify_source.go:398-401`), and `parseFailingTests` (`:408`) and `attributeFailures` (`:415`) call it. This happens when a job log has a `--- FAIL` line and no `FAIL<TAB><pkg>` line. The carried `ciKey` (`main_red_ci.go:130` on `console/f3-shadow-contract-r3`) turns that row into `Key{Package: "", Test: t}`. `baseattr.packagesOf` then passes `""` to `go test`.

In the round 2 review, `go test -tags integration -run '^(TestX)$' ""` printed `FAIL<TAB>. [setup failed]` and exited 1. `stopCheck.proof` (`main_red_stop.go:58-62`) and `StopHold.Admit` (`main_red_hold.go:50-54`) turn that error into "not a proven fix".

*Implication:* a test key with no package is a key that no prover can run. It deadlocks the fleet, as F3.7 says. "A test key" is not enough for "provable".

**F3.11** CI runs the suite with `-race` (`INTEGRATION_TEST`, `go/Makefile:131`). `ProofReds` does not (F3.2). A red that shows only under `-race`, or a flaky red, runs green on an unfixed tree. Any lane then proves "green", the stop releases, and a new stop opens on the next main SHA.

*Implication:* a key is provable only if the proof command reproduces it red at main. D1 checks this before the stop opens.

## 4. Root cause

The belief "this key can be proven" has several homes today, and each is wrong in a different way. `provable` answers "a row exists". `GateStepRunner` answers "I can start the command". The train answers "a `go test` can run it". Nobody records the one real fact: every prover of the stop can run the key, and it is red when run that way.

The input of five rows is a file that only one CI job makes. The input of the lint row is a pinned binary on one OS, which no prover owns. The input of a package-less test row is a package name that CI did not give.

## 5. Candidate designs

| Criterion | A. `LaneProof` field on `GateStep` (first design) | B. Run `make test-integration` in the lane, then each gate | C. Keep a separate allow-list in `ship` | D. Only a test key with a package that reproduces at main enters the stop; every other red emits the signal |
|---|---|---|---|---|
| A1: provable equals runnable by every prover | No: the train cannot run a step key (F3.9) | Not for the train | No, a second home can drift | Yes: the same `go test` run that the provers use decides it |
| A2: coverage red opens no stop | Yes | No | Yes, if the list is right | Yes |
| A3: one key vocabulary | Yes, `Gate` only | Yes | Needs a second rule | Yes, the `Gate` string that `ciwatch` writes |
| One home per rule | Yes | Yes | No | Yes: `provable` in `main_red_ci.go` |
| No feature flag, no env control value | Yes | Yes | Yes | Yes |
| Protected surface | `ciparity`, `ship`, `selfheal`, `cmd/evolve` | Adds a long run to ship | `ship` only | `ship` only, plus rows in section 7 |
| CI parity (Linux, Go 1.23, `-race`) | Not met on a lane host (section 10) | Race and coverage differ on a lane host | Same as A | Not needed for steps. A test red that does not reproduce is not held (D1) |
| Cost per proof | One command per row, with five open gaps | One full suite per proof | Same as A | One `go test` run of the CI keys at main, once per SHA |
| Lint red on main | Held, but can deadlock (F3.9) | Held | Held | Not held; signal and fix item |

Why not A: with the train as a second prover, a held lint key deadlocks the fix batch. To make A sound, the lane prover and the train prover must be designed together (section 12, Q2).

Why not B: a lane must run the full suite for each proof. The load of a live wave (one full suite at a time) makes that unsafe. This plan cites no measured wall time for the suite, so the rejection rests on the missing input. Why not C: a list in `ship` repeats the gate set of `ciparity`, which breaks "one home".

Why D is acceptable for lint: it loses the held lint red. The plan accepts that loss for coverage already, because "the train refuses on a red main" (train plan, F1). The signal names the gate and the run URL once for each SHA. A fix item for a red main needs a path that Q3 adds (Q1).

The incident I3 of the [shadow stage contract](../architecture/shadow-stage-contract.md) was a lint red on main for 5 commits. It now gets a signal once for each commit (D5), a `ci-red` item, and the train still refuses. A missed stop costs less than a false stop (F3.7).

## 6. Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | `provable` has one home, `main_red_ci.go`. A row `f` is provable when `f.Package != "" && f.Test != ""`. Before `mainCI` opens a stop, it runs the provable keys at the main SHA with `attr.RunAt(ctx, sha, mainstop.ProofReds(keys))`, the call that the train uses. Only keys that come back red enter the stop. All other rows go to the signal. When `RunAt` fails, every provable row goes to the signal with the error in `fields.error`, and no stop opens (fail closed). | F3.2, F3.9, F3.10, F3.11 | Check the package only: it misses a red that shows only under `-race`. Add `-race` to `ProofReds`: it changes the cost of each proof for every lane and needs its own item (section 12, Q5). |
| D2 | `mainCI` splits `[]ciwatch.FailingTest` directly. Provable rows become `baseattr.Key{Package, Test}`. Other rows become plain names for the signal: `f.Step`, then `f.Test`, then `f.Package`, then the job names. The signal reads `Step` and `Detail` of the row. | F3.3 | Keep `GateStepOfKey`: no code reads the row it finds, because `Step` already holds the `Gate` name and `Detail` holds the hint. |
| D3 | Add no step key and no new code in `baseattr` and `mainstop`. Add no `GateStep` field, no lint row, no `GateStepRunner`, no `StepRunner`, no `Options.stepRun`, no `Config.StepRun`, no `GateStepOfKey`. Drop `mainCI.greened`. | F3.9, section 5 | Carry `Key.Step`: after D1 no stop holds a step key, so the field only widens a persisted type. Without the field the type cannot hold a step key, so no guard is needed. |
| D4 | In `mainCI.opened`, every row that D1 does not accept goes to `SHIP_MAIN_RED_CI_UNPROVABLE` and out of the stop. A red run with only such rows opens no stop. A red run with no parsed row emits the signal with the keys `(none parsed)` and the run URL. | A2, F3.7 | Open the stop with a reduced key set and no signal: the operator sees no cause. Stay silent on an empty parse: a red main passes unseen. |
| D5 | The signal is emitted once for each main SHA. A small record beside `main-red-stop.json` keeps the last SHA that was judged. The order is claim, then judge (see "The order of D5" below). The text of the signal says: "run the gate on the CI artifact; the train refuses on the red main until it is green". | F3.7, I3 | Emit at each ship: with N lanes this repeats the same warning until main is green, and `signalcenter` has no dedupe. A record with no lock: two lanes both emit. |
| D6 | Four guards (section 8). T1: the table pin of `provable`, and a `ciwatch` pin of the shape of `gateRow`. T2: no stop that `mainCI` writes holds a key that a prover cannot run. T3: the train still admits as before. T4: the ship phase parks no lane on a step red. | A1, rule "a guard with the redesign" | A comment: a comment does not fail when a row changes. |
| D7 | The three `RegisterCode` texts of the carried code change to the D4 and D5 text, and so do their rows in `signal-codes.md`. The carried text of `UNPROVABLE` says "add a `GateSteps` row for the step" and the text of `OPENED` says "`step:<name>` of a step". Both are false after D1. `MainCIFetcher` reports the error of `goModulePath` (a `WARN`) and does not drop it. | F3.3 | Carry the texts: they tell the operator to add a row that D3 does not use. |

**The order of D5.** `mainCI` reads the record right after `Resolve` and skips the fetch of a judged SHA. For a red completed run, it claims the SHA under the `mainstop` file lock (`flock.PathLock`, as `go/internal/mainstop/record.go:40` does). It then releases the lock at once. It runs `RunAt` and `OpenMainRedStop` outside the lock, because `OpenMainRedStop` takes the lock again and the lock is not reentrant.

A lane that ships during the `RunAt` of the claimant goes on with no stop. The train still refuses the red main. The SHA stays judged after a `RunAt` error, so N lanes do not each wait for the run.

A failure to open the stop is different. Then `mainCI` releases the record, so the next lane judges the SHA again. No stop holds the red. A stop that holds with `MainSHA` equal to the main tip also marks the SHA as judged. Thus a release does not reopen the stop on that SHA.

Consequence, good: a stop never holds a key that a prover cannot run or that does not reproduce. The false-refusal deadlock of F3.7, F3.9 and F3.10 cannot occur. The type of the key cannot hold a step.

Consequence, bad: a coverage red or a lint red on main opens no stop and no lane parks. The train refuses on a red main, so no bad merge lands. A red test that does not reproduce without `-race` is also not held. The item that adds lint proof (section 12, Q2) changes the first. Q5 changes the second.

Reasoning chain: F3.9 says the stop has two provers and the train runs `go test` only, so D1 and D3 keep step keys out. F3.10 and F3.11 say a test key can still fail to run or to reproduce, so D1 runs the key at main first. F3.7 says a false stop costs more than a missed stop, so D4 fails closed to "no stop". D5 makes the missed stop visible once, not many times.

## 7. Files to change

**Pre-build steps.** The console does these before the build starts. Each one is an operator action at the boundary, because a mid-wave inbox edit is refused.

1. Commit the staged index of `dev/cl-1524bind` to the salvage ref `refs/archive/salvage/1524bind`, and cite its SHA in the build brief. The build base is the two commits `f1f84af64` and `35b0a885b` on `console/f3-shadow-contract-r3`.
2. Edit the inbox item `main-red-stop-provable-means-a-lane-can-prove-it`. Set `acceptance[0]` to A1 of section 1. Set A3 to the text of section 1. In `files`, drop `go/internal/phases/ship/gate_step_runner.go`, `go/internal/ciparity/gatesteps.go` and `docs/architecture/packages/internal-ciparity.md`, and add `go/internal/phases/ship/main_red_stop.go`, `go/internal/phases/ship/main_red_ci.go` and `go/internal/mainstop/record.go`. Cite this plan.
3. File the follow-up items of section 12 (Q2, Q3 and Q5) and write their ids into section 12.

**Carried base.** Carry from the two commits the files that this design keeps. The build starts from `git show --stat` of both commits, then removes what D1 and D3 drop. Each row says "carry", "carry and change", "new" or "drop".

| File | Action |
|---|---|
| `go/internal/baseattr/baseattr.go`, `debug.go` and their four tests | Drop. No change to `baseattr`: no `Key.Step`, no `StepKey`, no `IsStep`, no `TestKeys`, no `Names` change. |
| `go/internal/mainstop/mainstop.go`, `proof.go` and their tests | Drop the schema v2 with its v1 migration, `StepKeys`, `Proven` and `ProofReds` changes. Stop files keep schema v1 and test keys only. |
| `go/internal/mainstop/record.go` and its test | Add one record type for the once-per-SHA record of D5, read and set under the existing lock. |
| `go/internal/ciparity/gatesteps.go`, `gatesteps_test.go` | No change. `GateSteps` keeps five rows. |
| `go/internal/phases/ship/main_red_ci.go` (new) | Carry `mainCI` and `opened`. Change: `provable` (D1), the `RunAt` check at main, the split of D2, the record of D5, the empty-parse signal of D4. Drop `greened`. Rewrite the three `RegisterCode` texts (D7). |
| `go/internal/phases/ship/main_red_stop.go` | Carry the `mainCI.sync` call. Drop the step runner branch of `stopCheck.proof`. |
| `go/internal/phases/ship/gate_step_runner.go` and its test | Drop. |
| `go/internal/phases/ship/native.go`, `ship.go` | Carry `Options.mainCI` and `Config.MainCI`. Drop `Options.stepRun` and `Config.StepRun`. |
| `go/cmd/evolve/cmd_cycle.go`, `go/internal/cli/landcmd/cmd_ci.go` | Carry the `ship.Config` wiring. The verbs of `ci` moved to `landcmd`, so the fetcher is the new export `landcmd.MainCIFetcher(root, warn)`. It reports the error of `goModulePath` as D7 says. |
| `go/cmd/evolve/cmd_cycle_mainci_test.go`, `go/internal/phases/ship/mainci_wired_test.go`, `go/internal/cli/landcmd/cmd_ci_test.go` | Carry the wiring proofs and add the test of `MainCIFetcher`. Drop `ship_steprun_wired_test.go`. |
| `go/cmd/evolve/parallel_serial_tests_test.go` | Carry the one row of the wiring test. |
| `go/internal/core/main_red_stop_open.go` | No change. A step row never reaches it, so `fixItemOf` is not on the path. |
| `go/internal/selfheal/main_red_hold.go`, `go/cmd/evolve/cmd_integration_stop.go` | No change. They stay test-key-only. T3 pins this. |
| `go/internal/guards/integrity_surface.go` | Add `GateGuarantee` rows for `go/internal/phases/ship/main_red_ci.go` and `go/internal/phases/ship/main_red_stop.go` (section 7.1). |
| `go/internal/guards/integrity_surface_gate_files_test.go` | Add the matching rows. |
| `docs/architecture/packages/internal-mainstop.md`, `cmd-evolve.md` | Document the D5 record and the wiring. |
| `docs/architecture/packages/internal-phases-ship.md` | Document the CI source, `provable`, the check at main and the three codes. |
| `docs/architecture/signal-codes.md` | Register the three codes with the D7 text. |
| `docs/architecture/shadow-stage-contract.md` | Rewrite section F3 and the I3 row to D1, D4 and D5, and link this plan. F3 today says `baseattr.Key` gets a `Step` field. It says the stop moves to schema `main-red-stop/v2`. It says a step key is proven by a rerun of the step. All three are false after this plan. |
| `docs/architecture/gate-contract-g4-base-red-attribution.md` | Add a pointer to this plan in sections 7.5 and 7.6. Update section 5 (the dependency table): `ship` now imports `ciwatch`. Update section 9, decision 16, with the rows of 7.1. |

### 7.1 Protected surface

G4 section 9, decision 16, requires a `GateGuarantee` row "in the commit that adds each file". `phases/ship/main_red_stop.go` is not protected today, and `main_red_ci.go` is a new file. Both decide if a stop opens, so a lane that weakened them can hide a red main. The plan adds two rows.

`gatesteps.go` gets no row. After D1 it decides only which gate name a signal shows. A row there sends each change of a bound through the protected review fleet for no guarantee. The change also touches `native.go` and `ship.go`, which are already protected rows (`integrity_surface.go:154-155`).

`ciwatch` is an unprotected input of the stop. A change to its parse can hide a red main from the stop, but the train still refuses on the red main. The plan accepts this and adds no row.

It therefore needs the protected review fleet: full seats, an adversarial seat and a security seat.

## 8. Red tests

Each test enters through the real entry point. For the ship lane that is `checkMainRedStop(ctx, opts, warn)` or the ship phase run, with a fake `Options.mainCI` and an injected CI fetcher. The run at main uses the seam `Options.baseRun`, with a real temporary git repository, because `RunAt` checks out the commit. For the train it is `StopHold.Admit` through the integration stop gate. Run T2 and T4 with `MainRedStopEnforced()` false and true. New code must have 100% coverage.

| Test | Package | What it proves | Criterion |
|---|---|---|---|
| T1 `TestProvable_OnlyAWellFormedTestKey` | `ship` | A pure table pin. Each row of `GateSteps` and each row with `Test` and an empty `Package` is not provable. A row with `Package` and `Test` is provable. On the PR tree this test fails to compile, because `provable` takes a `baseattr.Key` there. The behavioral red for A1 is T2 (a), (b) and (c). `TestGateRow_NamesAStepAndNeverATestOrAPackage` in `ciwatch` pins the shape of `gateRow`. | A1 |
| T2 `TestCheckMainRedStop_UnrunnableRedOpensNoStopAndEmitsOnce` | `ship` | Through `checkMainRedStop`, with the five fixtures listed under this table. Each fixture that is not provable emits one `SHIP_MAIN_RED_CI_UNPROVABLE` and writes no stop. | A2, D5 |
| T3 `TestStopHold_AdmitStaysTestKeyOnly` | `selfheal` (through the integration stop gate) | A regression pin, not a red test for A1. A stop with test keys is admitted and refused as before. The existing tests of `StopHold.Admit` hold this pin, and the build changes none of them. | A1 |
| T4 `TestShipPhase_StepRedParksNoLane` | `ship` | The ship phase run, with a fake CI source that reports a step red on main, parks no lane and lands the lane. | A2 |

The fixtures of T2:

- **(a)** The red key `make cover-changed`: the signal has the `Step` and the run URL.
- **(b)** A log with `--- FAIL` and no package line.
- **(c)** A well-formed key that the run at main reports green (the `-race`-only case).
- **(d)** No parsed row: the signal has `(none parsed)`.
- **(e)** A well-formed key that the run at main reports red: a stop holds only that key, and no unprovable signal is emitted.
- **(f)** The run at main fails with an error: one signal with the error, and no stop.

A second ship or lane on the same SHA emits no second event, because of the D5 record.

A test with four concurrent lanes uses the real `OpenMainRedStop`. A nested lock then hangs the test.

Each of T1, T2 and T4 must fail on a tree without the change, as the rule "every issue gets a regression test through the real entry point" requires. The tests that proved a lint runner are removed with the lint row.

## 9. Rollout

- **Stage: shadow.** The stage is `gates.main_red_stop`, default `shadow`. In shadow, a lane that the stop will refuse emits `GATE_REPO_MAIN_RED_STOP_WOULD_REFUSE` and lands.
- **Precondition: a stage seed.** `gates.main_red_stop` has no row in `gateregistry.stageSeeds` today (`go/internal/gateregistry/seed_stages.go:21-82`). `lookupStageEntry` (`go/cmd/evolve/cmd_stage.go:60-67`) finds only those rows, so `evolve stage verify gates.main_red_stop` ends with "unknown stage key" (`cmd_stage.go:123-126`). The signal `SHIP_MAIN_RED_STOP_SHADOW` is not in the code or in `signal-codes.md`. A seed row is a precondition of any enforce flip: signal, emit sites `checkMainRedStop` and `StopHold.decide`, and fields. It is the follow-up item Q3.
- **Verb:** after Q3 lands, `evolve stage verify gates.main_red_stop` recomputes the verdict from the hash-chained records.
- **Codes the verdict joins:** `GATE_REPO_MAIN_RED_STOP_WOULD_REFUSE`, `SHIP_MAIN_RED_CI_UNPROVABLE` and `SHIP_MAIN_RED_CI_OPENED`.
- **Evidence before enforce:** at least two clean waves (the shadow rule of the contract). A verb must recompute three criteria. First, zero `WOULD_REFUSE` events on a stop key that a later landed lane proved green. Second, zero stops whose keys error in `RunIn` or `RunAt`. Third, one `UNPROVABLE` event for each main SHA, joined to the run URL in the event.
- **Before enforce:** prove that the enforce path is built and wired (the rule "shadow enable needs an enforce path"). T2 and T4 must also run with `MainRedStopEnforced()` true.

## 10. Risks and limits

| Risk | Mitigation |
|---|---|
| A coverage red or a lint red on main opens no stop, so no lane is steered to fix it. | The train refuses on a red main. The signal names the failed gate and the run URL, once for each SHA (D5). The Q3 item adds the path that files a fix item (Q1). |
| A new `GateSteps` row ships. | It cannot open a stop: D1 accepts a row only with a `Package` and a `Test`, and a gate row has neither (T1). |
| A test red that does not reproduce without `-race` is not held. | The signal names it. Q5 decides if `ProofReds` gains `-race`. |
| A flaky test that comes back red in the run at main enters the stop. | D5 limits it to one stop for each SHA. The stop ends when a lane proves the key green. |
| `ciwatch` decides which keys enter a stop, and it has no protected row. A weakened parse can hide a red main from the stop. | The train still refuses through `integration` GhCI, so the loss is low. Section 7.1 notes the input. |
| A later item adds lint proof and brings back F3.9. | The follow-up item (Q2) must meet the five conditions below. T3 stays green only while the train runs the same key. |

**Conditions for lint proof (for the follow-up item, Q2).** Each one is a failure that a held lint key can reach at run time:

1. **Version.** `golangci-lint run` on a lane runs the binary on `PATH`, not the pin of `go/.golangci-version` (2.14.0). A `go/Makefile` target must install and run the pin, as the `vulncheck` target does for govulncheck (the test `TestGoWorkflow_GatesTheSuiteOnAPinnedGovulncheckFromTheMakeTarget`). The CI lint job and the proof must both call that target.
2. **OS.** CI lints on ubuntu and lanes lint on darwin. The tree has `_darwin` and `_linux` file pairs (`internal/proctree`, `internal/events/wake`, `cmd/hostsampler`), so `unused` and type findings can differ. The target sets `GOOS=linux`.
3. **Lock.** Several lanes run lint at once. Without `--allow-serial-runners` a second run exits non-zero and counts as red. `commitgate/lanes.go:47` already passes the flag.
4. **Exit codes.** Only exit 1 means findings. Other non-zero codes mean the tool failed. Reuse the classification of the existing lint runner: exit 1 is red, other codes mean no proof ran.
5. **Budget and missing tool.** The proof runs through `lanebudget.Runner` with `ciparity.CIEnv` and a timeout from policy. Each runtime outcome "could not prove" (missing tool, timeout, tool failure) emits a signal and does not refuse on that key. A missing tool must not give an error that makes `Admit` return `RefuseNotAFix`.

The proof must live in one package that both `ship` and `selfheal` import. `cmd/evolve` wires it into `ship.Config` and `stopWiring.hold`.

A red test must run `StopHold.Admit` on a stop with only the lint key. It runs through the integration stop gate, with a batch that fixes the key, and expects `AdmitFix`.

Limits: the design does not make coverage or lint gates provable. A test cannot prove that a lint run on a lane host equals the CI run.

## 11. Questions

- **Q1 (answered).** Does a step red on main file a fix item? Not automatically. `fileEscalation` runs only from the manual `evolve ci watch` command. No wave path runs it on a push to main, and `integration.checkMain` files no item. The Q3 item adds one: a run of `evolve ci watch --sha` on a push to main, or a fix item for each `UNPROVABLE` SHA.
- **Q2 (answered, filed).** The lint row is a separate item, which designs the lane prover and the train prover together and meets the five conditions of section 10. Its id goes here at pre-build step 3: `<id of the lint-proof item>`.
- **Q3.** Does the operator want the stage seed row and the verb criteria of section 9 before the enforce flip? Default: yes, filed as a follow-up item.
- **Q4.** Does the operator want a cheap lane coverage profile later, so coverage rows can be proven? Default: no, until measured.

## 12. Follow-up items

These items hold the scope that the acceptance does not need. Pre-build step 3 files them.

| Item | Content | Why it is not in this plan |
|---|---|---|
| Q2 | Lint proof for the lane prover and the train prover, with the five conditions of section 10. | F3.9: it needs a design of both provers together. |
| Q3 | The `gates.main_red_stop` stage seed row, the signal `SHIP_MAIN_RED_STOP_SHADOW` and the three criteria of section 9. Also a run of `evolve ci watch --sha` on a push to main, or a fix item for each `UNPROVABLE` SHA (Q1). | No acceptance criterion needs it before the build. It must land before the enforce flip. |
| Q5 | Add `-race` to `ProofReds`, so that a red that shows only under `-race` is held. | It changes the cost and the result of each proof for every lane and for the train. D1 already fails closed on such a red. |
