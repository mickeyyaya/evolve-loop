# ADR-0157: The loop works its own code: a narrow fence and held proposals

- **Status:** Proposed (2026-10-27). It amends [ADR-0155](0155-the-loop-heals-itself-inside-its-boundaries-an-agent-proposes-and-the-pipeline-disposes.md) decisions 6 and 15.
- **Plan:** [narrow-fence-2026-10](../../plans/narrow-fence-2026-10.md). **Research:** [narrow-fence-2026-10](../../research/narrow-fence-2026-10.md). **Design:** [narrow-fence-and-held-proposals](../narrow-fence-and-held-proposals.md).
- **Relates to:**
  - [ADR-0155](0155-the-loop-heals-itself-inside-its-boundaries-an-agent-proposes-and-the-pipeline-disposes.md): its T0 kernel becomes the file list below. A T0 change is now a held proposal, not a console-only change.
  - [ADR-0154](0154-the-pipeline-is-the-only-controller-and-each-agent-is-an-isolated-replaceable-worker.md): the sandbox files and the dispatch credential keep I-1, I-3, I-4 and I-7.
  - [ADR-0139](0139-a-wave-orchestrator-supplies-admits-and-backfills-every-lane.md): the supply census and the admission judge the fence through `IsProtectedScope`.
  - [ADR-0064](0064-pipeline-integrity-boundary.md): the first fence. Its pre-refactor fragments stay.
- **Evidence:**
  - 1 item of 258 is admissible, and 76 items fall on the fence (F1.1, F1.2).
  - Whole-package entries of ADR-0103 fence code that holds no gate (F1.4).
  - Most bridge files hold no guarantee. The rules live in 18 of 94 files and 4 pinned call sites (F1.5).
  - The ledger, the policy parser and the phase-order state machine have no entry (F1.8).
  - Practice uses per-file owners and a named hold that only an approver removes (P2.2, P2.3, P2.5).

## Context

The operator wants the loop to work its own pipeline items. The fence blocks most of them, but much of the fence protects no guarantee. A package entry protects each file of the package, so one gate file fences 90 ordinary files. Also, some guarantees of the constitution have no entry. On 2026-10-27 the operator approved two changes.

The fence becomes the set of files and symbols that enforce a guarantee. A lane can then work a protected or pipeline item as a held proposal, which lands only after the operator approves.

## Decision

1. Each manifest entry names one of ten guarantee classes.
2. The ten classes are listed under "What stays protected" below.
3. A directory entry stays only when each file in it enforces its guarantee.
4. Remove an entry only when the path holds no gate, by the five-part evidence below.
5. A symbol pin under `go/acs/regression/gatepins` keeps the shape of a gate call in a lane-editable file.
6. A guard test names each protected file and its class. A lost entry fails it.
7. The gate-shaped tripwire stays. A new gate-shaped file with no entry fails the audit.
8. A lane can work a protected item or a `pipeline-*` item only as a held proposal.
9. A held proposal gets an adversarial audit from another family, at the deep tier.
10. A held ship stops before the landing, in the state `held-for-operator`.
11. Only `evolve landing approve` writes an approval. It is a class D operator verb.
12. An approval names the tree digest. A different tree is not approved.
13. The approvals live under `.evolve/landing/`, which is protected. No lane can write one.
14. `held_proposals.stage` starts at `shadow`. At `shadow` the ship records what it holds.
15. `enforce` needs an operator flip after the audit dispatch and the lane admission land.

## What stays protected

Each group gives the guarantee that its entries enforce. The guard test `protectedGateFiles` names each file entry. A directory entry protects each file under it.

### The manifest (`manifest`, 1 entry)

If a lane can edit the list, it can lift the fence of each other gate (P2.1, P2.6).

- `go/internal/guards/integrity_surface.go`

### Guards (`guard`, 41 entries)

A guard refuses an action or pins a rule. A lane that weakens a guard escapes the rule that the guard keeps.

