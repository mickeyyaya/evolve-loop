# Checkpoint and resume — current Go contract

Run `evolve loop --resume` to continue a checkpointed cycle. Resume preserves the original cycle/run identity and the existing worktree; it does not create a new task try merely because a process restarted.

## Discovery and validation

The host resolves the explicit state override or singleton state, then eligible per-run checkpoints. Fresh leases exclude work owned by another live process. It verifies checkpoint phase, worktree presence, and identity; moved HEAD is subject to the existing resume policy. An invalid checkpoint is an explicit error, not permission to start unrelated work.

## Repeated interruption

Fresh and resumed dispatch share policy application for difficulty budgets, recurrence tiers, and audit-repair constraints. Routine phase-complete checkpoints record the completed-phase list as a conservative crash-recovery boundary. A graceful `SIGINT` or `SIGTERM` records an `operator-requested` checkpoint for the phase that is still active while leaving that phase out of `completedPhases`. Resume therefore re-enters the interrupted phase instead of repeating the preceding completed phase. The signal path does not launch retrospective failure learning because an operator stop is not evidence that the task or phase failed.

Graceful cancellation is also nonterminal at closeout. The host preserves partial timing and phase-output diagnostics. The host does none of these actions:

- write a failure digest;
- create a terminal cycle dossier;
- mark the cycle aborted;
- make a dossier commit on the integration branch.

This keeps the integration HEAD unchanged while the preserved cycle branch remains resumable. Hard phase failures still use the abnormal closeout path and produce terminal evidence.

Resumed dispatch replaces the consumed pause with an active-phase checkpoint before each new model call. A later quota or operator interruption advances that recovery point, so repeated resume never falls back to the original pause. A typed quota pause remains authoritative when cancellation races its write because it carries the reset time and source needed for recovery. If the active-phase checkpoint write fails, the host reports that resume may repeat the previous completed phase; the original interruption remains the primary outcome.

Transport and quota signals are typed runtime observations. Dollar-cost estimates are telemetry, not checkpoint triggers. Elapsed time or a narrative claim does not prove a phase completed.

## Fleet lane resume

A fleet lane that hits a quota wall does not hold the wave. On 2026-11-02 the operator asked for this rule: "resume the stopped phase agent with the fallback llm". In wave 149, cycles 2106 to 2109 paused on a quota wall with a valid checkpoint. No part of the wave resumed them. Each todo went back to the inbox, and the next lane of each todo started again at Scout.

The lane flow now has three steps:

1. At the quota wall, `cycleRunQuotaExit` (`go/cmd/evolve/cmd_cycle_quota_deferral.go`) releases the claim of the lane and keeps the quota checkpoint. The lane exits with `fleet.ExitDeferred`. The todo is pending again, so the wave can never block on it.
2. The next lane of the same todo starts `evolve cycle run`. `laneResume.adopt` (`go/cmd/evolve/cmd_cycle_lane_resume.go`) calls `core.FindLaneResume` (`go/internal/core/lane_resume.go`) with the lane scope.
3. If the paused cycle is resumable, the lane continues that cycle at its paused phase through `RunCycleFromPhase`. The dispatch chain gives the phase to the first healthy family. Thus a benched family hands the phase to its fallback.

`core.FindLaneResume` accepts a checkpoint only when each condition is true:

| Condition | Reason |
|---|---|
| The newest cycle of the todo scope is the paused cycle. | A fresh try after the pause owns the todo. An older checkpoint never comes back. |
| The checkpoint is enabled, its reason is `quota-likely`, and no live run holds it. | Other pause reasons have their own recovery. |
| The phase is not `end` or `aborted`. | A closed cycle has nothing to resume. |
| The worktree exists, and the checkpoint age is within `resume.max_checkpoint_age_h`. | The resume needs the work of the paused cycle. |

Before the lane runs the cycle, it does these steps in this order:

