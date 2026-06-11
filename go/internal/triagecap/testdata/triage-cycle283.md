## top_n (commit to THIS cycle)
- coverage-swarm-packages: Write unit tests for swarmrunner (branchByID/groupKiller/tmuxKiller/enforce), swarmplan (BaseRunner/ComposePrompt), swarm (TotalCostUSD/persistLocked/LoadManifest) → all three packages ≥98% — priority=H, evidence=scout-report.md#task-1, source=scout
- coverage-bridge-coherence-looppreflight-modelcatalog: Write unit tests for adapters/bridge (New error path, Launch cancelled-ctx), phasecoherence (canonicalRole table), looppreflight (newDefaultBootTester, PrettyJSON), modelcatalog (Write) → all four packages ≥98% — priority=H, evidence=scout-report.md#task-2, source=scout
- coverage-ship-recovery-interaction-evalgate-adversarial: Write unit tests for ship/repair ladder, recovery, interaction, evalgate, faillearn; extend adversarial_faults_test.go with agy weak-busy-signal + codex update-menu wedge + pretrustCodexProjects fake-config → all target packages ≥98% — priority=M, evidence=scout-report.md#task-3, source=scout

## deferred (carry to NEXT cycle's carryoverTodos)
_(none — all scout tasks fit within the 3-slot budget; P0 carryovers dropped as subsumed)_

## dropped (rejected with reason)
- cycle-280-failed-test-amplification: Review the failed cycle learning and fix before retrying — reason=root-cause-fixed: worktree="" dispatch bug fixed in cycle-281 binary (ec4de60e); coverage work is Task 1 in top_n which subsumes the retry requirement
- cycle-282-failed-test-amplification: Review the failed cycle learning and fix before retrying — reason=root-cause-fixed: same worktree="" bug as cycle-280; binary fix already committed; Task 1 + Task 2 in top_n cover all five leaked-file targets (adapters/bridge, evalgate, faillearn, modelcatalog, phasecoherence)

## carryoverTodos warnings (if any)
_(none — both P0 items resolved by top_n inclusion; defer_count < 3 on all items)_

## Inbox Errors
