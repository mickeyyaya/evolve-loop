# internal/config

> The Loader design, the six warning codes, the pins and the declared deviations and quirks (D1-D9, Q1-Q10): [08-config.md](../decomposition/08-config.md). Dial defaults, escape hatches and flip evidence for the spine floor, the fatal pane, the gates and dynamic routing: [runtime-reference.md](../../operations/runtime-reference.md). The recovery dials: [ADR-0044](../adr/0044-unified-phase-recovery-protocol.md). This page keeps the package-level detail those do not.

## Purpose

`internal/config` resolves the routing configuration (`RoutingConfig`) once, at the composition root, and every consumer receives it by injection. It is the only reader of the routing env dials and of `docs/architecture/phase-registry.json`, so no consumer calls `os.Getenv` for these values. It also reads `.evolve/domain.json` (`LoadDomain`).

## Design

- **Two resolution steps.** `Loader.Load` applies the compiled defaults, then the registry, then the env dials, then the spine and inert-enable validators (precedence: env over registry over default). `Loader.ApplyPolicyStages` projects the root's `.evolve/policy.json` gate, recovery, router and parallel-evaluate dials over that value. The package-level `Load` is the Center-less facade that the per-phase callers keep.
- **Leaf package.** It imports the standard library, `signalcenter` and `paths` only (`importgraph_test.go`). `core` and `policy` both import it, so the policy dials cross as plain strings in the `PolicyStages` Parameter Object, and phase names cross as strings. Tests that need `core` or `policy` (`spine_contract_test.go`, `defaults_parity_test.go`) live in the external `config_test` package for the same reason.
- **The env dials are a Strategy table** (`envDials`), in the order their warnings appear. `model_routing` has no env dial. It is registry-only, because the flag-ceiling ratchet forbids new operator env flags.
- **Two stage ladders.** `GateStage` is the off/shadow/enforce trichotomy of the gate and recovery dials. Those dials either compute and log or act, so they have no advisory middle state. `RouterStage` is the full off/shadow/advisory/enforce ladder. The root's forwarders and the loader's parsers share both functions.
- **The rollout dials** (`RolloutStages`, embedded in `RoutingConfig`):

| Dial | Ladder | Default | Source | What it gates |
|---|---|---|---|---|
| `CommitEvidence` | gate | off | none (no key since X18c, and no reader) | Commit-as-evidence ([ADR-0027](../adr/0027-commit-as-evidence.md)): shadow computes git evidence beside the artifact; enforce makes the git evidence authoritative. |
| `ReviewGate` | gate | off | `gates.review_gate` | The per-phase review gate. Off keeps the no-op reviewer; at enforce a reject aborts the cycle. |
| `EvalGate` | gate | enforce | `gates.eval_gate` | `internal/evalgate`: Gate A (scout eval files exist) and Gate B (tdd predicate quality). It fails open on ambiguity. |
| `ContractGate` | gate | enforce | `gates.contract_gate` | The `internal/deliverable` contract gate ([ADR-0034](../adr/0034-unified-deliverable-contract.md)). A circuit breaker demotes enforce to advisory after N consecutive blocks. |
| `TriageCapGate` | gate | enforce | `gates.triage_cap_gate` | `internal/triagecap`: committed coverage floors above ceil(1.25·K) are rejected with a cap directive. |
| `TopNGate` | gate | enforce | `gates.topn_gate` | `internal/topngate`: a build whose task label is outside triage `top_n` is advisory (label drift); the TDD scope gate blocks a member-set mismatch, tests under an empty `top_n`, or a missing commitment record. |
| `PhaseRecovery` | gate | shadow | `recovery.phase_recovery` | The ADR-0044 program: the live channel, the ask-broker, the transient-dwell fast-fail, the chain-backed `StallPolicy`, the failure-adviser promotion and the misplaced-deliverable salvage. |
| `SpineFloor` | gate | enforce | `recovery.spine_floor` | Only whether a clean-absence handoff gap aborts the cycle. |
| `FatalPane` | gate | enforce | `recovery.fatal_pane` | Only whether a persisted, non-Busy fatal-pane match ends the stop-review wait. |
| `PhaseIO` | router | enforce | `gates.phase_io` | The unified phase-I/O envelope ([ADR-0050](../adr/0050-modularization-and-unified-phase-io.md)); off is the rollback. |
| `RouterReplan` | router, enforce unused | shadow | `router.router_replan` | The post-scout re-plan ([ADR-0052](../adr/0052-advisor-maximization.md)); at advisory it replaces the clamped plan after the floor re-clamps it. |
| `MergeGate` | router | shadow | compiled only | Merge-to-main promotion ([ADR-0057](../adr/0057-merge-to-main-gate.md)). No production code reads this field; only enforce is meant to auto-promote. |
| `ParallelEvaluate` | router | off | `parallel_evaluate.stage` | Concurrent post-build evaluate phases (archetype evaluate, excluding the audit verdict-brancher), merged serially with a weakest-link verdict. |
| `ScoutDecompose` | router | off | compiled only | Scout map-reduce: N scout-scan workers over codebase slices, bounded by `ScoutDecomposeConcurrency` (4). No production code reads either field. |