1. It makes sure that each todo of the scope is pending or already held by the paused cycle. This check changes nothing, so a refusal here spends no auto-resume.
2. It records one auto-resume through `checkpoint.RecordResume`. This call is the only budget rule. It refuses the resume when the auto-resume budget of the checkpoint is spent. Thus a family that stays walled cannot loop the lane.
3. It claims the todos back for the paused cycle through the claimer of the cycle (`hostInboxClaimer`). It then makes sure that the paused cycle holds each todo. If a todo is still not held, it releases each claim of the paused cycle through `releaseDeferredLaneClaims`. Thus no todo stays claimed by a cycle that does not run.
4. It removes the wave goal from the request, so the checkpoint goal stays the goal of the cycle.
5. It publishes `ORCHESTRATOR_LANE_RESUMED` with the scope, the phase and the auto-resume count.

If one of these steps fails, the lane starts a fresh cycle. A refused resume thus costs a fresh cycle, never a held lane. A resume that hits a quota wall again writes a new quota checkpoint and keeps the auto-resume count.

A resumed lane does not take the project lock. `RunCycle` and `RunCycleFromPhase` ask one rule, `Orchestrator.acquireCycleLock` (`go/internal/core/orchestrator.go`): a fleet lane (`EVOLVE_FLEET=1`) skips the lock, and a sequential cycle takes it. In wave 150, before this rule, the resume of cycle 2106 took the project lock for its full run. One millisecond later, the resume of cycle 2107 stopped with `acquire lock: core: project lock held by another process`. The lane exited, it spent one auto-resume, and the pool had one less live lane.

Proof: `go/internal/core/lane_resume_test.go`, `go/cmd/evolve/cmd_cycle_lane_resume_test.go`, the lock tests in `go/internal/core/fleet_lock_test.go`, and the wiring test `TestCycleRunRoot_AFleetLaneResumesTheQuotaPausedCycleOfItsTodoAtThePausedPhaseWhileASiblingHoldsTheProjectLock`. The wiring test runs the real `evolve cycle run` root with a paused build while the real project lock is held. Scout does not run, and Build runs at once.

## Resume every paused cycle at once

On 2026-11-03 the operator asked: "resume should take place for all cycles concurrently ... It must be able to support any cycle to resume". `evolve cycle resume` resumes only the newest paused cycle, one at a time. A pause with an unknown reset time sets the auto-resume budget to 0 (`withQuotaReset`). The lane refuses such a resume, starts a fresh cycle, and the paused work is lost.

`evolve cycle resume --all` starts no cycle. It does these steps:

1. `core.PausedLaneScopes` (`go/internal/core/lane_resume_all.go`) lists each lane scope whose newest cycle a lane can resume.
   - It uses the one rule of `laneResume.adopt`: `laneResumable` and `newestPerScope` in `go/internal/core/lane_resume.go`.
   - Under that rule, the cycle did not ship and did not end. It holds an enabled quota checkpoint that loads, and no live run holds it.
   - The list is in cycle order. A read error of the runs directory is an error. The log names each checkpoint that does not load.
2. `checkpoint.GrantOperatorResume` gives a fresh budget only to an enabled quota checkpoint whose budget is spent.
   - The attempts become 0, and the maximum becomes at least `DefaultAutoResumeAttempts`.
   - The operator action and the reset time are cleared. The reset source becomes `operator-grant`.
   - The reason, the phase and the enabled flag stay.
   - A checkpoint that still has budget does not change. Thus a second `--all`, or a grant after a lane adopted the cycle, does not reset the count.
3. The verb prints one line for each cycle (`granted`, `has budget`, `would grant` or `not granted`) and the next step, `evolve wave next`. If the scan fails, the verb exits with `cliExitIO`. If one grant fails, the verb reports it and exits with a non-zero code. The other cycles are still granted.

The budget is 0 for an unknown reset time because no automatic retry can know when the quota returns. Thus the operator must decide. The grant is that decision.

The lane rule does not use the cross-launch staleness rule of the single resume ("a cycle of another launch started or shipped after this checkpoint"). In a wave, the sibling lanes always start and ship after a pause. With that rule, the lane rule refuses each paused lane cycle. `TestPausedLaneScopes_ASiblingLaneThatStartedLaterDoesNotMakeAPausedLaneStale` holds this decision.

