# internal/phases/tdd

> Related: [ADR-0098](../adr/0098-task-contract-block.md) (the task contract block). This page keeps the package-level detail that the code cannot say.

## Purpose

`internal/phases/tdd` is the RED test-writer phase. The boilerplate of a phase lives in `internal/phases/runner`. This package encodes only the variation points of the tdd phase.

## Design

- **Skip.** `ShouldSkip` delegates to the central phase policy. `config.Load` is the only reader of `EVOLVE_TEST_PHASE_ENABLED`. The legacy posture stays: tdd runs unless the flag disables it. The skip result is SKIPPED with next phase build and no bridge call. From the Enforce stage up, the conditional pin (`cycle_size != trivial`) stops the flag from disabling tdd.
- **Verdict mapping.** An empty artifact is FAIL. A report without an acceptance section is FAIL. A report without a RED-tests section is FAIL.

  Otherwise the result is PASS. `classify` equals `phasecontract.TDD.Complete`. The producer-side contract test shares the accepted headings.

  On cycle 192 the headings drifted (`## RED Run Output`, `## Test Files Written`, `## AC-Materialization`, `## Coverage Map`). The classifier accepts the current variants and the legacy `## Acceptance` and `## RED Tests`. Pinned by `TestClassify_HeadingVariants`.
- **The prompt.** `ComposePrompt` equals `runner.BaseCycleContext(body, req)` plus the one extra of this phase, the worktree line. Byte equality is the guarantee that the refactor changed structure and not prompts (`compose_parity_test.go`). With no worktree the prompt is the bare core block.
- **Lane scope.** The composed prompt renders the pinned lane scope as a `fleet_scope` line, so predicates bind only to the todo ids of this lane. Cycle 766 put the scope into the context, and only triage rendered it. With no lane scope there is no line, and a sequential cycle is byte-identical.
- **Protected predicates.** The tdd phase does not check the paths that its predicates read. The orchestrator does it after the phase ends (`core.routeProtectedPredicates`). A predicate that reads a protected file stops the cycle as a FAIL with a route code. The closeout routes the item. See [internal-cycleoutcome.md](internal-cycleoutcome.md#protected-predicate-route-tdd).
- **Audit repair.** A re-dispatch after an audit rejection gives the agent the reason of the audit, so the repair is targeted and not blind. The text is fenced as data, because it quotes lines that agents wrote and must never read as instructions. An absent key keeps the legacy prompt byte-identical.

  The heading is harness-owned (ADR-0098). `core` composes the preamble and the block once, so tdd, build and audit read identical words. The proof is a real string from the rejection of cycle 1574, so the check is not tautological.

  Cycle 1577 resumed that failed task. Its scout, fault-localization and bug-reproduction prompts had zero references to the failure.
- **Standing findings.** On cycle 1679 an audit passed WARN with three MEDIUM defects. The ship hit a fleet rebase need and recovered through a rebuild. The brief of that rebuild had no audit section, so round 5 found the same defects again. The same rubric audits a recovery rebuild again. So the last actionable findings of the audit travel in the brief as data.
- **Options.** `ContractVerifier` is the accessor of the deliverables gate for the verdict engine (`runner.Options.ContractVerifier`), so the gate and the engine share one verifier. A nil value selects the catalog-aware default. `CompactPrompts` strips the on-demand reference tail from the disk-loaded agent doc. Its value comes from `workflow.compact_prompts` in `policy.json` and is never a literal in code (the standing rule: phase settings come from config).

## Invariants

- **The artifact name comes from the `phasecontract` registry.** The registry is the single source for the name that the tdd-engineer agent doc and the downstream build-planner and builder contract use. The stale name `team-context.md` from before the rewrite left the bridge polling a file that the agent never writes. Every tdd phase then timed out (exit 81) after it finished its work.
- **`ArtifactName`, not `ArtifactFilename`.** The registered artifact name of tdd differs from the `<phase>-report.md` convention. If the registration is ever lost, the convention fallback silently brings back `tdd-report.md` and the same exit-81 timeout. `ArtifactName` returns the empty string, which fails loudly.