- **Plain bools, not stages.** `RoutingJudge` and `ReconDigest` cannot move behavior: the judge scores the route off the build path, and the recon digest injects facts that the floor still clamps. A shadow/advisory distinction would mean nothing.
- **Fallbacks for registry lists.** An empty `Order` falls back to the router's built-in canonical order. An empty `SpineOrder` falls back to the kernel's spine literal. An empty `LegalSuccessors` falls back to the kernel's literal graph, and the load-time `ValidateSafetyInvariants` validator gates any operator edit to it. `LegalSuccessors` keys by phase name, including the `start`/`end` sentinels and the `debugger` control phase, so the whole graph has one source ([ADR-0060](../adr/0060-data-driven-transition-kernel.md)).
- **`CompactPrompts`** flows `config.Load` → `RoutingConfig.CompactPrompts` → the phase `Config.CompactPrompts` → `runner.Options.CompactPrompts`, and is never a literal in a phase constructor. It strips everything from "## Reference Index (Layer 3, on-demand)" onward in disk-loaded agent docs.
- **`GoalRecipes`** is the one source projected into the persona's "## Goal-Type Recipes" table (`router.RenderRecipeProjection`, locked by `TestRouterPersonaRecipeTable_NoDrift`) and read by the RecipeVerifier.
- **`RubricHint`** lines render into the advisor's decision rubric, so rubric guidance is registry data. A hint also decides admission on the plan path: `RoutingBlock.TriggerIsTheWholeRule` holds for an `insert_when` with no hint, and the router then skips a planned phase whose trigger does not fire (`insert-when-gates-plan`, ADR-0052 amendment 2026-09-28), so a hint that only restates its trigger still opens the phase to the advisor. A rubric-only routing block is walk-inert: mandatory phases never consult `Triggers`, and an empty `insert_when` never fires (`TestWalk_MandatoryPhaseWithRubricOnlyRoutingBlockUnchanged`).
- **`LoadDomain` and `DefaultDeliverableKind`.** Writing and research projects default to document deliverables. Everything else defaults to code, the conservative side that keeps the tdd pin ([ADR-0099](../adr/0099-deliverable-kinds.md)). Only the `domain` field is read; the other fields are parsed for round-trip fidelity.