- `go/internal/guards/`
- `.claude/settings.json`
- `go/internal/binaryguard/`
- `go/internal/guardslog/`
- `go/internal/cli/guardcmd/`
- `go/internal/core/workspace_guard.go`
- `go/internal/core/activating_fields_guard_test.go`
- `go/internal/core/orchestrator_guard_test.go`
- `go/internal/core/phase_advisor_guard_test.go`
- `go/internal/core/flow_span_guard_test.go`
- `go/internal/core/ledger_runid_writers_guard_test.go`
- `go/internal/phases/ship/binary_staging_guard.go`
- `go/internal/phases/ship/binary_staging_guard_test.go`
- `go/internal/phases/ship/committed_doc_guard.go`
- `go/internal/phases/ship/committed_doc_guard_test.go`
- `go/internal/phases/ship/foreign_predicate_guard.go`
- `go/internal/phases/ship/foreign_predicate_guard_test.go`
- `go/internal/bridge/driver_tmux_submitverify_guard_test.go`
- `go/internal/bridge/integration_wait_guard_test.go`
- `go/internal/sysexec/command_guard_test.go`
- `go/internal/sysexec/llm_launch_guard_test.go`
- `go/internal/orchestrator/guard_test.go`
- `go/test/structure/nopoll_guard_test.go`
- `go/internal/profiles/gate_inputs_realtree_test.go`
- `go/internal/recoveryguard/`
- `go/internal/inboxrank/weightsort_ban_test.go`
- `go/internal/router/code_review_pin_test.go`
- `go/internal/bridge/dispatch_signal_guard_test.go`
- `go/internal/bridge/resolve_guard_test.go`
- `go/internal/bridge/selfmatch_guard_test.go`
- `go/internal/bridge/selfmatch_guard_prefilter_test.go`
- `go/internal/bridge/tmux_own_server_guard_test.go`
- `go/internal/bridge/dispatch_reap.go`
- `go/internal/proctree/`
- `go/internal/phaseobserver/phaseobserver.go`
- `go/internal/inboxbatch/consoleroute.go`
- `go/internal/inboxmover/inboxmover.go`
- `go/internal/inboxmover/lifecycle/lifecycle.go`
- `go/internal/inboxmover/lifecycle/claim.go`
- `go/internal/inboxmover/lifecycle/route.go`
- `go/internal/inboxmover/lifecycle/edit.go`

### Ship and commit gates (`ship-or-commit-gate`, 44 entries)

These files decide if a change may commit, land or pass the build floor. They include the landing, the floors and the overlap proof.

- `go/internal/commitgate/`
- `go/internal/commentaudit/`
- `go/internal/core/disposition_gate.go`
- `go/internal/core/disposition_gate_test.go`
- `go/internal/core/build_explanation_handoff.go`
- `go/internal/core/build_floor_reviewer.go`
- `go/internal/core/floor_lists_base.go`
- `go/internal/core/floor_lists_base_test.go`
- `go/internal/core/floor_lists_regression.go`
- `go/internal/core/floor_lists_regression_test.go`
- `go/internal/core/cover_changed_floor.go`
- `go/internal/core/ste_floor.go`
- `go/internal/stelint/added.go`
- `go/internal/core/build_explanation_floor_test.go`
- `go/internal/core/reviewer.go`
- `go/internal/core/leak_recovery.go`
- `go/internal/core/leak_ownership.go`
- `go/internal/core/errors.go`
- `go/internal/core/failure_diag.go`
- `go/cmd/evolve/cmd_cycle_config.go`
- `go/internal/phases/audit/ciparitygate/`
- `go/internal/phases/audit/ciparity.go`
- `go/internal/triagedecision/`
- `go/internal/core/ship_recovery.go`
- `go/internal/derived/`
- `go/internal/core/bridge_completion.go`
- `go/internal/phases/ship/native.go`
- `go/internal/phases/ship/ship.go`
- `go/internal/phases/ship/gitops.go`
- `go/internal/phases/ship/landing/`
- `go/internal/phases/ship/gitops_landing.go`
- `go/internal/phases/ship/landing_intent.go`
- `.evolve/landing/`
- `go/internal/overlap/`
- `go/internal/phases/audit/solution_gate.go`
- `go/internal/solutioncheck/`
- `go/internal/core/solution_floor.go`
- `go/internal/phases/ship/native_explanation_gate.go`
- `go/internal/core/build_handoff_floor.go`
- `go/internal/core/phase_bindings_selfcheck.go`
- `go/internal/core/blocker_proof.go`
- `go/internal/core/build_blocker_exit.go`
- `go/internal/core/floor_blocker_notes.go`
- `go/internal/addedtests/budget.go`

