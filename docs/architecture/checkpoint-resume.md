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

Proof: `go/internal/core/lane_resume_test.go`, `go/cmd/evolve/cmd_cycle_lane_resume_test.go`, and the wiring test `TestCycleRunRoot_AFleetLaneResumesTheQuotaPausedCycleOfItsTodoAtThePausedPhase`. The wiring test runs the real `evolve cycle run` root with a paused build. Scout does not run, and Build runs at once.

## Terminal closeout

Fresh and resumed execution use the same closeout semantics: final outcome reconciliation, lost-landing detection, preservation of salvageable work, carryover, and dossier creation. Terminal replay must not double-count outcomes or delete preserved work before durable state exists. Exhausting a transition bound is a diagnosed failure, not successful completion.

A resumable pause is distinct from terminal failure. Review its reason and worktree before reset. Operator salvage follows the normal review and manual-ship path; do not clear state or remove a worktree merely to make the next invocation start.

Implementation and regression evidence: [resume lifecycle recovery](recovery-resume-lifecycle.md), `go/internal/core/resume.go`, `go/internal/checkpoint`, and `go/cmd/evolve/cmd_loop.go`.

The [old shell protocol](../private/research/archived-2026-09-09/architecture-checkpoint-resume.md) is historical and must not be executed. See also [the current runtime contract](current-runtime-contract.md).
