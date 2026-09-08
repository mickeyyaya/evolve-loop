# Checkpoint and resume — current Go contract

Run `evolve loop --resume` to continue a checkpointed cycle. Resume preserves the original cycle/run identity and the existing worktree; it does not create a new task attempt merely because a process restarted.

## Discovery and validation

The host resolves the explicit state override or singleton state, then eligible per-run checkpoints. Fresh leases exclude work owned by another live process. It verifies checkpoint phase, worktree presence, and identity; moved HEAD is subject to the existing resume policy. An invalid checkpoint is an explicit error, not permission to start unrelated work.

## Repeated interruption

Fresh and resumed dispatch share policy application for difficulty budgets, recurrence tiers, and audit-repair constraints. Checkpoint advancement is durable at phase boundaries, and a new quota interruption records the current recovery point. A second resume must use that point rather than replaying the original checkpoint forever.

Transport and quota signals are typed runtime observations. Dollar-cost estimates are telemetry, not checkpoint triggers. Elapsed time or a narrative claim does not prove a phase completed.

## Terminal closeout

Fresh and resumed execution use the same closeout semantics: final outcome reconciliation, lost-landing detection, preservation of salvageable work, carryover, and dossier creation. Terminal replay must not double-count outcomes or delete preserved work before durable state exists. Exhausting a transition bound is a diagnosed failure, not successful completion.

A resumable pause is distinct from terminal failure. Review its reason and worktree before reset. Operator salvage follows the normal review and manual-ship path; do not clear state or remove a worktree merely to make the next invocation start.

Implementation and regression evidence: [resume lifecycle recovery](recovery-resume-lifecycle.md), `go/internal/core/resume.go`, `go/internal/checkpoint`, and `go/cmd/evolve/cmd_loop.go`.

The [old shell protocol](../private/research/archived-2026-09-09/architecture-checkpoint-resume.md) is historical and must not be executed. See also [the current runtime contract](current-runtime-contract.md).