### Sandbox rules (`sandbox-rule`, 28 entries)

These files make the deny and grant rules of a cell and refuse an unconfined launch (ADR-0154 I-1, I-4, I-7).

- `go/internal/core/ports.go`
- `go/internal/phases/runner/runner.go`
- `go/internal/phases/runner/dispatch.go`
- `go/internal/adapters/bridge/`
- `go/internal/adapters/sandbox/`
- `go/internal/inputindex/`
- `go/internal/looppreflight/checks.go`
- `go/internal/looppreflight/drivers.go`
- `go/internal/preflight/preflight.go`
- `.evolve/profiles/builder.json`
- `.evolve/profiles/auditor.json`
- `.evolve/profiles/code-reviewer.json`
- `go/internal/bridge/sandbox_paths.go`
- `go/internal/bridge/sandbox_wrap.go`
- `go/internal/bridge/sandbox_terminal.go`
- `go/internal/bridge/read_isolation.go`
- `go/internal/bridge/protected_write.go`
- `go/internal/bridge/no_nested_agency.go`
- `go/internal/bridge/hermetic_env.go`
- `go/internal/bridge/context_access.go`
- `go/internal/bridge/launch.go`
- `go/internal/bridge/launch_llm.go`
- `go/internal/bridge/driver_tmux_boot.go`
- `go/internal/bridge/driver_claudep.go`
- `go/internal/bridge/driver_claudetmux.go`
- `go/internal/bridge/driver_codextmux.go`
- `go/internal/bridge/driver_tmux_prepare.go`
- `go/internal/bridge/manifest.go`

### Ledger and hash chain (`ledger`, 6 entries)

These files append, seal, anchor and verify the hash-chained ledger and the per-phase integrity chain.

- `go/internal/phaseintegrity/`
- `go/internal/subagent/subagentrun/`
- `go/internal/subagent/run.go`
- `go/internal/adapters/ledger/`
- `go/internal/ledgerverify/`
- `go/internal/ledgertip/`

### Policy parser (`policy-parser`, 3 entries)

These files read `policy.json` and refuse an unknown key, so no knob can disarm a gate.

- `.evolve/policy.json`
- `go/internal/policy/core.go`
- `go/internal/policy/strict_keys.go`

### Phase order (`phase-order`, 1 entry)

`CanTransition` clamps each plan of the advisor to the legal phase edges (ADR-0155 F3.2).

- `go/internal/core/statemachine.go`

### Audit verdict (EGPS) (`audit-verdict`, 41 entries)

These files run the gates of the audit, grade the verdict and carry the explanation contract that the audit and ship verify.

