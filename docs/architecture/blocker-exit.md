# Blocker exit: the host proves a blocker, and the round stops

> plan: [build-blocker-exit-2026-10.md](../plans/build-blocker-exit-2026-10.md) · package notes: [internal-core.md](packages/internal-core.md) · floor: [ADR-0117](adr/0117-pre-handoff-probes-run-the-build-floor.md) · protected surface: [ADR-0064](adr/0064-pipeline-integrity-boundary.md).
> This file is the spec of record for the proven-blocker exit of a correction loop. The plan holds the evidence and the rejected designs.
>
> **Terms used throughout:**
>
> - **floor failure**: one string from the build handoff floor, for one failed check.
> - **blocker claim**: one verdict failure defect that starts with the word `BLOCKER`.
> - **proven blocker**: a set of claims that the host verified, which explains every floor failure.
> - **explainable failure**: a failure kind that a blocker can cause. The caller of the prover decides the kinds.

## Table of contents

1. Goals
2. The claim grammar
3. The proof
4. The loop exit
5. The ending and the console route
6. The floor text
7. Guards
8. Limits

## 1. Goals

| Goal | Measure | Target |
|---|---|---|
| Stop the rounds on a real blocker | build dispatches after a proven claim | 0 more |
| Never trust the claim | false claims that end as a route | 0 |
| Keep the work | worktree and salvage snapshot after the ending | kept |
| Read coverage correctly | a percent below the floor that prints as the floor | never |

The order is the order of the table. The second goal outranks the first: a doubtful claim keeps the rounds.

## 2. The claim grammar

### Issue

In cycle 1946 the builder wrote its diagnosis as prose (plan F2). Prose has no fields that a check can read.

### Gap

The `evolve-verdict` sentinel has a `failure.defects` list of free strings. No defect had a structure.

### Solution

A blocker claim is one defect with whitespace-separated fields:

```text
BLOCKER protected-path <path>[:line] <why>
BLOCKER predicate-conflict <failing test> <guard test> <path>[:line] <why>
```

- The path is relative to the repository, for example `go/internal/core/blocker_breaker.go`.
- The parser removes a `:line` or `:start-end` suffix.
- A defect that does not start with `BLOCKER` is not a claim.
- A claim with too few fields, or an unknown kind, is a refused claim.

Example (cycle 1950):

```text
BLOCKER predicate-conflict TestC1950_001_CoreLeaseBreakerCarryforwardBatchReachesFullCoverage TestC1950_002_BatchAddsNoProductionEdits go/internal/core/blocker_breaker.go:76-77 <why>
```

## 3. The proof

### Issue

LLM self-report is not reliable (plan P1, P2). A lane that can stop the rounds with one sentence can escape every failure.

### Gap

The loop had no check of a claim. It had only the rung budget.

### Solution

The seam is phase-agnostic. It lives in `go/internal/core/blocker_proof.go`:

```go
type BlockerProver interface { Prove(c BlockerCase) BlockerProof }
```

`BlockerCase` holds the phase, the worktree, the predicates at the phase request, the predicates now and the diff of the lane. It also holds the manifest test, the floor failures, the verdict defects, the `Explainable` function and an evidence fault. `NewBlockerProver` returns a table of verifiers, one for each kind.

The prover refuses every claim of a case in these conditions:

- The caller gave no `Explainable` function.
- The caller did not collect all the evidence. For example, `git diff --numstat` failed. An empty diff from a failed command must not read as "the lane changed nothing".
- The predicates now are not equal to the predicates at the phase request. The host reads `go/acs/cycle<N>/*.go` before it dispatches the build. Thus a lane cannot write a predicate that fails on a protected file, or a guard that names a directory.

A `protected-path` claim is proven when all of these are true:

1. A manifest test is wired.
2. The path is a non-test `.go` file that exists in the worktree.
3. The lane did not change the path since the base, and the path is a regular file, not a symlink.
4. The lane removed no lines from a `_test.go` file in the directory of the path. A lane that deletes a test can make its own gap.
5. The manifest test says that the path is protected.

A `predicate-conflict` claim is proven when all of these are true:

1. The two test names are different.
2. The path passes checks 2, 3 and 4 above.
3. Both tests are functions in the predicates at the phase request.
4. A floor failure has `--- FAIL: <failing test>`.
5. No floor failure has `--- FAIL: <guard test>`, so the guard holds.
6. The body of the guard test names the directory of the path.

Then each floor failure must be explained. The prover cuts a failure into items. An item starts at a `--- FAIL: ` line or a `cover strict: ` line. A failure is explained when all of these are true:

1. The `Explainable` function accepts it.
2. It has no `…` mark, so no part of the diagnostic was cut.
3. It names at least one non-test `.go` file.
4. Each item names at least one non-test `.go` file, and each such file is the path of a proven claim.
5. The text before the first item names no other file.

`BlockerProof.Blocked()` is true only when at least one claim is proven and no refusal exists. One refused claim, or one unexplained failure, keeps the rounds.