- **Small contracts the code cannot show.**
  - `mustCondRule` is the only panic of the package. It fires for a compiled expression that does not parse.
  - `parseCondClause` matches the two-character operators before the one-character ones.
  - `applyEnv` stamps the env var name on the warnings of each dial. Thus `fields.key` names the var, and the message keeps its registry spelling.
  - The message of `parseMode` names the registry key, also for the env dial.
  - Tests build `PolicyStages` by position, so a new field breaks the build.
  - `applyRegistryDials` uses pointer fields to tell an explicit 0 or false from absence.
  - `applyDeliverableKinds` installs a spec also when it warns. The registry owns the root and `min_options`, so a consumer must see the hole and not a silent default.
  - `staticSpinePhases` is a local copy, because `config` cannot import `core`. `TestStaticSpineMatchesStateMachine` pins it.
  - A missing `signalCodes` entry reaches the Center empty and is stamped `SIGNALCENTER_MISSING_CODE`.
  - The `config.warning` event has no cycle number, because the loader runs before one exists.
  - The kitchen-sink golden pins the order of `envDials`.
  - The gate dials of `defaultRollout` must equal the compiled defaults of `policy`. `ApplyPolicyStages` resolves them again at the root. `TestDefaults_GateDialsMatchPolicyCompiledDefaults` pins this.
  - `ModelRoutingAdvisory` logs the tier proposal of the advisor for each phase and never applies it. `ModelRoutingAuto` applies it as a soft overlay that `router.ClampPlanModelRouting` clamps. Since X19a the proposal has no CLI.
  - `AuditFailRoutesTo` comes from the `failure_floor` of the policy, and `LegalSuccessors` edits are gated by `ValidateSafetyInvariants`.
- **The divergence is checked by behaviour.** `acs/cycle1141` (`TestC1141_005`) asserts that the compiled routing `Mandatory` adds exactly `ship` beyond the phases of `phasecontract.RequiredRoles()`. It no longer reads `config.go` as text. `config.go` must stay free of the four retired router env names that `acs/cycle37` searches for.

## Invariants

- **Unknown words take the fail-safe side, with a warning.** A stage word resolves to off, `routing_mode` to llm, `model_routing` to static, and `enabled` to content. A bad conditional rule or insertion cap is ignored. A typo must never silently enable a kill path or a staged rollout.
- **A conditional rule never shrinks.** One malformed clause fails the whole `&&` rule. Registry rules merge over the compiled ones, so the tdd default survives a registry that omits it. `CondRule.String` renders a rule in the registry's own spelling (`cycle_size!=trivial && deliverable_kind!=document`), so a prompt that quotes the rule quotes the declaration (`TestCondRuleString_ReproducesTheRegistryExpression`).
- **`DefaultTddRuleExpr` never lags the registry.** A project with no registry file runs on it. X18d removed the `EVOLVE_USE_PHASE_REGISTRY` switch, so the loader always reads the file when it exists. `TestDefaults_TddRuleMatchesRegistry` pins the two equal.
- **`Mandatory` deliberately differs from `phasecontract.RequiredRoles()`.** `RequiredRoles` answers "what must a completed cycle have produced": the report-bearing spine phases that cyclehealth, redteamcheck and ledgerverify read. `Mandatory` answers "what must the router always plan", a superset that includes `ship`, which writes no report. A `Mandatory` derived from `RequiredRoles` drops `ship` from every plan. The binding invariant is that `Mandatory` is the registry-required phases plus `ship`, and nothing more. `TestC1141_005` asserts it.
- **The compiled baseline omits triage.** The registry adds it to `mandatory_phases`, and tests that build a `RoutingConfig` directly get the four-phase baseline. That gap is why an unreadable or malformed registry warns loudly.
- **`ModelRouting` is orthogonal to `Stage` and `Mode`.** `Stage` decides which phases run, `Mode` picks the routing brain, and `ModelRouting` decides who picks the model tier for a phase. Parsing or applying one never reads or writes the others (`TestC436_015`). The zero value is static: tiers stay profile-pinned. Advisory logs the tier proposal of the advisor; auto applies it as a soft overlay clamped by `router.ClampPlanModelRouting`. Since X19a the routing data picks the CLI, and the advisor picks only the tier.
- **Composition-root views.** `CommitEvidence` has no consumer since X18c. The sandbox mode is not a field of this package since X18d: the bridge reads `sandbox.mode` of policy.json through `bridge.Deps.SandboxMode`. `PhaseRecovery` has no env var: the root forwards the resolved value to the bridge (`wireBridgeStages`) and to the observer's IPC stage key.
- **Inert enables.** Below advisory the static state machine drives, so a force-enabled phase that is neither mandatory nor in `staticSpinePhases` never runs, and `validateInertEnables` warns. Shadow counts as below advisory: the router only computes and logs there. `staticSpinePhases` is a local copy because config cannot import core; `TestStaticSpineMatchesStateMachine` pins it to the state machine.
- **Spine order.** `core.SpineSatisfiedUpTo` walks the mandatory anchors by their configured position, so an order with ship before audit would let ship skip the shippable-audit check. The legality graph and the audit verdict branch still block it; the `spine-order` warning keeps the order from being the only guard.
- **One appender.** `warn` copies the producer's fields map, so the range stamper can add step, source, key and path without touching the caller's map. Only `warn` mints a `Warning`.
- **Source-text tests read comments too.** `TestNoStderrOrWritesInTheLeaf` fails if a non-test file contains `os.Stderr`, `fmt.Fprint`, `os.WriteFile`, `os.Create` or `os.MkdirAll`, and `TestNoBareWarningLiteralOutsideTheAppender` counts `Warning{`. Both match comment text as well as code. `TestLimits_FunctionsFilesAndNestingStayWithinTheBar` counts comment lines inside a function body toward the 50-line limit.
- **`LoadDomain` tells absence from damage.** An absent `domain.json` is the ordinary case. A file that exists but cannot be read or parsed is an error, so a writing project with a trailing comma never silently becomes a code project.
- **`RePlanMaxDepth` defaults to 1**: one measured re-plan per cycle, then escalation to the debugger rather than thrashing.