- `go/acs/regression/`
- `go/internal/acssuite/`
- `go/internal/phases/audit/probe_quarantine`
- `go/internal/flagregistry/registry_table.go`
- `go/internal/flagregistry/registry_ceiling_test.go`
- `knowledge-base/research/flag-campaign-plan.json`
- `skills/audit/`
- `skills/adversarial-testing/`
- `skills/solution-audit/`
- `go/internal/explanationdocs/`
- `go/internal/reportdoc/`
- `go/internal/core/orchestrator.go`
- `go/internal/core/cyclerun.go`
- `go/internal/core/cyclerun_dispatch.go`
- `go/internal/core/cyclerun_review.go`
- `go/internal/core/cyclerun_postreview.go`
- `go/internal/core/cyclerun_remediate.go`
- `go/internal/core/continuation_ancestor_predicates.go`
- `go/internal/core/evaluate_batch.go`
- `go/internal/core/failure_learning.go`
- `go/internal/core/defectledger/`
- `go/internal/phases/audit/defect_ledger.go`
- `go/internal/core/resume_execution.go`
- `go/internal/core/resume_bootstrap.go`
- `go/internal/phases/runner/verdict/`
- `go/internal/phases/runner/verdict_engine.go`
- `go/internal/phases/audit/audit.go`
- `go/internal/phases/audit/classification.go`
- `go/internal/phases/retro/retro.go`
- `go/internal/phases/audit/explanation_review_gate.go`
- `go/internal/phases/retro/explanation_review_gate.go`
- `agents/evolve-auditor.md`
- `agents/evolve-auditor-reference.md`
- `schemas/handoff/audit-report.schema.json`
- `.evolve/build-explanation-contracts/`
- `skills/quality-index/`
- `go/internal/qualityindex/`
- `skills/architecture-review/`
- `go/internal/codereview/`
- `agents/evolve-code-reviewer.md`
- `.evolve/phases/code-review/`

### Dispatch credential (`dispatch-credential`, 4 entries)

These files keep the credential that no cell can read, and refuse each dispatch verb without it (ADR-0154 I-3).

- `go/internal/dispatchcred/`
- `go/cmd/evolve/dispatch_gate.go`
- `go/cmd/evolve/dispatch_gate_test.go`
- `go/internal/bridge/dispatch_credential.go`

### Kernel-loaded instructions (`kernel-instruction`, 14 entries)

The kernel loads these skills, personas, schemas and rules into each agent. A tampered copy rewrites what each agent does.

- `skills/fable/`
- `skills/solution-scout/`
- `skills/solution-build/`
- `skills/engineering-craft/`
- `skills/code-review-simplify/`
- `skills/docs-write/`
- `go/internal/guidance/`
- `guidance/rules/`
- `agents/evolve-builder.md`
- `agents/evolve-builder-reference.md`
- `agents/evolve-retrospective.md`
- `agents/evolve-memo.md`
- `schemas/handoff/build-report.schema.json`
- `schemas/handoff/retrospective-report.schema.json`

### Symbol pins

A pin protects one call in a file that a lane can change. A pin checks the shape of the code, not only a name. The refusal must be a top-level `if` of the function, and its arguments and its return must stay. The suite runs at audit, so a lane that breaks a pin fails before its landing.

| File | Function | Shape that must stay |
|---|---|---|
| `go/internal/bridge/driver_agy.go` | `agyDriver.Launch` | `wrapped` from `wrapHeadlessInvocation` goes into `if sandboxRequiredButUnavailable(..., wrapped)`, which returns `ExitSafetyGate` |
| `go/internal/bridge/driver_codex.go` | `codexDriver.Launch` | the same refusal |
| `go/internal/bridge/engine.go` | `Deps.withDefaults` | `if d.SandboxWrap == nil` assigns `defaultSandboxWrap(d)` |
| `go/internal/bridge/engine.go` | `launchArgs` | `if req.RequireSandbox` appends `--require-sandbox` |

A second tripwire, `TestRefusalCallSitesAreFencedOrPinned`, fails when a file outside the fence calls `sandboxRequiredButUnavailable`, `validateEnvPassthrough` or `WithProtectedPath` and has no pin.

## What leaves the fence

Each removed entry has five parts of evidence:

- **E1** No file in it has a gate-shaped name. `TestEveryGateShapedFileIsProtectedSurface` stays green.
- **E2** The explanation call-site scanner has no hit in it.
- **E3** No protected pin test names it.
- **E4** Its rationale names a role, not a refusal.
- **E5** No file in it calls a refusal of a protected file, by a scan for the refusal symbols, the credential checks and `syscall.Kill`. The scan ran once, on 2026-10-27. The standing tripwire checks only three of these symbols.

Five guard tests of the bridge failed E1, so they got entries of their own. The architecture review found more refusals by E5. These files stay on the fence as their own entries:

