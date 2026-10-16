# internal/phases/build

## Purpose

`internal/phases/build` is the GREEN code-writer phase. The boilerplate (profile lookup, prompt composition, bridge dispatch, artifact reading, response packaging) lives in `internal/phases/runner`. This package holds only the build-specific variation points: the agent name, the artifact file name, the prompt context and the classification rules.

## Design

- **Verdict mapping.** `classifyArtifact` derives the verdict from the completeness of the report. A missing or empty `## Files Modified` section gives FAIL, and every other GREEN path gives PASS. The accepted headings of the changed-files section are in `phasecontract.Build`, the single source. The heading drifted once to `## Changes` (cycle 192), and the classifier then failed a valid report and blocked ship, so `TestClassifyArtifact_HeadingVariants` pins the accepted variants.
- **`Phase` embeds `*runner.BaseRunner` concretely.** This is the same as every other phase on `BaseRunner`. The public seams of `BaseRunner`, such as `ComposePrompt` (which the audit of the cache-stable prefix relies on), then promote onto `*Phase`. An embedded `core.PhaseRunner` interface hid them behind its two methods.
- **Prompt blocks and the legacy prompt.** Each optional block is keyed on a context value. An absent key leaves the prompt byte-identical to the legacy prompt. This holds for the lane scope, the findings of a prior try, the standing findings and the audit-repair reason. A paired negative test pins each one.
- **Findings are data.** The findings of a prior try, the standing findings and the audit reason quote lines that agents wrote. They are fenced as data. The builder reads them as a record of the failure, and never as instructions to follow.
- **Audit-repair reason.** The repair branch is worthless if the reasoning of the audit never reaches the agent that must act on it. The cycle then rebuilds blind and earns the same rejection again at full cost. `TestComposePrompt_AuditRepair*` asserts that the reason text survives prompt composition word for word.
- **Measured motivation.** Cycle 1577 resumed the failed task of cycle 1574. Only the build prompt mentioned the failure, and it did so once.
- **Standing findings.** A recovery rebuild is audited again by the same rubric. The actionable findings of the last audit therefore ride in the brief as data. Without them, a rebuild after `GIT_FLEET_REBASE_NEEDED` gave the next round the same defects as "standing" (cycle 1679).
- **Lane scope.** In a fleet lane, the prompt renders the pinned lane scope as a `fleet_scope` line. The builder then binds only to the todo ids of its own lane, and the id of a foreign lane never appears. A sequential cycle has no such line.
- **Task contract.** The harness owns the task-contract block (ADR-0098). The heading is added here and the preamble and the block are composed once in `core`, so tdd, build and audit read the same words.
- **Build plan.** The build plan is served only through the typed `PhaseRequest.BuildPlan` envelope. The dispatch seam fills it when the planner is enabled at advisory level or above (ADR-0050). When it is empty, no section is injected. The disk fallback no longer exists.
- **`Config` fields.** `ContractVerifier` gives the verdict engine the same verifier accessor as the deliverables gate, so there is one verifier for the gate and the engine. A nil value means the catalog-aware default. `PhaseIO` carries the `EVOLVE_PHASE_IO` stage into the reconcile rung, and the zero value (`StageOff`) changes nothing. `CompactPrompts` strips the on-demand reference tail of the agent document before dispatch. Its value comes from `workflow.compact_prompts` in `policy.json` and is never set to a bare literal here.
- **Profile.** `ProfileName` is the agent profile under which the phase runs. Its sandbox bounds what a lane can write.
- **Flag resolution.** The resolver of the extra flags has its own unit tests in `internal/phaseflags`. The `TestRun_*` tests here exercise it through `Run`.

## Invariants

- **An absent prompt key leaves the legacy prompt unchanged.** Pinned by the negative test of each block.
- **Agent-authored text is fenced as data.** Pinned by the fence assertions in `audit_repair_prompt_test.go` and `prior_findings_test.go`.
- **A lane never sees a foreign lane id.** Pinned by `lanescope_prompt_test.go`.

## Findings

- **cycle-776**: the lane scope was in `Context` since cycle 766, but only triage rendered it. The builder did not bind to its own lane. `lanescope_prompt_test.go` is the contract.
- **PR 503**: the hardening of the audit-repair loop shipped without a check that the reason reaches the prompt. `audit_repair_prompt_test.go` is that check.
