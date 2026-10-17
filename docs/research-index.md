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

**The engineering chronicle** — workstream-level narratives (problem /
approaches / decision / results / retro) — lives at
[docs/chronicle/](chronicle/README.md).

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
| [research/cli-coverage-inventory-2026-09-30.md](research/cli-coverage-inventory-2026-09-30.md) | The CLI runs 64 functions fully, 36 partly, and 47 not at all. It records evidence and 50 CLI requests (PR #750). |
| [reports/inbox-review-2026-09-30.md](reports/inbox-review-2026-09-30.md) | The queue planned 0 batches from 183 items. It shows the verdicts against main and reopening to 29 batches (PRs #745, #747). |
| [reports/comment-round-12-findings-2026-09-30.md](reports/comment-round-12-findings-2026-09-30.md) | The 43 findings of comment reduction round 12 (PR #749), including two `scopedelta` security holes, and the inbox item that carries each. |
| [reports/comment-strip-clean-code-review-2026-09-30.md](reports/comment-strip-clean-code-review-2026-09-30.md) | A Clean Code review of 64 stripped production files. Two thirds of removed comments carried intent, invariants, or design. |
| [incidents/2026-09-30-a-lane-ship-grew-a-function-past-the-size-ratchet.md](incidents/2026-09-30-a-lane-ship-grew-a-function-past-the-size-ratchet.md) | Cycle 1779's lane ship turned main red through a whole-tree test its change scope never selected; the class fix (PR #751). |
| [architecture/adr/0110-whole-tree-tests-run-before-main.md](architecture/adr/0110-whole-tree-tests-run-before-main.md) | Decision: every test that reads the whole tree runs before main, and an AST detector keeps the scanner pack complete. |
| [architecture/adr/0111-code-carries-no-comments.md](architecture/adr/0111-code-carries-no-comments.md) | Decision: code carries no comments. Comments move to code, tests, notes, and the history archive. The commit gate refuses added comments (PRs #746, #749, #753). |
| [architecture/adr/0114-the-acs-verdict-is-always-written-and-complete.md](architecture/adr/0114-the-acs-verdict-is-always-written-and-complete.md) | Decision: every ACS run writes a complete verdict. Unrun scopes become egps/ harness reds. The lane budget is one shared deadline. |
| [architecture/adr/0115-commit-gate-keeps-history-and-records-waivers.md](architecture/adr/0115-commit-gate-keeps-history-and-records-waivers.md) | Decision: the commit gate refuses unrecorded comment removals. It records a review waiver as a Review-waived trailer. The directory docs/ is exempt from the rule. |
| [architecture/adr/0116-inbox-curation-verbs.md](architecture/adr/0116-inbox-curation-verbs.md) | Decision: curate inbox items through evolve inbox verbs. Each field has one owner. An edit evaluates written fields, and withdrawal undoes a filing. |
| [architecture/adr/0117-pre-handoff-probes-run-the-build-floor.md](architecture/adr/0117-pre-handoff-probes-run-the-build-floor.md) | Decision: evolve selfcheck build and evolve phase verify build run the build floor itself. This check list uses the cycle inputs directly. |

## 2026-10-07 — convergence policy (ADR-0126)

| Document | What it records |
|---|---|
| [research/review-convergence-limited-rounds-2026-10.md](research/review-convergence-limited-rounds-2026-10.md) | Research: policies to converge review feedback in limited rounds. Two rounds capture most gains. Stop on marginal gain and keep the best round. |
| [incidents/2026-10-07-l2-seven-review-rounds.md](incidents/2026-10-07-l2-seven-review-rounds.md) | The example issue: the L2 routing lane needed seven review fix rounds on the deepest Claude model. Four causes: a review added a capability, a heuristic over an unbounded input, delta checks that hunted instead of verifying, and nothing changing between rounds. |
| [architecture/convergence-policy.md](architecture/convergence-policy.md) | Design: one pure `convergence.Decide` for every repeat-until-accepted loop. A three-rung ladder: change the feedback, then the strategy, then the scope. Stop on marginal gain, land the best round, and never run a fourth. Output first, with deferrals filed as follow-ups. Headroom is read from the manifests. |
| [plans/convergence-policy-2026-10.md](plans/convergence-policy-2026-10.md) | Plan: strategies A to C, decisions V-D1 to V-D11, and components V0 to V9. The L2 replay serves as the acceptance test. |
| [architecture/adr/0126-every-iterative-loop-converges-or-escalates.md](architecture/adr/0126-every-iterative-loop-converges-or-escalates.md) | Decision: iterative loops converge or escalate, keep the best round, and never run a fourth round. Non-critical findings do not block. |

## 2026-10-07 — the code-review loop and the audit evaluation contract

| Document | What it records |
|---|---|
| [research/audit-evaluation-contract-prior-art-2026-10.md](research/audit-evaluation-contract-prior-art-2026-10.md) | Research: how AI software factories publish evaluation standards. The rule is contract before code with public criteria and private instruments. |
| [architecture/review-loop-and-quality-index.md](architecture/review-loop-and-quality-index.md) | Design: the independent `code-review` loop between build and audit, and the ten-dimension quality index the review and the audit share (ADR-0124). |
| [plans/code-review-phase-2026-10.md](plans/code-review-phase-2026-10.md) | Plan: the code-review phase's strategies (S1–S3), operator decisions, components and landings. |
| [architecture/adr/0124-code-review-phase.md](architecture/adr/0124-code-review-phase.md) | Decision: an independent code-review loop sits between build and audit, and the audit qualifies its result on one shared quality index. |
| [architecture/audit-evaluation-contract.md](architecture/audit-evaluation-contract.md) | Design: the audit published standard and gate criteria. The audit-plan phase writes expectations before build. The kernel cross-checks the plan. |
| [plans/audit-evaluation-contract-2026-10.md](plans/audit-evaluation-contract-2026-10.md) | Plan: approaches A to C, decisions E-D0 to E-D9, and components E0 to E7. Components land in stages across two reviews. |
| [architecture/adr/0125-audit-publishes-its-evaluation-contract.md](architecture/adr/0125-audit-publishes-its-evaluation-contract.md) | Decision: the audit publishes its standard and expectations before build. Verdicts link to public criteria with private probes. |

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
| [architecture/adr/0130-a-retry-carries-the-evidence-of-the-attempt-it-replaces.md](architecture/adr/0130-a-retry-carries-the-evidence-of-the-attempt-it-replaces.md) | Decision: abnormal dispatch ends leave postmortem records. The next dispatch receives a bridge section with do-not-repeat rules. |

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
| [research/ci-wall-time-2026-10.md](research/ci-wall-time-2026-10.md) | Research: where the 21–25 min CI run goes, measured from one test up to the workflow. The macOS job is the critical path, and three shapes of long pole set the package floors. Findings F3.1 to F3.15 (16 darwin-sensitive packages in F3.10; per-package rusage in F3.13), refinements R1 to R9. |
| [plans/ci-wall-time-2026-10.md](plans/ci-wall-time-2026-10.md) | Plan: five candidate designs with weighted scores. The winner is telemetry first, then a scoped CI matrix, then the unit-level poles. A telemetry index catalog for five levels. Decisions D1 to D13, components C0 to C5. |
| [architecture/ci-telemetry-and-critical-path.md](architecture/ci-telemetry-and-critical-path.md) | Design: how each CI run records time, resources and load, and how the records join. The report names the critical path and its limiting resource. A worked example. |
| [architecture/adr/0135-ci-measures-itself-before-it-changes-its-shape.md](architecture/adr/0135-ci-measures-itself-before-it-changes-its-shape.md) | Decision: the CI measures itself first, then each job proves only what it alone can prove. |
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
| [research/practice-claude-code-changelog-2026-10.md](research/practice-claude-code-changelog-2026-10.md) | Research: the Claude Code changelog updates for versions 2.1.299 through 2.1.302 read as data. No persona changes before the M2 baseline. |
| [research/practice-claude-prompting-guide-2026-10.md](research/practice-claude-prompting-guide-2026-10.md) | Research: the Claude prompting guide updates (XML structure, effort levels, and medication dosing blocks) read as data. No persona changes before the M2 baseline. |
| [research/practice-gemini-prompting-guide-2026-10.md](research/practice-gemini-prompting-guide-2026-10.md) | Research: the Gemini prompting guide change (Gemini 4 Argon, the end of custom `temperature`, `top_p` and `top_k`) read as data. The loop sends none of these parameters, and no persona changes before the M2 baseline. |
| [research/practice-agy-cli-changelog-2026-10.md](research/practice-agy-cli-changelog-2026-10.md) | Research: the agy changelog updates for versions 1.3.4 through 1.4.0 read as data. No persona changes before the M2 baseline. |
| [research/practice-agy-cli-changelog-page-2026-10.md](research/practice-agy-cli-changelog-page-2026-10.md) | Research: the agy changelog page updates for versions v1.4.0, v2.23.0, v2.23.1, and v2.24.0 read as data. No persona changes before the M2 baseline. |
| [research/practice-claude-model-release-notes-2026-10.md](research/practice-claude-model-release-notes-2026-10.md) | Research: the Claude model release notes updates for Opus 5.5 fast mode rate limits and research preview status read as data. No persona changes before the M2 baseline. |

## 2026-10-13 — fleet landing at scale: 200 concurrent lanes (ADR-0134)

| Document | What it records |
|---|---|
| [research/fleet-landing-at-scale-2026-10.md](research/fleet-landing-at-scale-2026-10.md) | Research: why one ship takes five minutes (`cmd/evolve` in the importer backstop). The cost of today's path at N = 2, 20 and 200, and the overlap rate of the import graph. It also surveys merge queues, speculative chains, batches and bisection, content-keyed test selection and git mechanics. Findings F2.1 to F3.5 and P4.1 to P8.4, refinements R1 to R12. |
| [plans/fleet-landing-at-scale-2026-10.md](plans/fleet-landing-at-scale-2026-10.md) | Plan: five approaches with weighted scores, and the winner with what it takes from the others. The winner is partitioned speculative chains with content-keyed verification and group commit. Decisions D1 to D17 and components S0 to S17 with red tests. The rollout from shadow to enforce, and the simulation at N = 2, 20 and 200. |
| [architecture/adr/0134-fleet-landings-scale-through-content-keyed-verification-and-group-commit.md](architecture/adr/0134-fleet-landings-scale-through-content-keyed-verification-and-group-commit.md) | Decision (Proposed): a content key proves each package run, and candidates verify before the head. Candidates with no edge land in any order, and one push lands the verified run. It amends ADR-0128. |