- `driver_claudep.go`, `driver_claudetmux.go`, `driver_codextmux.go`, `driver_tmux_prepare.go`, `manifest.go` and `dispatch_reap.go` of the bridge;
- `proctree/` and `phaseobserver.go`, the kill paths;
- `inboxbatch/consoleroute.go`, `inboxmover.go` and four files of `inboxmover/lifecycle/`, the claim refusal.

| Removed entry | Role (E4) |
|---|---|
| `go/internal/bridge/` | replaced by 18 files with a sandbox, credential or kill rule, 7 guard tests and 4 symbol pins (F1.5) |
| `go/internal/bridge/launchoutcome/` | the exit-code classifier of a launch failure |
| `go/internal/bridge/engine.go` | the launch spine; its two sandbox calls are pinned |
| `go/internal/core/advisor/` | the advisor writes a proposal; `CanTransition` clamps it |
| `go/internal/core/phase_advisor.go` | the seam of the advisor; its guard test stays |
| `go/internal/core/carryover/` | the lifecycle of carryover items |
| `go/internal/core/carryover_lifecycle.go` | the seam of the carryover lifecycle |
| `go/internal/core/failurediag/` | the writer and classifier of failure diagnostics |
| `go/internal/core/failurelearning/` | the recorder of failed approaches |
| `go/internal/core/failure_learning_engine.go` | the seam of the failure-learning engine |
| `go/internal/core/outcome/` | the recorder of phase outcomes |
| `go/internal/core/continuation_stamp.go` | the history stamp of a continuation |
| `go/internal/core/resume.go` | the resume entry; its gate call sites live in `resume_execution.go` and `resume_bootstrap.go`, which stay |
| `go/internal/core/phase.go` | types; a removed field breaks the build of a protected file |
| `go/internal/cyclestate/state.go` | types; a removed field breaks the build of a protected file |
| `go/internal/phaseio/handoffs.go` | types; a removed field breaks the build of a protected file |
| `go/internal/config/` | the loader of the routing config |
| `go/internal/observerengine/` | the phase observer; its kill site `phaseobserver.go` stays |
| `go/internal/adapters/observer/core_adapter.go` | the adapter of the phase observer |
| `go/internal/textcap/` | two text-length rules |
| `go/internal/inboxmover/lifecycle/` | the inbox promote, release and file mover; its four claim files stay |
| `go/internal/loopwave/` | the wave engine; its process guard `orchestrator/guard_test.go` stays |
| `go/internal/loopchain/` | the batch-chain driver |
| `go/cmd/evolve/cmd_loop_wave.go` | the seam of the wave engine |
| `go/cmd/evolve/cmd_loop_chain.go` | the seam of the batch-chain driver |

## Consequences

Good:

- 25 items leave `console_protected` on a copy of the live inbox. The lane-routable supply goes from 2 to 27.
- Three guarantees get an entry for the first time: the ledger, the policy parser and the phase order.
- Each entry says which guarantee it keeps, and a test pins each file.
- A change to the kernel has a path that does not need a console session, and the operator still decides.

Bad:

- A symbol pin checks a shape. A lane can keep the shape and still change what the called function gets. The audit and the review are the next layers.
- The freed items share the bridge and core cells, so they run one after another.
- Until the audit dispatch lands, `enforce` holds each held proposal with no way to approve it. The stage stays at `shadow`.
- PR 2 and PR 3 change the manifest again. Each change is a manual ship.

What changes our mind: a freed file that weakens a gate on `main` returns to the fence with a pin test. The operator can reject a held gate change that the audit passed. If the `shadow` record shows that, `enforce` waits for a stronger audit.

## Rejected

| Option | The fact that rules it out |
|---|---|
| Keep the package fence and add an allow list of items | An allow list is a hand list with no oracle (P2.6). |
| A second lane class with its own fence | It doubles the fence and the scheduler work for the same result. |
| Remove the explanation call sites now | Each call site of the explanation lifecycle needs a pin first. The plan defers them (Q2). |
| Let an AI reviewer approve a held proposal | An automated author never approves its own change (P2.4, P2.7). |
| Approve an item, not a tree | A later tree reuses the approval (P2.5, P2.8). |
