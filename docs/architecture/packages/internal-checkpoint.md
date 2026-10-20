# internal/checkpoint

## Purpose

`internal/checkpoint` writes the cycle checkpoint: a resume record that lets `evolve cycle resume` and the resume scripts continue an interrupted cycle. It ports `cycle_state_checkpoint` from the bash lifecycle. The block sits inside `.evolve/cycle-state.json` as an additive `checkpoint` key. There is no separate checkpoint file.

## Design

- **Reasons.** A checkpoint carries one reason. Four are escalation reasons: quota-likely, batch-cap-near, operator-requested and stall-inactivity. The fifth, phase-complete, is the lowest priority, and it marks each phase boundary.
- **Compose is pure.** `Compose` builds the block without I/O. `ComposeChecked` validates the reason, for callers whose reason comes from outside. `ComposeWithIntegrity` also carries the recorded integrity chain, so a phase-complete write does not drop it. `Compose` alone stays integrity-free for back-compat.
- **Escalation is not overwritten.** The phase-boundary hook never replaces an escalation checkpoint. The escalation survives until its consumer reads it.
- **One lock for the file.** `ApplyToStateFile` and `RecordPhaseIntegrity` run under `flock.WithPathLock` on the cycle-state file. This is the same lock that `storage.WriteCycleState` takes, so a phase write and a checkpoint write cannot revert each other. The escalation check is lock-free, and the caller must hold the lock before calling it. Otherwise the check and the write straddle the lock, and a peer's escalation can be lost.
- **Distinct temp file.** The integrity writer uses its own temp suffix. The lock already serializes the writers, and a unique name makes a temp-file collision impossible.
- **Integrity chain.** The per-phase digest chain is best-effort in this file. A garbled chain reads as empty and never blocks a cycle. The tamper-evident record is the hash-chained ledger anchor, which a security check verifies.
- **Quota wake time.** The resume time comes from the first source that exists. The sources are, in order: an operator override in policy, a hint that the CLI adapter scraped into the workspace, and an estimate. Only the quota wall has a reset instant, so a phase-complete checkpoint never invents one.
- **Auto-resume cap.** `DefaultAutoResumeAttempts` is the default cap that `Compose` stamps on each block.

## Invariants

- Two writers of one cycle-state file (phase and checkpoint) lose no update. Pinned by `TestCycleState_ConcurrentWriters_BothUpdatesSurvive`.
- A phase-boundary write never replaces an escalation, under any interleaving. Pinned by `TestPhaseBoundaryCheckpointer_ConcurrentEscalation_NotClobbered` and `TestPhaseBoundaryCheckpointer_YieldsToEscalation`.
- Concurrent integrity appends keep every entry and never tear the file. Pinned by `TestRecordPhaseIntegrity_ConcurrentPipeline_NoLostUpdates`.
- A phase-complete write carries the integrity chain. Pinned by `TestPhaseBoundaryCheckpointer_CarriesIntegrityChain` and `TestComposeWithIntegrity_CarriesChain`.
- The phase-complete reason is canonical and validated. Pinned by `TestReasonPhaseComplete_IsValid` and `TestComposeChecked_AcceptsPhaseComplete`.
- The quota wake time follows the source order, and a routine checkpoint never invents one. Pinned by `TestQuotaBoundaryCheckpointer_UsesWorkspaceHint`, `TestQuotaBoundaryCheckpointer_HonoursOperatorOverride` and `TestPhaseBoundaryCheckpointer_DoesNotFabricateWakeAt`.
- Each boundary writer records the cycle base as `gitHead`: the run's `worktree_base_sha`, else its `pre_cycle_head` (`cycleBase`, 2026-10-21). `core.LoadResumeState` refuses the checkpoint when `origin/main` has moved past that base. Pinned by `TestBoundaryCheckpointers_RecordTheCycleBaseAsGitHead`.

## Findings

- The reason type comment says "four" canonical reasons, and `IsValid` says "five". The set has five values. The four-reason wording in the `Reason` doc and in `ComposeChecked` is stale.