```text
rejection ──► claims in verdict? ──no──► correction round
                   │yes
                   ▼
            prove each claim ──any refusal──► signal ORCHESTRATOR_BLOCKER_REFUSED ──► correction round
                   │all proven
                   ▼
            one committed item? ──no──► signal ORCHESTRATOR_BLOCKER_REFUSED ──► correction round
                   │yes
                   ▼
            ending: blocked-needs-console
```

## 4. The loop exit

### Issue

`reviewWithCorrections` sent the same failure back three times in cycle 1946 (plan F1).

### Gap

The loop had two exits: approval, and the spent budget.

### Solution

At the top of each pass of the loop, `cycleRun.provenBlocker` builds the case and calls the prover. The build caller is `buildBlockerCase` in `build_blocker_exit.go`:

- It acts only for `PhaseBuild` and only when the review result carries `Failures`.
- It reads the defects from `build-report.md` of the workspace.
- Its `Explainable` function is `buildFloorExplainable`. It accepts the cover-strict failure and a failure of a `./acs/cycle<N>` package. It refuses a plain unit-test failure, because the lane can fix its own tests.

`BuildHandoffFloor.Review` now returns the failure list in `ReviewResult.Failures`. `reviewInputFor` sets `ReviewInput.Protected` from the orchestrator's manifest test.

## 5. The ending and the console route

### Issue

A FAIL after the rounds counted as a code failure, and the work was discarded (plan F1).

### Gap

The build phase had no coded ending. The TDD phase had one (plan F6).

### Solution

`endOnProvenBlocker` does these steps:

1. It records the build outcome as FAIL, with the `BLOCKER_ROUTE` diagnostic first.
2. It sets the termination reason to `blocked-needs-console`.
3. It emits `ORCHESTRATOR_BLOCKER_ROUTED`.
4. It returns `loopBreak`. The cycle ends through `completeCycle`.

The FAIL verdict preserves the worktree and stamps the salvage snapshot. The diagnostic message names each proven claim and quotes the defect of the lane. The subject is the one committed item.

At closeout, `cycleclassify.Classify` reads the coded record. `Disposition.OutranksSentinel` lets `BLOCKER_ROUTE` win over the FAIL sentinel of the lane, because the host proved that claim. `cycleoutcome.routeRefusal` then calls the inbox `RouteConsole` with the message. The item gets `route: console-manual` and no `failure_count` bump.

## 6. The floor text

### Issue

The floor printed `100.0% < 100.0%` for 99.97% (plan F4). It told the lane to "test them back up", which is wrong for a dead branch.

### Gap

`measureFloor` used one decimal, and the cover-strict failure named no file.

### Solution

- `coverstrict.belowFloor` keeps the one-decimal text when it reads below the floor. Otherwise it prints two truncated decimals and the statement count: `99.97% (3412 of 3413 statements) < 100.0%`.
- `blockNoter.notes` lists each uncovered block of a failed package: `uncovered <path>:<start>-<end> (<n> statement(s))`. It shows 12 blocks at most, then a `…` line.
- When a block is in a protected file, a note tells the lane to declare a `BLOCKER protected-path` claim.
- When a block is the `err != nil` return after `json.Marshal` or `json.MarshalIndent`, a note suggests a seam or the removal of the dead branch. This note is a hint. It gates nothing.
- `withProtectedFileNotes` adds the protected-file note to an ACS predicate failure that names a protected file.

## 7. Guards

| Rule | Guard | State |
|---|---|---|
| A false claim never ends as a route | `TestBlockerProver_AClaimThatIsNotRealStillFails`, `TestBuildBlockerExit_AClaimedBlockerThatIsNotRealStillFailsAfterTheRounds` | enforced |
| A proven claim does not excuse other failures | `TestBlockerProver_AProvenClaimDoesNotExcuseAFailureItDoesNotExplain` | enforced |
| A proven blocker spends no round | `TestBuildBlockerExit_AProvenBlockerEndsTheRoundsAndRoutesTheItemToTheConsole` | enforced |
| The route wins over the lane's sentinel | `TestClassify_AHostProvenBlockerOutranksTheLanesOwnFailSentinel` | enforced |
| The item reaches the console | `TestApplyFailure_AProvenBuildBlockerRoutesTheItemConsoleWithTheDiagnosis` | enforced |
| A lane cannot edit the proof | `TestProtectedSurfaceManifest_CoversTheBlockerProof` | enforced |

## 8. Limits

- A `protected-path` proof does not show that the uncovered block is unreachable. A test file can cover a reachable block of a protected file. Thus a lane can claim such a block and end as a route. The task forbids a gate on the dead-branch hint, so the console decides. The work does not ship, and the claim of the lane is in the route reason.

- The proof does not show that a branch is unreachable. It shows that the failure is in a file that the lane must not edit, or between two predicates that conflict. The console decides the rest.
- The route needs exactly one committed item, as the TDD route does.
- Only the build phase calls the prover today. Another phase must supply its own `Explainable` function.
- The ending is not a code FAIL for the halt counters (ADR-0141 decision 10). `completeSequentialCycle` keeps the consecutive-FAIL count as it was, and `core.IsProvenBlockerResult` decides it. The batch breaker counts only cycles with a `failure-digest.json`, and a blocked cycle writes none. The cycle still counts as no ship for the zero-ship rule. The TDD protected route still counts as a FAIL.
