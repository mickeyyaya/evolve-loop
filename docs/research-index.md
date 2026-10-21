# Research Index

> Reference documents available to evolve-loop. The split:
>
> - **Active references** (5 files; 4 in `docs/private/research/archived-2026-05-19/`,
>   `eval-grader-best-practices.md` at `docs/`) — cited by runtime
>   personas/skills/scripts. (`docs/research/` itself now holds the merged
>   research tree — see the 2026-08-05 section below.)
> - **Archived references** (42 files, in `docs/private/research/`) — for
>   contributor reference; explicitly excluded from agent context through the
>   trust kernel (deny_subpaths + Layer-B filter). See
>   [docs/architecture/private-context-policy.md](architecture/private-context-policy.md)
>   for the convention.

---

## Summary Statistics

| Bucket | Path | Documents | LOC |
|---|---|---|---|
| Active | `docs/private/research/archived-2026-05-19/` + `docs/` | 5 | 1,220 |
| Archived | `docs/private/research/` | 42 | 7,737 |
| **Total available** | — | **47** | **8,957** |

---

## Active Reference Documents

These load into agent runtime context. Cited by the listed runtime artifact.

| Doc | Purpose | Used By |
|-----|---------|---------|
| [accuracy-self-correction.md](private/research/archived-2026-05-19/accuracy-self-correction.md) | CoT verification and anti-conformity checks | evolve-auditor.md |
| [eval-grader-best-practices.md](eval-grader-best-practices.md) | Eval grader precision and mutation resistance | eval-runner.md |
| [evaluator-research.md](private/research/archived-2026-05-19/evaluator-research.md) | Evaluator agent design rationale — 14 papers, 8 benchmarks | evaluator/SKILL.md |
| [performance-profiling.md](private/research/archived-2026-05-19/performance-profiling.md) | Token attribution and cost baselines | docs/index.md |
| [token-optimization-guide.md](private/research/archived-2026-05-19/token-optimization-guide.md) | Per-cycle token + cost optimization | docs/index.md |

---

## Archived Research (in `docs/private/research/`)

