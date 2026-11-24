# internal/core/auditfacts

> Decision: [ADR-0158](../adr/0158-a-repair-round-runs-only-for-a-finding-the-lane-can-fix-and-the-auditor-applies-a-small-prescribed-fix.md) decisions 20 and 21. Plan: [cost-aware-repair-2026-10](../../plans/cost-aware-repair-2026-10.md), item 7.

## Purpose

`internal/core/auditfacts` renders the block that the audit prompt carries below the cycle-context boundary. The block holds the gate and floor facts of the cycle and the rubric of the two mandatory report sections. The pipeline composes it from its own records. The auditor does not write it and does not argue it.

## Design

- **One pure function.** `Block(Inputs)` returns the text. All I/O is in `Inputs`. It holds the round, the gate names, the ACS reading, the handoff checks, the audit fail reasons and the diff. It also holds two seams: `ReadFile` and `Protected`. The package imports only `covchanged`.
- **The facts.** Each fact has one line. The grammar is fixed: `round`, `gates_the_host_decides_after_your_report`, `acs_verdict_round<N>`, `handoff_<pair>`, `audit_gate_fail_untrusted_json`, `changed_unprotected_functions` and `changed_tests` (at most 60 entries, then `+N more`). A fact that the host cannot derive reads `not recorded`. It never reads as an empty list.
- **Changed functions.** `covchanged.ParseDiff` gives the added lines. `covchanged.ChangedFuncNames` names the functions of a file whose span holds an added line. A file that git does not track yet counts as wholly changed. A path on the fence (`guards.IsProtectedSurface`) is not listed as unprotected code.
- **The rubric.** `Rubric` names `## Final Diff Regression Review` and `## Guard Honesty Check`. It is part of the block, so it applies to each audit dispatch, with no link to the explanation-documentation version.
- **The gate list.** The names come from `core.AuditGateNames()`, a projection of the one table `auditGateRemedies`. `TestGateNames_EveryGateLabelOfGatesGoIsARemedyRowOfTheOneHome` pins each label of `phases/audit/gates.go` to it.

The audit phase runs the pipeline step. `runner.ContextSeeder` is an optional hook. `preparePhaseExecution` calls it once, before `ComposePrompt`.

`audit.hooks.SeedContext` gathers the records:

- the archived `acs-verdict.round<N>.json`;
- `run.json`;
- each `handoff-*.json`;
- `git diff --unified=0 <WorktreeBaseSHA>` in the worktree, the same base as the build floor.

It stores the block in a copy of `req.Context[core.CtxKeyAuditFacts]`. `ComposePrompt` only renders that text. It runs no git and writes no file. Nothing is persisted, so no file in the workspace can change what the host judges.

## Limits

- Gates run after the auditor reply, so in round 1 only the floor facts exist. The block says so with `not recorded`.
- The function level is Go only. A file of another language is not listed.
- Empty test bodies are not in the block. `internal/emptytests` (PR #1723) is not on main. The inbox item `audit-facts-empty-tests` adds them and adds the check of the two sections under its own shadow stage.
- This build ships prompt data only. No validator, policy key or signal exists (ADR-0158 line 95).