## Findings

- **Advisory routing by default.** The compiled `Stage` default moved from off to advisory after the advisory mode soaked from cycle-108 onward ([cycle-108-routing-live-data.md](../../research/cycle-108-routing-live-data.md)). The integrity floor (`ClampPlanToFloor` and `SpineSatisfiedUpTo`), not a flag, protects the ship guarantee.
- **cycles 263/264**: the advisory router skipped triage on the premise that scout picks one item, while scout had authored three tasks. Without the scope clamp the builder under-delivered and the all-or-nothing audit failed the cycle. Triage joined the registry's mandatory spine. Triage's own runner-level auto-skip still short-circuits trivial cycles, so the clamp stays cheap.
- **cycle-120**: a phase force-enabled under default routing never ran and the operator could not tell why. That produced the inert-enable warning.
- **cycle-217**: the refactor recipe needs six optional insertions, so the registry raised `max_optional_insertions` from 4 to 6 (micro-phase catalog §4.2). The compiled default stays 4.
- **cycle-440**: the scout report and the eval placed the `model_routing` default flip in `.evolve/policy.json`, but the value is parsed only from the registry's `config.model_routing`. The tests target the file the code actually reads.
- **cycle-1141**: the audit of `Mandatory` against `phasecontract.RequiredRoles()` kept the two separate on purpose (see Invariants).
- **Retired env flags stay inert.** The phase-enable flags (`EVOLVE_TRIAGE_DISABLE`, `EVOLVE_TEST_PHASE_ENABLED`, `EVOLVE_BUILD_PLANNER`, `EVOLVE_PLAN_REVIEW`) moved to `WorkflowPolicy.PhaseEnables` in cycle-39. The gate env keys, `EVOLVE_REVIEW_GATE` among them, moved to policy in cycle-34. `EVOLVE_PHASE_RECOVERY` went in the cycle-12 flag retirement. In-package tests pin that these keys change nothing and raise no warning.
- **Split dials.** `PhaseRecovery` is overloaded, so the spine floor (R8.5) and the fatal-pane fast-fail (F27) each got a dial of their own instead of flipping it. The flip evidence is in runtime-reference.md.
- **`TopNGate`** exists because out-of-lane builds (a build-report task outside triage `top_n`) recurred eight times before the gate landed.
- **`ParallelEvaluate`** concurrency 3 is the soak sweet spot: about an 11% projected fleet saving, diminishing past it.
- **`CompactPrompts`** (cycle-413) defaults on and strips about 23 KB of reference tails per cycle. The in-package tests target the silent-false path, where a default-true field turns false on a missing key. The strip itself once cut the auditor to 27% of its persona: [2026-08-10-persona-strip-lobotomy.md](../../incidents/2026-08-10-persona-strip-lobotomy.md).