Restored verbatim from commit `35b31c4^` (cycle 13's parent). Cycle 13
correctly deleted these from runtime context per Liu and others (2023) "Lost in
the Middle"; v9.1.x re-introduced them as developer-only reference under
`docs/private/`. Agents never see these during cycles. Contributors
read them directly.

Grouped by theme:

### Agent architecture & capabilities

| File | Topic |
|---|---|
| [agent-capability-benchmarking.md](private/research/agent-capability-benchmarking.md) | Capability measurement frameworks |
| [agent-role-specialization.md](private/research/agent-role-specialization.md) | Role decomposition patterns |
| [agent-skill-composition.md](private/research/agent-skill-composition.md) | Skill composition + selection |
| [agent-state-persistence.md](private/research/agent-state-persistence.md) | State models, durable execution patterns |
| [agent-testing-frameworks.md](private/research/agent-testing-frameworks.md) | Test harness patterns for agents |
| [emergent-agent-behaviors.md](private/research/emergent-agent-behaviors.md) | Emergence + unintended capabilities |
| [agent-lifecycle-management.md](private/research/agent-lifecycle-management.md) | Lifecycle stages + transitions |

### Multi-agent systems & coordination

| File | Topic |
|---|---|
| [agent-collaboration-games.md](private/research/agent-collaboration-games.md) | Multi-agent interaction games |
| [agent-orchestration-anti-patterns.md](private/research/agent-orchestration-anti-patterns.md) | Anti-patterns in orchestration |
| [multi-agent-blackboard.md](private/research/multi-agent-blackboard.md) | Blackboard / shared-state pattern |
| [reasoning-orchestration-patterns.md](private/research/reasoning-orchestration-patterns.md) | Reasoning chains across agents |

### Autonomous loops & self-improvement

| File | Topic |
|---|---|
| [autonomous-experiment-loops.md](private/research/autonomous-experiment-loops.md) | Self-improving loop protocols |
| [self-evolving-tool-creation.md](private/research/self-evolving-tool-creation.md) | Tool/gene library evolution |
| [self-healing-agents.md](private/research/self-healing-agents.md) | Recovery + self-repair |
| [prompt-evolution-optimization.md](private/research/prompt-evolution-optimization.md) | Promptbreeder-style evolution |

### Economics & deployment

| File | Topic |
|---|---|
| [agent-economics.md](private/research/agent-economics.md) | Unit economics, cost amplification |
| [agent-deployment-patterns.md](private/research/agent-deployment-patterns.md) | Production deployment shapes |
| [agent-config-versioning.md](private/research/agent-config-versioning.md) | Config versioning + migration |
| [token-cost-optimization.md](private/research/token-cost-optimization.md) | Token-budget patterns |

### Trust, safety, governance

| File | Topic |
|---|---|
| [agent-governance-compliance.md](private/research/agent-governance-compliance.md) | Compliance + governance frameworks |
| [agent-interpretability.md](private/research/agent-interpretability.md) | Interpretability techniques |
| [agent-output-validation.md](private/research/agent-output-validation.md) | Output validation strategies |
| [agent-sandboxing-patterns.md](private/research/agent-sandboxing-patterns.md) | Sandboxing approaches |
| [reward-hacking-prevention.md](private/research/reward-hacking-prevention.md) | Reward-hacking detection + prevention |
| [hitl-trust-calibration.md](private/research/hitl-trust-calibration.md) | Human-in-the-loop trust calibration |

### Memory, context, retrieval

| File | Topic |
|---|---|
| [memory-consolidation-pipeline.md](private/research/memory-consolidation-pipeline.md) | Memory consolidation across cycles |
| [agentic-rag-patterns.md](private/research/agentic-rag-patterns.md) | RAG patterns for agents |
| [context-engineering-patterns.md](private/research/context-engineering-patterns.md) | Context engineering techniques |
| [long-context-agent-strategies.md](private/research/long-context-agent-strategies.md) | Long-context utilization |
| [knowledge-distillation-agents.md](private/research/knowledge-distillation-agents.md) | Distillation for agent systems |

### Interfaces & ecosystem

| File | Topic |
|---|---|
| [agentic-ide-integration.md](private/research/agentic-ide-integration.md) | IDE integration patterns |
| [agent-interoperability-protocols.md](private/research/agent-interoperability-protocols.md) | A2A / MCP-style protocols |
| [agentic-systems-roadmap.md](private/research/agentic-systems-roadmap.md) | Ecosystem roadmap notes |
| [ai-code-review-agents.md](private/research/ai-code-review-agents.md) | Code review agent designs |
| [workflow-dag-patterns.md](private/research/workflow-dag-patterns.md) | Workflow DAG patterns |

### Code generation & refactoring

| File | Topic |
|---|---|
| [code-correctness-verification.md](private/research/code-correctness-verification.md) | Correctness verification |
| [constrained-decoding-patterns.md](private/research/constrained-decoding-patterns.md) | Constrained decoding |
| [refactoring-llm-research.md](private/research/refactoring-llm-research.md) | LLM-driven refactoring research |
| [refactoring-pipeline-architecture.md](private/research/refactoring-pipeline-architecture.md) | Refactoring pipeline shape |
| [refactoring-tools-landscape.md](private/research/refactoring-tools-landscape.md) | Tool landscape |
| [synthetic-data-generation.md](private/research/synthetic-data-generation.md) | Synthetic data techniques |
| [agent-failure-tracing.md](private/research/agent-failure-tracing.md) | Failure tracing + classification |

## Merged 2026-08-05 (from kb/research and knowledge-base/research)

Research packages: [code-audit-2026-07](research/code-audit-2026-07/README.md) ·
[deliverable-alignment-2026-08](research/deliverable-alignment-2026-08/README.md) ·
[failed-loop-analysis-2026-07](research/failed-loop-analysis-2026-07/README.md) ·
[graph-engineering-2026-08](research/graph-engineering-2026-08/README.md) ·
[llm-output-stability-2026-07](research/llm-output-stability-2026-07/README.md) ·
[merge-concurrency-2026](research/merge-concurrency-2026/README.md) ·
[token-concurrency-2026](research/token-concurrency-2026/README.md) ·
[token-optimization-2026](research/token-optimization-2026/README.md) —
plus three note dirs without READMEs: `research/coding-craft-2026/`,
`research/fable-simulation-2026/`, `research/tmux-live-capture-2026-06-04/`.

The architecture reviewer's comparison with five popular online review prompts,
blind-judged on three scenarios (2026-09-28), is
[research/coding-craft-2026/architecture-reviewer-comparison.md](research/coding-craft-2026/architecture-reviewer-comparison.md).

Plus 12 single-file notes from knowledge-base/research (lessons-and-resolutions,
verdict-classifier drift, flag-reduction design, token histories, and others) — see
`docs/research/`.

**Dated entries.** The sections below the chronicle come from one file for each entry in [research-index/](research-index/). To add an entry, add a file there and run `evolve docs index generate`. Do not edit the generated region by hand. After a rebase, `evolve pr refresh` writes it again ([plan P3](plans/pr-merge-latency-2026-10.md)).

**The engineering chronicle** — workstream-level narratives (problem /
approaches / decision / results / retro) — lives at
[docs/chronicle/](chronicle/README.md).

<!-- GENERATED:research-index BEGIN — do not edit by hand; run `evolve docs index generate` -->

## 2026-09-02 — ship-rate hardening + pipeline dashboard

| Document | What it is |
|---|---|
| [research/ship-rate-harness-reliability-2026-09-02.md](research/ship-rate-harness-reliability-2026-09-02.md) | Synthesis: measured ship rate (19.6 %, 0/11 streak), eleven source-verified architectural gaps, failure-bucket census, eight ranked proposals (R1/R2 implemented, R3–R8 filed). |
| [research/ship-rate-harness-reliability-2026-09-02-sources.md](research/ship-rate-harness-reliability-2026-09-02-sources.md) | Literature survey, 60 sources: SWE-agent ACI ablations, self-repair limits, architect/editor split, best-of-N with verifiers, over-claiming incentives, deterministic hooks, verifier isolation, cascades. |
| [research/pipeline-dashboard-patterns-2026-09-02.md](research/pipeline-dashboard-patterns-2026-09-02.md) | UI/observability patterns (~45 sources) behind ADR-0095: trace/session model, lanes not trees, immutable retry rounds, Sentry-style fingerprint groups, SSE mechanics for a Go single binary. |
| [superpowers/specs/2026-09-02-ship-rate-harness-and-pipeline-dashboard-design.md](superpowers/specs/2026-09-02-ship-rate-harness-and-pipeline-dashboard-design.md) | The design spec (both sub-projects, TDD protocol, named patterns, clean-code limits). |

## 2026-09-30 — comment policy, whole-tree tests, inbox and CLI coverage

| Document | What it records |
|---|---|
| [research/cli-coverage-inventory-2026-09-30.md](research/cli-coverage-inventory-2026-09-30.md) | The inventory checks 147 core functions against the published `evolve` CLI. The CLI runs 64 fully, 36 partly, and 47 not at all. Each row has file:line evidence. The inventory gave 50 CLI requests (PR #750). |
| [reports/inbox-review-2026-09-30.md](reports/inbox-review-2026-09-30.md) | The queue planned 0 batches from 183 items. A six-agent check against main gave a verdict for each item. A drain then opened the queue to 29 batches (PRs #745, #747). |
| [reports/comment-round-12-findings-2026-09-30.md](reports/comment-round-12-findings-2026-09-30.md) | The 43 findings of comment reduction round 12 (PR #749), including two `scopedelta` security holes, and the inbox item that carries each. |
| [reports/comment-strip-clean-code-review-2026-09-30.md](reports/comment-strip-clean-code-review-2026-09-30.md) | A Clean Code review of 64 stripped production files. No file reads fully without its comments. Two thirds of the removed comments carried intent, invariants, or design. The review lists the defects that the comments hid. |
| [incidents/2026-09-30-a-lane-ship-grew-a-function-past-the-size-ratchet.md](incidents/2026-09-30-a-lane-ship-grew-a-function-past-the-size-ratchet.md) | Cycle 1779's lane ship turned main red through a whole-tree test its change scope never selected; the class fix (PR #751). |
| [architecture/adr/0110-whole-tree-tests-run-before-main.md](architecture/adr/0110-whole-tree-tests-run-before-main.md) | Decision: every test that reads the whole tree runs before main, and an AST detector keeps the scanner pack complete. |
| [architecture/adr/0111-code-carries-no-comments.md](architecture/adr/0111-code-carries-no-comments.md) | Decision: code carries no comments. Their content moves to code, tests, design notes, and the history archive. The commit gate refuses an added comment (PRs #746, #749, #753). |
| [architecture/adr/0114-the-acs-verdict-is-always-written-and-complete.md](architecture/adr/0114-the-acs-verdict-is-always-written-and-complete.md) | Decision: every ACS run writes a complete verdict. A scope that cannot run becomes a named `egps/` harness red with its cause. The lane budget is one shared deadline. Predicates run in a scrubbed environment that the suite owns. |
| [architecture/adr/0115-commit-gate-keeps-history-and-records-waivers.md](architecture/adr/0115-commit-gate-keeps-history-and-records-waivers.md) | Decision: the commit gate refuses a removed history comment that its change does not record. It records a review waiver as a `Review-waived` trailer. Pure-docs changes keep their review. The added-comment rule does not apply to `docs/`. |
| [architecture/adr/0116-inbox-curation-verbs.md](architecture/adr/0116-inbox-curation-verbs.md) | Decision: curate inbox items through `evolve inbox show/list/edit/withdraw/verify`. Each item field has one owner: authored, curated, or verb-owned. The gate judges an edit on the fields that it writes. A withdrawal undoes a filing. A premise check is a lifecycle stamp. |
| [architecture/adr/0117-pre-handoff-probes-run-the-build-floor.md](architecture/adr/0117-pre-handoff-probes-run-the-build-floor.md) | Decision: `evolve selfcheck build` and `evolve phase verify build` run the build handoff floor itself. They use one named check list with the inputs of the cycle. Three cycles (1763, 1764, 1788) failed on a floor check that a probe showed as green. |

## 2026-10-07 — convergence policy (ADR-0126)

| Document | What it records |
|---|---|
| [research/review-convergence-limited-rounds-2026-10.md](research/review-convergence-limited-rounds-2026-10.md) | Research: the best policy to converge review feedback in limited rounds. Two rounds capture 76–95% of the gain, so stop on marginal gain and keep the best round (VRR-Stop). LLM reviewers vary across runs, so verify findings and do not search again (OpenCodeReview). More revisions do not converge without less uncertainty (33,596 agent PRs). Google asks for "better, not perfect", and effort rises before the model, for reasoning failures only. The research maps to refinements CR1–CR8. |
| [incidents/2026-10-07-l2-seven-review-rounds.md](incidents/2026-10-07-l2-seven-review-rounds.md) | The example issue: the L2 routing lane needed seven review fix rounds on the deepest Claude model. Four causes: a review added a capability, a heuristic over an unbounded input, delta checks that hunted instead of verifying, and nothing changing between rounds. |
| [architecture/convergence-policy.md](architecture/convergence-policy.md) | Design: one pure `convergence.Decide` for every repeat-until-accepted loop. A three-rung ladder: change the feedback, then the strategy, then the scope. Stop on marginal gain, land the best round, and never run a fourth. Output first, with deferrals filed as follow-ups. Headroom is read from the manifests. |
| [plans/convergence-policy-2026-10.md](plans/convergence-policy-2026-10.md) | Plan: strategies A to C, decisions V-D1 to V-D11, and CQ1. In CQ1 the operator sets top to Opus 5.5 at xhigh effort. Components are V0 to V9 plus V3b, which lands on the console first. The L2 replay is the acceptance test. It lands at round 3 through rung 2. |
| [architecture/adr/0126-every-iterative-loop-converges-or-escalates.md](architecture/adr/0126-every-iterative-loop-converges-or-escalates.md) | Decision: every iterative loop converges or escalates. The order is feedback, then strategy, then scope. The loop lands the best round and never runs a fourth round. Capability findings below CRITICAL never block. Judges verify after `J_0`. A CRITICAL finding is never deferred. |

## 2026-10-07 — the code-review loop and the audit evaluation contract

| Document | What it records |
|---|---|
| [research/audit-evaluation-contract-prior-art-2026-10.md](research/audit-evaluation-contract-prior-art-2026-10.md) | Research: how AI software factories publish the standard of an evaluator to the agents that it judges. The sources are Anthropic, OpenAI, StrongDM, Stripe, and Augment. The 2026 evaluator research includes SpecBench, ImpossibleRubrics, PROCTOR, Agentic Rubrics, and self-preference bias. The verdict: contract before code, public criteria, private instruments, evidence as certificates, and facts over the judge. The research maps to refinements AR1–AR9. |
| [architecture/review-loop-and-quality-index.md](architecture/review-loop-and-quality-index.md) | Design: the independent `code-review` loop between build and audit, and the ten-dimension quality index the review and the audit share (ADR-0124). |
| [plans/code-review-phase-2026-10.md](plans/code-review-phase-2026-10.md) | Plan: the code-review phase's strategies (S1–S3), operator decisions, components and landings. |
| [architecture/adr/0124-code-review-phase.md](architecture/adr/0124-code-review-phase.md) | Decision: an independent code-review loop sits between build and audit, and the audit qualifies its result on one shared quality index. |
| [architecture/audit-evaluation-contract.md](architecture/audit-evaluation-contract.md) | Design: the published standard of the audit, with dimensions and `G-…` gate criteria that have verifiers and certificates. The `audit-plan` phase writes the expectations of each cycle before build. Delivery uses capped digests. A kernel cross-check holds the final audit to its plan (ADR-0125). |
| [plans/audit-evaluation-contract-2026-10.md](plans/audit-evaluation-contract-2026-10.md) | Plan: approaches A to C, decisions E-D0 to E-D9, refinements AR1–AR9, and components E0 to E7. The operator made decisions E-D1 to E-D4. E0 has its own docs landing. E1 to E6 land inside landing 2 of the code-review plan. E7 lands at its flip. |
| [architecture/adr/0125-audit-publishes-its-evaluation-contract.md](architecture/adr/0125-audit-publishes-its-evaluation-contract.md) | Decision: the audit publishes its standard, and before build it publishes the expectations of this cycle. Its verdicts link to published criteria. The criteria are public and the probe instances are private. |

## 2026-10-08 — logging and process hygiene

| Document | What it records |
|---|---|
| [research/logging-and-process-cleanup-2026-10.md](research/logging-and-process-cleanup-2026-10.md) | Research: structured logging in Go, log categories and retention, and process-tree cleanup on Linux and macOS. It also covers MCP server shutdown, harness cleanup practice and log reading for agents. Findings F1.1 to F6.8, refinements R1 to R21. |
| [plans/logging-and-process-hygiene-2026-10.md](plans/logging-and-process-hygiene-2026-10.md) | Plan: the log catalog, a log dir for each loop launch, the ship gate output split and keep-on-fail for raw tool output (lane H1). It also covers process hygiene (H2) and the layered reader (H3, H4). Decisions K1 to K13, each with a status. |

## 2026-10-09 — event channels (ADR-0127)

| Document | What it records |
|---|---|
| [research/event-notification-protocol-2026-10.md](research/event-notification-protocol-2026-10.md) | Research: kernel file notification (kqueue, inotify, fsnotify, FSEvents, pidfd) and the cost of local IPC. It also covers topic and channel prior art: Kafka, NATS, Redis Streams, MQTT, D-Bus, Kubernetes and etcd. Findings F1.1 to F7.3, an adopt-or-reject table, refinements R1 to R24. |
| [architecture/event-channels.md](architecture/event-channels.md) | Design, the spec of record: channel logs with byte cursors, a locked append, two QoS classes and the kernel wake (arm, catch up, wait). It also covers the filter grammar, subscriptions, delivery, consumer groups, retention, the `evolve events` verbs, the exit codes and the limits. |
| [plans/event-notification-protocol-2026-10.md](plans/event-notification-protocol-2026-10.md) | Plan: the poll audit, three candidate designs with scores, the operator decisions, decisions D1 to D30 and the Signal Center through the channels. It also lists the migrations, and components E0 to E14 with their red tests. |
| [architecture/adr/0127-push-only-event-channels.md](architecture/adr/0127-push-only-event-channels.md) | Decision: events reach other programs through channel logs that the kernel wakes, never through a poll, with the Signal Center as the one producer path. |

## 2026-10-09 — concurrent cycle landing (ADR-0128)

| Document | What it records |
|---|---|
| [research/concurrent-cycle-landing-2026-10.md](research/concurrent-cycle-landing-2026-10.md) | Research: a census of 46 fleet-rebase recoveries, at 24.2 min and 1.49 LLM phases each. It shows why cycle 1843 was not proven identical and why the prefix queue of ADR-0078 is off. It also covers merge queues, agent harnesses, test selection, the cost model and five approaches. Findings F2.1 to F9.3, refinements R1 to R33. |
| [architecture/fleet-landing-queue.md](architecture/fleet-landing-queue.md) | Design, the spec of record: the durable landing queue, the composition with `merge-tree`, the overlap proof and the tiers T1 to T4. It also covers the scoped gates, the interaction review, ejection, budgets, signals, config keys, failure modes and limits. |
| [plans/concurrent-cycle-landing-2026-10.md](plans/concurrent-cycle-landing-2026-10.md) | Plan: operator decisions O1 to O13, console decisions CD1 to CD4, decisions D1 to D53, and components Q0 to Q19 with red tests. It also holds the migration from the current ladder, the rollout from shadow to enforce with its metrics, and the risks. |
| [architecture/adr/0128-landing-queue-tiered-reverification.md](architecture/adr/0128-landing-queue-tiered-reverification.md) | Decision: fleet lanes land through one queue, and their overlap with the peers picks the re-verification. It amends ADR-0078, ADR-0105 and ADR-0049. |

## 2026-10-09 — cycle agent messaging (ADR-0129)

| Document | What it records |
|---|---|
| [research/agent-messaging-2026-10.md](research/agent-messaging-2026-10.md) | Research: agent-to-agent protocols (A2A, MCP, Claude Code teams and messages, AutoGen, MetaGPT, CAMEL) and scope (NATS, MQTT, Kafka). It also covers identity under one uid, delivery to a busy LLM agent, safety and scale. Findings F1.1 to F7.5, an adopt-or-reject table, refinements R1 to R24. |
| [architecture/agent-messaging.md](architecture/agent-messaging.md) | Design, the spec of record: the `msg/1.0` envelope, the address grammar, the identity with the outbox location and the token, and the policy rules. It also covers the flow, the store on the ADR-0127 log, the delivery adapters, the limits, the `evolve msg` verbs, the paths and the threat model. |
| [plans/agent-messaging-2026-10.md](plans/agent-messaging-2026-10.md) | Plan: operator decisions OD1 to OD4, four candidate designs with scores, decisions D1 to D15 and the skeleton. It also lists the wiring seams, and components M0 to M8 with their tests. |
| [architecture/adr/0129-cycle-scoped-agent-messaging.md](architecture/adr/0129-cycle-scoped-agent-messaging.md) | Decision: the agents of one cycle send messages through a broker of that cycle, with a general protocol and a scoped policy. It amends ADR-0127 and ADR-0064. |

## 2026-10-09 — a retry carries the evidence of the dispatch that it replaces (ADR-0130)

| Document | What it records |
|---|---|
| [research/attempt-postmortem-2026-10.md](research/attempt-postmortem-2026-10.md) | Research: the cycle 1853 chain, the records that the repository writes today, and how agent harnesses carry failure context into a retry. It covers Reflexion, SWE-agent, OpenHands, Claude Code, Aider, LangGraph and same-input retry. Findings F1.1 to F3.10, refinements R1 to R9. |
| [plans/attempt-postmortem-2026-10.md](plans/attempt-postmortem-2026-10.md) | Plan: four candidate designs with scores, decisions D1 to D18, components P1 to P5, the wiring lane W1 and the repeat guard G1 (R3). |
| [architecture/adr/0130-a-retry-carries-the-evidence-of-the-attempt-it-replaces.md](architecture/adr/0130-a-retry-carries-the-evidence-of-the-attempt-it-replaces.md) | Decision: each abnormal end of a phase dispatch leaves a postmortem record. The next dispatch or a resume gets a section from the bridge with a do-not-repeat rule. |

## 2026-10-11 — a gate that refuses a changed function below 100%

| Document | What it records |
|---|---|
| [plans/cover-changed-gate-2026-10.md](plans/cover-changed-gate-2026-10.md) | Plan: the 14-day coverage audit, the root cause, four candidate designs and decisions D1 to D13 (no allowance list, CI enforcement). It also holds the criteria to move the loop floor from shadow to enforce. |

## 2026-10-12 — the test system: one run, many judges, one home for each helper (ADR-0131)

| Document | What it records |
|---|---|
| [research/test-system-efficiency-2026-10.md](research/test-system-efficiency-2026-10.md) | Research: the growth of the suite, the repeats in the local floor, the poles of the integration run and the 10 GB transcript scan. It also covers duplicate helpers and isolation. Current practice: coverage, fake time and test selection. Findings F1.1 to F5.5, refinements R1 to R13. |
| [architecture/test-system.md](architecture/test-system.md) | Design, the spec of record: the design of one test case, one instrumented run with judges that read it, and the order of seams. It also covers the helper kits, the duplicate ratchet, wall budgets and test lanes. |
| [plans/test-system-efficiency-2026-10.md](plans/test-system-efficiency-2026-10.md) | Plan: four candidate designs with scores, decisions D1 to D16, and components T0 to T9 with their red tests, order and lane models. |
| [research/ci-wall-time-2026-10.md](research/ci-wall-time-2026-10.md) | Research: where the 21–25 min CI run goes, measured from one test up to the workflow. The macOS job is the critical path, and three shapes of long pole set the package floors. Findings F3.1 to F3.19, refinements R1 to R10. F3.10 has 16 darwin-sensitive packages, F3.13 the rusage, and F3.16 to F3.19 the two test families that use half of the test CPU. Section 3.1 (2026-10-17) adds F3.20 to F3.35 from the first instrumented run. They show the macOS kernel tax, a race sleep per binary and four C1 defects (R11 to R18). |
| [plans/ci-wall-time-2026-10.md](plans/ci-wall-time-2026-10.md) | Plan: five candidate designs with weighted scores. The winner is telemetry first, then a scoped CI matrix, then the unit-level poles. A telemetry index catalog for five levels. Decisions D1 to D14, components C0 to C5b. Section 6a ranks components C6 to C14 (decisions D15 to D23) by measured saving, and scores four approaches for C6, the e2e double run. |
| [architecture/ci-telemetry-and-critical-path.md](architecture/ci-telemetry-and-critical-path.md) | Design: how each CI run records time, resources and load, and how the records join. The report names the critical path and its limiting resource. A worked example. Section 13 specifies C12 (the limit of a package from its own time window) and C13 (macOS CPU samples and a spawn counter). |
| [architecture/adr/0135-ci-measures-itself-before-it-changes-its-shape.md](architecture/adr/0135-ci-measures-itself-before-it-changes-its-shape.md) | Decision: the CI measures itself first, then each job proves only what it alone can prove. |

| [research/model-tier-policy-2026-10.md](research/model-tier-policy-2026-10.md) | Research: the tier defaults of the phase profiles and the CLI and tier of each phase in cycles 1879 to 1914. It also covers agy's own `/usage` screen with its three quota pools, and quota-aware routing in current practice. Each practice has an adopt or reject decision. |
| [research/bridge-factory-2026-10.md](research/bridge-factory-2026-10.md) | Research: an audit of every call that queries or acts on an LLM CLI, with file and line (B1 to B12). It also covers the Factory, Service Locator and dependency injection patterns (P6.1 to P6.5). |
| [architecture/agent-bridge-factory.md](architecture/agent-bridge-factory.md) | Design, the spec of record: the factory `bridge.Resolve`, the DriverSet, capabilities and the manifest `controls` schema. It also covers interfaces that the consumer owns, a worked example, guards and limits. |
| [plans/model-tier-policy-2026-10.md](plans/model-tier-policy-2026-10.md) | Plan: a purpose and a chain for each model tier across separate quota pools, and Gemini 4 Argon (Medium) for agy at deep and top. Three mechanisms scored; the winner is a catalog tier pin. |
| [plans/bridge-factory-2026-10.md](plans/bridge-factory-2026-10.md) | Plan: every query and action on an LLM CLI goes through one bridge factory, and the manifest holds the CLI facts. An audit of the bypasses with file and line, three designs scored, components F1 to F6. |
| [architecture/adr/0137-one-bridge-factory-resolves-every-llm-cli-and-the-manifest-holds-its-facts.md](architecture/adr/0137-one-bridge-factory-resolves-every-llm-cli-and-the-manifest-holds-its-facts.md) | Decision: one bridge factory resolves every LLM CLI, and the manifest holds its facts. |
| [architecture/adr/0136-each-model-tier-walks-separate-quota-pools-and-agy-runs-argon-medium-at-deep-and-top.md](architecture/adr/0136-each-model-tier-walks-separate-quota-pools-and-agy-runs-argon-medium-at-deep-and-top.md) | Decision: each tier walks separate quota pools, and agy runs Argon (Medium) at deep and top. |
| [architecture/adr/0131-one-test-run-feeds-every-coverage-gate.md](architecture/adr/0131-one-test-run-feeds-every-coverage-gate.md) | Decision: one instrumented run feeds every coverage gate. A test never reads the host home. Each test helper has one home. |

## 2026-10-12 — one skill writes every document that a person reads (ADR-0132)

| Document | What it records |
|---|---|
| [research/docs-write-skill-2026-10.md](research/docs-write-skill-2026-10.md) | Research: the house rules for documents, the conflict of the front matter rule, and the skill preload. It surveys Diátaxis, Google design docs, Rust RFCs, KEPs, ADR collections and The Good Docs Project. Findings F2.1 to F2.8 and P3.1 to P3.7, refinements R1 to R9. |
| [plans/docs-write-skill-2026-10.md](plans/docs-write-skill-2026-10.md) | Plan: four candidate approaches with scores, the winner (reasoning first) and what it takes from the others. Decisions D1 to D9 and components K0 to K4. |
| [architecture/adr/0132-documents-are-written-through-one-skill.md](architecture/adr/0132-documents-are-written-through-one-skill.md) | Decision: every document goes through the `docs-write` skill. Each decision cites its evidence and its rejected options. The skill is on by default. |

## 2026-10-12 — the loop watches CLI practice and trims its scaffolding by measure (ADR-0133)

| Document | What it records |
|---|---|
| [research/model-capability-practice-2026-10.md](research/model-capability-practice-2026-10.md) | Research: the size and the emphatic words of the phase prompts, the share of console commits, the full-disk stop, and the watch that exists. It also covers the current guidance of Anthropic, OpenAI and Google, and the sources that a watch can read. Findings F2.1 to F3.3 and P4.1 to P4.6, refinements R1 to R7. |
| [plans/model-capability-practice-2026-10.md](plans/model-capability-practice-2026-10.md) | Plan: four approaches for less scaffolding and four for the practice watch, each with scores, a winner and what it takes from the others. Decisions D1 to D11 and components M0 to M5. |
| [architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md](architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md) | Decision: a deterministic watch files a research item when a source changes. Prompt trims are measured, one persona at a time. |
| [research/practice-claude-code-changelog-2026-10.md](research/practice-claude-code-changelog-2026-10.md) | Research: the Claude Code changelog updates for versions 2.1.299 through 2.1.303 read as data. No persona changes before the M2 baseline. |
| [research/practice-claude-prompting-guide-2026-10.md](research/practice-claude-prompting-guide-2026-10.md) | Research: the Claude prompting guide updates (XML structure, effort levels, and medication dosing blocks) read as data. No persona changes before the M2 baseline. |
| [research/practice-gemini-prompting-guide-2026-10.md](research/practice-gemini-prompting-guide-2026-10.md) | Research: the Gemini prompting guide change (Gemini 4 Argon, the end of custom `temperature`, `top_p` and `top_k`) read as data. The loop sends none of these parameters, and no persona changes before the M2 baseline. |
| [research/practice-agy-cli-changelog-2026-10.md](research/practice-agy-cli-changelog-2026-10.md) | Research: the agy changelog updates for versions 1.3.4 through 1.4.2 read as data. No persona changes before the M2 baseline. |
| [research/practice-agy-cli-changelog-page-2026-10.md](research/practice-agy-cli-changelog-page-2026-10.md) | Research: the agy changelog page updates for v1.4.0 through v2.24.0 and the snapshot diff of search filter labels read as data. No persona changes before the M2 baseline. |
| [research/practice-claude-model-release-notes-2026-10.md](research/practice-claude-model-release-notes-2026-10.md) | Research: the Claude model release notes updates for Opus 5.5 fast mode rate limits and research preview status read as data. No persona changes before the M2 baseline. |

## 2026-10-13 — fleet landing at scale: 200 concurrent lanes (ADR-0134)

| Document | What it records |
|---|---|
| [research/fleet-landing-at-scale-2026-10.md](research/fleet-landing-at-scale-2026-10.md) | Research: why one ship takes five minutes (`cmd/evolve` in the importer backstop). The cost of today's path at N = 2, 20 and 200, and the overlap rate of the import graph. It also surveys merge queues, speculative chains, batches and bisection, content-keyed test selection and git mechanics. Findings F2.1 to F3.5 and P4.1 to P8.4, refinements R1 to R12. |
| [plans/fleet-landing-at-scale-2026-10.md](plans/fleet-landing-at-scale-2026-10.md) | Plan: five approaches with weighted scores, and the winner with what it takes from the others. The winner is partitioned speculative chains with content-keyed verification and group commit. Decisions D1 to D17 and components S0 to S17 with red tests. The rollout from shadow to enforce, and the simulation at N = 2, 20 and 200. |
| [architecture/adr/0134-fleet-landings-scale-through-content-keyed-verification-and-group-commit.md](architecture/adr/0134-fleet-landings-scale-through-content-keyed-verification-and-group-commit.md) | Decision (Proposed): a content key proves each package run, and candidates verify before the head. Candidates with no edge land in any order, and one push lands the verified run. It amends ADR-0128. |

## 2026-10-17 — a code index CLI for every phase agent (ADR-0138)

| Document | What it records |
|---|---|
| [research/code-index-cli-2026-10.md](research/code-index-cli-2026-10.md) | Research: why the trail-code MCP trial got 0 calls in 20 agy phases, and why Claude Code phases cannot reach it. It also records the index timings and surveys CLI against MCP, code-graph tools and `gopls`. Findings C1 to C8 and P3.1 to P3.5, refinements R1 to R6. |
| [plans/code-index-cli-2026-10.md](plans/code-index-cli-2026-10.md) | Plan: four designs with weighted scores. The winner is `evolve code` over a swappable backend, with trail-code first. Decisions D1 to D8 and components K1 to K5 with red tests. |
| [architecture/adr/0138-phase-agents-query-the-code-index-through-an-evolve-cli-not-mcp.md](architecture/adr/0138-phase-agents-query-the-code-index-through-an-evolve-cli-not-mcp.md) | Decision (Proposed): phase agents query the code index through four read-only `evolve code` commands, not MCP. A `Backend` interface keeps a type-checked backend open. |
| [architecture/code-index-cli.md](architecture/code-index-cli.md) | Spec: the commands and their exit codes, the `Backend` interface, the trail-code adapter, the index lifecycle, the sandbox, the usage log and the prompt text. It has a worked example and the guards. |
| [conventions/cli-only-no-mcp.md](conventions/cli-only-no-mcp.md) | Convention: no phase, lane or project config uses MCP. Each outside service is reached through a CLI. It names the three guards: the launch flag test, the structure test and the preflight warning. |

## 2026-10-17 — wave orchestrator: at least four lanes, few conflicts, a large fleet (ADR-0139)

| Document | What it records |
|---|---|
| [research/wave-orchestrator-2026-10.md](research/wave-orchestrator-2026-10.md) | The quota shrink costs a lane for a family with a fallback. 0 of 230 inbox items reach a lane. A barrier of 4 lanes keeps 66% of the slot time busy. The console-routing census shows the lane supply fell from 41 items to 1 in one month, and gives six remedies. It also has an outside survey. Findings F2.1 to F2.24 and P3.1 to P3.17, refinements R1 to R8. |
| [plans/wave-orchestrator-2026-10.md](plans/wave-orchestrator-2026-10.md) | Plan: four approaches with weighted scores. The winner is one deterministic orchestrator that supplies, admits by cell, backfills a rolling pool, leases each lane and oversees on events. Decisions D1 to D15 and components O0 to O8 with red tests. |
| [plans/pool-resupply-2026-10.md](plans/pool-resupply-2026-10.md) | Plan: a free pool slot takes the work that became ready after the run started. Four designs scored; the winner is a `Resupply` port that re-runs the plan source on a lane exit. A deadline stop no longer re-queues the todo in the same run. Decisions D1 to D8. |
| [architecture/adr/0139-a-wave-orchestrator-supplies-admits-and-backfills-every-lane.md](architecture/adr/0139-a-wave-orchestrator-supplies-admits-and-backfills-every-lane.md) | Decision (Proposed): one Go component owns the run of lanes. It builds ADR-0134 S14 as its admission step and wakes on the ADR-0127 channels. |
| [architecture/wave-orchestrator.md](architecture/wave-orchestrator.md) | Design, the spec of record: the census, the sizing, the admission by cell, the pool, the lease, the oversight and the window. It gives the measures and the cost of each step as a function of N. The limits: about 6 lanes for the serial landing, 8–12 lanes for one host (estimate), and 139 disjoint Go cells. |

## 2026-10-17 — code-index backends: trail-code, graphify and the type checker, measured (ADR-0140)

| Document | What it records |
|---|---|
| [research/code-index-backends-2026-10.md](research/code-index-backends-2026-10.md) | Research: trail-code and graphify 0.9.92 measured against a stdlib Go type checker on 40 symbols and 10 files. For each backend, it gives the precision, the recall, the latency, the output size, the build, the disk and the refresh. It also records graphify's writes and advice text, a grep check of the truth and a CLI prototype of the type-checked index. It also surveys graphify, code-graph tools and type-checked Go options. Findings F2.1 to F2.22 and P3.1 to P3.8, refinements R1 to R7. |
| [plans/code-index-backends-2026-10.md](plans/code-index-backends-2026-10.md) | Plan: four approaches with weighted scores, under the operator's rule that forbids MCP. The winner is a capability route that a benchmark fills: a type-checked `gotypes` backend for `callers` and `api`, trail-code for `find` and `map`. Decisions D1 to D11, which match the ADR numbers, and components G1 to G5 with red tests. |
| [architecture/adr/0140-the-type-checker-answers-callers-and-each-query-goes-to-its-best-measured-backend.md](architecture/adr/0140-the-type-checker-answers-callers-and-each-query-goes-to-its-best-measured-backend.md) | Decision (Proposed): the type checker answers `callers` and `api`, and `evolve code bench` decides the backend of each query. graphify stays a bench-only run of its CLI. It relates to ADR-0138. |
| [architecture/code-index-cli.md](architecture/code-index-cli.md) | Spec, section 9 "Backends and the benchmark": how each backend maps the four queries, the bench and its truth, the capability route and the guards. |

## 2026-10-18 — the phase handoff contract: request, build and check between every phase pair (ADR-0141)

| Document | What it records |
|---|---|
| [research/phase-handoff-contract-2026-10.md](research/phase-handoff-contract-2026-10.md) | Research: cycles 1946 and 1950 failed on their request, not on their code. Three of five phase pairs have no check of the output against the ask, and the audit chain is shadow. It also surveys self-checks, independent judges and stop rules. It also covers blocked requests and attestation chains. Findings F1.1 to F1.14 and P2.1 to P2.17, refinements R1 to R10. |
| [plans/phase-handoff-contract-2026-10.md](plans/phase-handoff-contract-2026-10.md) | Plan: four approaches with weighted scores. The winner is one handoff contract: a request, a ledger, a deterministic check and then a judge, with three exits. Decisions D1 to D12 and components H1 to H12 with red tests. |
| [architecture/adr/0141-every-phase-handoff-is-a-request-build-check-loop-with-three-exits.md](architecture/adr/0141-every-phase-handoff-is-a-request-build-check-loop-with-three-exits.md) | Decision (Accepted): every phase pair runs a request, build and check loop. The exits are pass, cap and blocked. Only deterministic proof blocks. |
| [architecture/phase-handoff-contract.md](architecture/phase-handoff-contract.md) | Design, the spec of record: the pairs and their requests, the ledger schema and the two layers of the check. It also gives the exits, the chain in the audit and a worked example on cycle 1950. |

## 2026-10-19 — a proven blocker stops the correction rounds

| Document | What it records |
|---|---|
| [plans/build-blocker-exit-2026-10.md](plans/build-blocker-exit-2026-10.md) | Plan: cycles 1946 and 1950 used all their correction rounds on blockers that no lane edit can fix. Three designs with weighted scores. The winner proves each `BLOCKER` claim in the review loop. Decisions D1 to D10 and components K1 to K7 with red tests. |
| [architecture/blocker-exit.md](architecture/blocker-exit.md) | Spec: the claim grammar, the `BlockerProver` seam, the loop exit, the `blocked-needs-console` ending, the console route and the floor text. |

## 2026-10-19 — a capability failure climbs one bounded retry ladder (ADR-0142)

| Document | What it records |
|---|---|
| [research/retry-ladder-2026-10.md](research/retry-ladder-2026-10.md) | Research: about half of the build correction rounds of cycles 1800 to 1970 fail again, and no round raises the tier or the effort. The builder pin turns off the raises that exist. It also surveys model cascades, effort guidance and self-repair limits. Findings F2.1 to F2.12 and F3.1 to F3.10. |
| [plans/retry-ladder-2026-10.md](plans/retry-ladder-2026-10.md) | Plan: five designs with scores. The winner is one retry ladder with a deterministic classifier, three rungs and a stop rule. Decisions D1 to D14, components L0 to L8 and four open decisions. |
| [architecture/adr/0142-a-capability-failure-climbs-one-bounded-retry-ladder.md](architecture/adr/0142-a-capability-failure-climbs-one-bounded-retry-ladder.md) | Decision (Proposed): only a capability failure climbs. It takes one effort step and then one tier step. Two tiers that fail on one check stop as a spec failure. |
| [architecture/retry-ladder.md](architecture/retry-ladder.md) | Design, the spec of record: the flow, the classifier signals and the rungs for each family. It also gives the retry paths, the try record, the policy keys and the signals. |

## 2026-10-19 — a wider wave, a faster build and gates that test once (ADR-0143)

| Document | What it records |
|---|---|
| [research/wave-latency-2026-10.md](research/wave-latency-2026-10.md) | Research: waves 130 to 133 used 51% of their slot-hours. A lane that edits `internal/core` held 13 ready items for 71 minutes. Build is 46% of lane time, and the gates are 28%. It also surveys merge queues, test caches, routers and pools. Findings F2.1 to F5.3 and O1 to O11. |
| [plans/wave-latency-2026-10.md](plans/wave-latency-2026-10.md) | Plan: three levers with scored designs. The choices are optimistic admission, a headroom-aware build order and one verdict store. Decisions D1 to D7 and components W0 to W5. |
| [architecture/adr/0143-admission-holds-only-shared-edits-and-the-ship-gate-owns-api-drift.md](architecture/adr/0143-admission-holds-only-shared-edits-and-the-ship-gate-owns-api-drift.md) | Decision (Accepted, stage shadow): admission holds only shared edits and the hub, and the composed-tree gate of the ship owns API drift. |

## 2026-10-20 — each rule has one home and loads by audience, path, action or failure (ADR-0145)

| Document | What it records |
|---|---|
| [research/guidance-layers-2026-10.md](research/guidance-layers-2026-10.md) | Research: the rules sit in seven kinds of source. The console carries about 316 always-on lines, and phase agents cannot read the memory rules. It also surveys skills, rule tiers, instruction load and rules as code. Findings F2.1 to F2.12 and S1 to S11. |
| [plans/guidance-layers-2026-10.md](plans/guidance-layers-2026-10.md) | Plan: five designs with scores. The winner is one rule registry that generates every layer, with a check in CI. Decisions D1 to D11, components G0 to G7 and G3a, the migration and eight conflicts for the operator. |
| [architecture/adr/0145-each-rule-has-one-home-and-loads-by-audience-path-action-or-failure.md](architecture/adr/0145-each-rule-has-one-home-and-loads-by-audience-path-action-or-failure.md) | Decision (Proposed): one registry, five layers, four triggers, and a forced load for every rule that must hold. |
| [architecture/guidance-layers.md](architecture/guidance-layers.md) | Design, the spec of record: the layers, the budgets, the triggers and their loaders, and the registry schema. It also gives the load of each audience, the situation skills, the checks and a worked example. |

## 2026-10-20 — latency is gated by invariants at merge and by trends on main (ADR-0144)

| Document | What it records |
|---|---|
| [research/latency-gates-2026-10.md](research/latency-gates-2026-10.md) | Research: the latency fixes, the existing test tools and current practice for gates on noisy runners (F3.1 to F3.5, G1 to G12). |
| [plans/latency-gates-2026-10.md](plans/latency-gates-2026-10.md) | Plan: hard gates on behavior, a seeded wave simulation and soft trend gates (D1 to D5, T1 to T7). |
| [architecture/adr/0144-latency-is-gated-by-invariants-at-merge-and-by-trends-on-main.md](architecture/adr/0144-latency-is-gated-by-invariants-at-merge-and-by-trends-on-main.md) | Decision (Proposed): hard gates never read the wall clock, and measured times file an item after two breaches. |

## 2026-10-20 — a defect that survives two builder rounds is fixed from the finder's spec (ADR-0146)

| Document | What it records |
|---|---|
| [research/finder-fix-ownership-2026-10.md](research/finder-fix-ownership-2026-10.md) | Research: the first repair round fixes about 71% of audit failures. It also surveys who finds, writes and approves a fix. Findings F2.1 to F2.4 and R1 to R12. |
| [plans/finder-fix-ownership-2026-10.md](plans/finder-fix-ownership-2026-10.md) | Plan: five designs with weighted scores. The finder writes the fix spec, a fresh builder applies it, and another check verifies. Decisions D1 to D8 and components F0 to F6. |
| [architecture/adr/0146-a-defect-that-survives-two-builder-rounds-is-fixed-from-the-finders-spec-and-verified-by-another.md](architecture/adr/0146-a-defect-that-survives-two-builder-rounds-is-fixed-from-the-finders-spec-and-verified-by-another.md) | Decision (Accepted): R1 retry, R2 effort, R3 a fix from the finder's spec, verified by a check that did not write it. |

## 2026-10-20 — a change of documents gets a docs review, not a code review

| Document | What it records |
|---|---|
| [research/docs-review-skill-2026-10.md](research/docs-review-skill-2026-10.md) | Research: the commit gate asked a docs-only change for code reviewers. It surveys docs review practice (Diátaxis quality, ADR review, docs-as-code checks). Three approaches are scored. A `docs-review` skill and gate capability win. Findings F2.1 to F2.4, M1 to M7, decisions D1 to D5. |

## 2026-10-20 — the wave orchestrator owns integration and decides when the wave reaches main (ADR-0147)

| Document | What it records |
|---|---|
| [research/wave-integration-2026-10.md](research/wave-integration-2026-10.md) | Research: about one lane in four needed a rebase recovery at landing, and `main` moved 100 times in two days. It also surveys merge trains, merge queues, batching and culprit finding. Findings F2.1 to F2.7 and M1 to M16. |
| [plans/wave-integration-2026-10.md](plans/wave-integration-2026-10.md) | Plan: five designs with scores. The orchestrator orders, composes and tests the deliverables on one integration branch, and flushes a batch as one PR. Decisions D1 to D23, with four test layers L1 to L4, and components I0 to I11. |
| [architecture/adr/0147-the-wave-orchestrator-owns-integration-and-decides-when-the-wave-reaches-main.md](architecture/adr/0147-the-wave-orchestrator-owns-integration-and-decides-when-the-wave-reaches-main.md) | Decision (Proposed): lanes deliver and do not push `main`; the orchestrator owns the order, the compose, the test and the push. |

## 2026-10-20 — docs, inbox and CI PRs merge in a window; code merges at a boundary (ADR-0148)

| Document | What it records |
|---|---|
| [plans/pr-merge-latency-2026-10.md](plans/pr-merge-latency-2026-10.md) | Plan: the merge wait of PRs #979 to #1005, four designs with weighted scores, and components P1 to P5. P1 is shelved. The window stays off until `fleet.integration.stage` is `enforce` (ADR-0147). |
| [architecture/adr/0148-docs-inbox-and-ci-changes-merge-in-a-window-and-code-merges-at-a-boundary-that-the-console-can-call-early.md](architecture/adr/0148-docs-inbox-and-ci-changes-merge-in-a-window-and-code-merges-at-a-boundary-that-the-console-can-call-early.md) | Decision (Accepted): a window PR merges when green, also in a wave. A code PR merges at a boundary, and the console can call one early. |
| [architecture/packages/internal-mergewindow.md](architecture/packages/internal-mergewindow.md) | Package note: the merge window rule reads the CI routing table of `required.yml`. |

## 2026-10-20 — a test-writing phase that states FAIL on its own work is retracted (ADR-0149)

| Document | What it records |
|---|---|
| [architecture/adr/0149-a-test-writing-phase-that-states-fail-on-its-own-work-is-retracted-to-its-dispatch-tree.md](architecture/adr/0149-a-test-writing-phase-that-states-fail-on-its-own-work-is-retracted-to-its-dispatch-tree.md) | Decision (Proposed): on a stated FAIL, the runner restores every path the phase wrote and keeps its copies. Three options are scored, with the argument that a builder red cannot leave through this path. |

## 2026-10-21 — the deep tier shares its seats between Argon and Claude by budget (ADR-0150)

| Document | What it records |
|---|---|
| [research/deep-tier-share-2026-10.md](research/deep-tier-share-2026-10.md) | Research: the Argon burn of 2026-10-17, the usage windows the router can read, and current practice on budget pacing, hash splits and weighted routing. |
| [plans/deep-tier-share-2026-10.md](plans/deep-tier-share-2026-10.md) | Plan: four designs with scores, the share plus the pace, and components S1 to S6. |
| [architecture/adr/0150-the-deep-tier-gives-its-first-family-a-fixed-share-of-the-seats-and-paces-it-by-its-week-window.md](architecture/adr/0150-the-deep-tier-gives-its-first-family-a-fixed-share-of-the-seats-and-paces-it-by-its-week-window.md) | Decision: a fixed share by a hash of the seat, and a pace by the week window. |
| [architecture/deep-tier-share.md](architecture/deep-tier-share.md) | Spec of record: the share stage, the pace and the operator surfaces. |

## 2026-10-21 — the host writes a retrospective document for each recovery and each judgment lesson (ADR-0151)

| Document | What it records |
|---|---|
| [research/recovery-retrospective-2026-10.md](research/recovery-retrospective-2026-10.md) | Research: the detector rule finds 12 recoveries in the dossiers, and no record of a recovery exists. The dossier, not the ledger, is the exact record of a try, and a ship is a PASS with a commit. It also surveys Safety-II, blameless postmortems, flaky-test recovery and agent memory from success and failure pairs. Findings F2.1 to F2.13, P3.1 to P3.7. |
| [plans/recovery-retrospective-2026-10.md](plans/recovery-retrospective-2026-10.md) | Plan: four designs with weighted scores. The host writes the documents from the committed dossiers, at stage `shadow` first. Decisions D1 to D15, components C1 to C6. |
| [architecture/adr/0151-the-host-writes-a-retrospective-document-for-each-recovery-and-each-judgment-lesson.md](architecture/adr/0151-the-host-writes-a-retrospective-document-for-each-recovery-and-each-judgment-lesson.md) | Decision (Proposed): a recovery and a judgment lesson each get a document and a lesson YAML. A judgment todo leaves the carryover log only when its document is on `main`. |
| [architecture/recovery-retrospective.md](architecture/recovery-retrospective.md) | Spec of record: the recovery document, the judgment-lesson document, the lesson YAML, the closeout write and the `enforce` stage. |
| [architecture/packages/internal-recoveryretro.md](architecture/packages/internal-recoveryretro.md) | Package note: the renderers and the one writer, `WriteShadow`. |

## 2026-10-21 — one phase agent runs from the command line with no cycle (ADR-0152)

| Document | What it records |
|---|---|
| [research/agent-run-standalone-2026-10.md](research/agent-run-standalone-2026-10.md) | Research: the phase command, the spec runner and the ledger form for manual entries. Current practice: Claude Code `-p`, `codex exec`, Gemini CLI, the OpenAI Agents SDK and LangGraph. |
| [plans/agent-run-standalone-2026-10.md](plans/agent-run-standalone-2026-10.md) | Plan: four designs with scores, the decisions D1 to D11 and the components A1 to A5. |
| [architecture/adr/0152-one-phase-agent-runs-from-the-command-line-with-no-cycle-through-the-phase-runner.md](architecture/adr/0152-one-phase-agent-runs-from-the-command-line-with-no-cycle-through-the-phase-runner.md) | Decision: `evolve agent run` through the phase runner, with cycle 0, a temporary workspace and read-only profiles only. |
| [architecture/agent-run.md](architecture/agent-run.md) | Spec of record: the input and output contracts, the no-cycle rules and the profiles. |

<!-- GENERATED:research-index END -->
