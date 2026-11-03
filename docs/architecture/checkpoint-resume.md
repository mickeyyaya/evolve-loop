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
2. It records one auto-resume through `checkpoint.RecordLaneResume`. This call is the only budget rule. It refuses the resume when the auto-resume budget of the checkpoint is spent. Thus a family that stays walled cannot loop the lane. The same call also takes the run lease of the paused cycle. Step 2 has its own section below.
3. It claims the todos back for the paused cycle through the claimer of the cycle (`hostInboxClaimer`). It then makes sure that the paused cycle holds each todo. If a todo is still not held, it releases each claim of the paused cycle through `releaseDeferredLaneClaims`. Thus no todo stays claimed by a cycle that does not run.
4. It removes the wave goal from the request, so the checkpoint goal stays the goal of the cycle.
5. It publishes `ORCHESTRATOR_LANE_RESUMED` with the scope, the phase and the auto-resume count.

If one of these steps fails, the lane starts a fresh cycle. A refused resume thus costs a fresh cycle, never a held lane. A resume that hits a quota wall again writes a new quota checkpoint and keeps the auto-resume count.

### One lane adopts a paused cycle

A resumed lane does not take the project lock. Thus the project lock cannot stop two lanes that resume the same paused cycle. Before this rule, two lanes with the same scope can both pass `FindLaneResume` and both record a resume. Both lanes then reclaimed the same cycle, and two builders wrote into one worktree.

`checkpoint.RecordLaneResume` (`go/internal/checkpoint/autoresume.go`) makes the adoption exclusive. Under the flock of the state file, it counts the auto-resume. It then calls `core.TakeResumeLease` (`go/internal/core/lane_resume.go`) on the workspace of the paused cycle. A spent budget takes no lease.

If the workspace has a fresh run lease, the call refuses with `*core.ResumeHeldError`. This error names the run ID, the owner PID and the heartbeat of the holder. The refusal writes no auto-resume, and the lane starts a fresh cycle. If no fresh lease exists, the call writes an admission lease with the PID of the lane. `RunCycleFromPhase` then replaces this lease with its own heartbeat lease.

One function, `resumeHolder` (`go/internal/core/lane_resume.go`), decides if a lease holds the workspace. `FindLaneResume` and `TakeResumeLease` both call it, so they cannot disagree. A fresh lease holds the workspace, and an expired lease does not.

A corrupt or unreadable lease holds the workspace until its file modification time is older than the lease TTL (`runlease.DefaultTTL`). After that time, the next lane reclaims the cycle. A lease file with no readable modification time holds nothing. Thus a live holder keeps the cycle, and a damaged lease never strands it. Before this rule, `FindLaneResume` read a corrupt lease as free and `TakeResumeLease` read it as held. Each lane then found the cycle and was refused, so the cycle stayed paused.

An adoption can fail after the lane takes the admission lease. The reclaim of the todos can fail, or `RunCycleFromPhase` can return an error before its heartbeat starts. In both cases, the lane calls `core.ReleaseResumeLease`. This call removes the lease only when it is still the admission lease of this process. A heartbeat lease of the resumed run stays. Thus a failed adoption does not block the paused cycle for a full TTL.

`newLaneResume` (`go/cmd/evolve/cmd_cycle_lane_resume.go`) is the one constructor of the lane. `runCycleRun` and the lane tests both call it, so a test cannot pass on a second copy of the composition.

Proof: `TestRecordLaneResume_TwoConcurrentLaneResumesOfOnePausedCycleAdmitExactlyOne`, `TestRecordLaneResume_ASpentBudgetTakesNoLease`, the `TestTakeResumeLease_*` tests and `TestResumeHolder_ACorruptLeaseHoldsThePausedCycleForFindAndTakeUntilItsFileAgesPastTheTTL`.

More proof: the release has `TestReleaseResumeLease_*` and `TestLaneResume_AnAdoptionThatFailsBeforeTheHeartbeatReleasesTheAdmissionLease`. The constructor has `TestNewLaneResume_TheComposedRecordRefusesACycleThatAnotherLaneHolds`. The root has `TestCycleRunRoot_ALaneDoesNotAdoptAPausedCycleWhoseLeaseIsUnreadable`.

### The project lock of a resume

`RunCycle` and `RunCycleFromPhase` call one function, `Orchestrator.acquireCycleLock` (`go/internal/core/orchestrator.go`). Each call site gives its skip rule as a function value. A fresh fleet cycle (`EVOLVE_FLEET=1`) skips the lock. A resume skips the lock only when it is a fleet lane with a fleet scope (`EVOLVE_FLEET=1` and a non-empty `EVOLVE_FLEET_SCOPE`). A sequential `evolve loop --resume` can inherit `EVOLVE_FLEET=1` from its shell. It has no fleet scope, so it takes the project lock.

In wave 150, before this rule, the resume of cycle 2106 took the project lock for its full run. One millisecond later, the resume of cycle 2107 stopped with `acquire lock: core: project lock held by another process`. The lane exited, it spent one auto-resume, and the pool had one less live lane.

Proof: `go/internal/core/lane_resume_test.go`, `go/cmd/evolve/cmd_cycle_lane_resume_test.go`, `TestRunCycleFromPhase_ASequentialResumeThatInheritedTheFleetFlagWithoutAScopeTakesTheProjectLock`, the lock tests in `go/internal/core/fleet_lock_test.go`, and the wiring test `TestCycleRunRoot_AFleetLaneResumesTheQuotaPausedCycleOfItsTodoAtThePausedPhaseWhileASiblingHoldsTheProjectLock`. The wiring test runs the real `evolve cycle run` root with a paused build while the real project lock is held. Scout does not run, and Build runs at once.

## Terminal closeout

Fresh and resumed execution use the same closeout semantics: final outcome reconciliation, lost-landing detection, preservation of salvageable work, carryover, and dossier creation. Terminal replay must not double-count outcomes or delete preserved work before durable state exists. Exhausting a transition bound is a diagnosed failure, not successful completion.

A resumable pause is distinct from terminal failure. Review its reason and worktree before reset. Operator salvage follows the normal review and manual-ship path; do not clear state or remove a worktree merely to make the next invocation start.

Implementation and regression evidence: [resume lifecycle recovery](recovery-resume-lifecycle.md), `go/internal/core/resume.go`, `go/internal/checkpoint`, and `go/cmd/evolve/cmd_loop.go`.

The [old shell protocol](../private/research/archived-2026-09-09/architecture-checkpoint-resume.md) is historical and must not be executed. See also [the current runtime contract](current-runtime-contract.md).