The pool then puts each ready paused scope at the head of the backlog (`pausedPlan.order` in `go/cmd/evolve/cmd_loop_pool_paused.go`):

- A scope is ready when `checkpoint.ResumeReady` is true. Its budget is not spent (`checkpoint.BudgetLeft`, the rule of admission). Its known reset time is not in the future.
- The checkpoint writes `quotaResetAt` in the layout `2006-01-02T15:04:05-0700`, for example `2026-11-04T08:53:30+0800`. The offset has no colon, so this value is not RFC3339. `quotareset.ParseISO` reads this layout and RFC3339. The writer and the parser share one layout constant. `ResumeReady` and the loop status line use this parser.
- An unreadable reset time that is not empty does not stop the resume. `ResumeReady` reports the scope as ready and returns an error, and the pool writes a `[pool] WARN` line that names the cycle.
- The reset time is an order hint only. A pause that waits for a known reset keeps its triage place. Admission (`RecordResume`) does not check the reset time, so a lane that draws that triage todo still resumes the cycle.
- The lead todo is the triage todo of the scope, with its files, priority, dependencies and output contract. The pool serializes lanes on their files, so a resumed lane keeps its file claims. A scope that is not in the triage list takes the declared files of its inbox items.
- The pool does not put a scope first when no triage todo matches it and none of its inbox items is found. Such a scope has no file claim, and its todo is gone.
- An untriaged scope leads with only its ID and its files. Its dependencies and its output contract are not recorded yet.
- The triage todos of the scope do not appear again.

The lanes adopt the scopes through `laneResume.adopt`, and the cycles run at the same time. If the scan fails, the pool writes a warning and keeps the triage order.

Use `evolve cycle resume` for one cycle in the foreground. Use `--all` when several cycles wait.

Two tests guard the concurrent resume (`go/cmd/evolve/cmd_cycle_resume_concurrent_test.go`). They seed four lane cycles that paused on an unknown reset, as cycles 2137, 2138, 2139 and 2141 of wave 151 did:

- `TestResumeAll_WithoutTheGrantEachLaneOfAnUnknownResetWallRefusesItsPausedCycle` proves the gap: with no grant, no lane adopts its paused cycle.
- `TestResumeAll_EveryPausedCycleResumesConcurrentlyAtItsOwnPhaseAfterOneGrant` runs one `evolve cycle resume --all`, then checks the backlog order (`pausedPlan.order`), then starts the four lane adoptions at the same time. Each lane must adopt its own cycle at its paused phase, spend exactly one resume, and hold its own todo again. `ORCHESTRATOR_LANE_RESUMED` must fire once for each cycle. The test runs with the race detector. It fails without the grant and without the paused-first order. `TestProductionPoolPlanFn_AGrantedPausedLaneLeadsTheRealBacklogWithItsFiles` drives the real `productionPoolPlanFn`, and it fails when the plan does not call `pausedPlan.order`.

The test calls `laneResume.adopt` in goroutines, not `laneResume.run`. `run` sets the state file of the cycle in the process environment, which is safe only because each production lane is its own process. Two lanes that adopt the same scope are not covered here; that case is the run lease (PR #1256).

## Terminal closeout

Fresh and resumed execution use the same closeout semantics: final outcome reconciliation, lost-landing detection, preservation of salvageable work, carryover, and dossier creation. Terminal replay must not double-count outcomes or delete preserved work before durable state exists. Exhausting a transition bound is a diagnosed failure, not successful completion.

A resumable pause is distinct from terminal failure. Review its reason and worktree before reset. Operator salvage follows the normal review and manual-ship path; do not clear state or remove a worktree merely to make the next invocation start.

Implementation and regression evidence: [resume lifecycle recovery](recovery-resume-lifecycle.md), `go/internal/core/resume.go`, `go/internal/checkpoint`, and `go/cmd/evolve/cmd_loop.go`.

The [old shell protocol](../private/research/archived-2026-09-09/architecture-checkpoint-resume.md) is historical and must not be executed. See also [the current runtime contract](current-runtime-contract.md).
