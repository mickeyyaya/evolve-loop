# Checkpoint and resume — current Go contract

Run `evolve loop --resume` to continue a checkpointed cycle. Resume preserves the original cycle/run identity and the existing worktree; it does not create a new task attempt merely because a process restarted.

## Discovery and validation

The host resolves the explicit state override or singleton state, then eligible per-run checkpoints. Fresh leases exclude work owned by another live process. It verifies checkpoint phase, worktree presence, and identity; moved HEAD is subject to the existing resume policy. An invalid checkpoint is an explicit error, not permission to start unrelated work.

## Repeated interruption

Fresh and resumed dispatch share policy application for difficulty budgets, recurrence tiers, and audit-repair constraints. Routine phase-complete checkpoints record the completed-phase list as a conservative crash-recovery boundary. A graceful `SIGINT` or `SIGTERM` records an `operator-requested` checkpoint for the phase that is still active while leaving that phase out of `completedPhases`. Resume therefore re-enters the interrupted phase instead of repeating the preceding completed phase. The signal path does not launch retrospective failure learning because an operator stop is not evidence that the task or phase failed.

Graceful cancellation is also nonterminal at closeout. The host preserves partial timing and phase-output diagnostics, but it does not write a failure digest, create a terminal cycle dossier, mark the cycle aborted, or make a dossier commit on the integration branch. This keeps the integration HEAD unchanged while the preserved cycle branch remains resumable. Hard phase failures still use the abnormal closeout path and produce terminal evidence.

Resumed dispatch replaces the consumed pause with an active-phase checkpoint before each new model call. A later quota or operator interruption advances that recovery point, so repeated resume never falls back to the original pause. A typed quota pause remains authoritative when cancellation races its write because it carries the reset time and source needed for recovery. If the active-phase checkpoint write fails, the host reports that resume may repeat the previous completed phase; the original interruption remains the primary outcome.

Transport and quota signals are typed runtime observations. Dollar-cost estimates are telemetry, not checkpoint triggers. Elapsed time or a narrative claim does not prove a phase completed.

## Terminal closeout

Fresh and resumed execution use the same closeout semantics: final outcome reconciliation, lost-landing detection, preservation of salvageable work, carryover, and dossier creation. Terminal replay must not double-count outcomes or delete preserved work before durable state exists. Exhausting a transition bound is a diagnosed failure, not successful completion.

A resumable pause is distinct from terminal failure. Review its reason and worktree before reset. Operator salvage follows the normal review and manual-ship path; do not clear state or remove a worktree merely to make the next invocation start.

Implementation and regression evidence: [resume lifecycle recovery](recovery-resume-lifecycle.md), `go/internal/core/resume.go`, `go/internal/checkpoint`, and `go/cmd/evolve/cmd_loop.go`.

The [old shell protocol](../private/research/archived-2026-09-09/architecture-checkpoint-resume.md) is historical and must not be executed. See also [the current runtime contract](current-runtime-contract.md).
