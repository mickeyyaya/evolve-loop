# internal/campaign

## Purpose

`internal/campaign` plans and runs a multi-cycle campaign. It loads the campaign plan that the preliminary study emits, verifies it, and turns it into dependency-ordered waves. It then runs the waves, checkpoints progress after each one, and resumes past completed work. It also holds the cross-session lease that stops two sessions from running one plan.

## Design

- **Deterministic trust boundary.** `Verify` decides if a plan may run. The plan needs a version, a goal, at least one cycle, unique non-empty cycle ids, and an acyclic `depends_on` graph with no dangling references. The LLM plan reviewer is advisory and lives in the driver. The per-cycle integrity floor is enforced by each launched cycle, and the plan has no field that can weaken it.
- **Strict load.** `Load` rejects unknown fields, so schema drift fails loudly.
- **Humble executor.** `RunWaves` has no `os/exec` dependency. The command shell owns the processes and the signals. The executor owns the policy: skip completed waves on resume, run each wave through an injected `WaveRunner`, and checkpoint after each wave.
- **Checkpoint order.** Progress is saved before `AfterWaveComplete` fires, so a promotion always sees a saved boundary. The hook is fail-open. A merge-gate failure never aborts the campaign. A `BeforeWave` error aborts the run before dispatch. An interrupt during the probes therefore cannot spawn a doomed wave.
- **Retry.** Only the failed specs of a wave are re-run, as one batch, up to `MaxRetries`. `Cooldown` waits before a retry, so a quota-walled wave backs off until its bench expires.
- **Quarantine.** A wave whose only failures are optional cycles still counts as complete, and those ids go to `FailedCycleIDs`. A required failure leaves the wave incomplete, so a resumed run retries it.
- **Progress file.** `CampaignProgress` is bound to the plan by `PlanSHA`. A changed plan discards stale progress. An absent file is a zero value, so "never ran" and "ran nothing yet" behave the same. `Save` writes atomically under a sidecar lock. `MarkWaveComplete` is idempotent.
- **Ownership lease.** The lease lives in the git common directory, which every worktree shares, and it is keyed by goal hash. It is a non-blocking flock. A second `campaign run` on the same goal gets a `HeldError` that names the live owner, and it refuses instead of clobbering. The OS frees the flock when the holder dies, so no stale-PID heuristic is needed. The owner record is informational, and the flock alone is the liveness signal.

## Invariants

- A plan that fails `Verify` never runs. Pinned by the verification tests in `campaign_test.go`.
- A wave whose required cycle failed stays incomplete, and a resumed run retries it. Pinned by the retry tests in `executor_test.go`.
- The merge hook fires once per newly completed wave, after the save, and never on a resumed wave. Pinned by `executor_mergehook_test.go`.
- An interrupt during `BeforeWave` aborts before dispatch. Pinned by the F20 case in `executor_test.go`.
- A second live owner is refused with its identity. Pinned by the lease tests in `ownership_test.go`.
- Progress for a different plan is ignored. Pinned by the progress tests in `progress_test.go`.
