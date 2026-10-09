# Research Index

> Reference documents available to evolve-loop. The split:
>
> - **Active references** (5 files; 4 in `docs/private/research/archived-2026-05-19/`,
>   `eval-grader-best-practices.md` at `docs/`) — cited by runtime
>   personas/skills/scripts. (`docs/research/` itself now holds the merged
>   research tree — see the 2026-08-05 section below.)
> - **Archived references** (42 files, in `docs/private/research/`) — for
>   contributor reference; explicitly excluded from agent context via the
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
correctly deleted these from runtime context per Liu et al. 2023 "Lost in
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
verdict-classifier drift, flag-reduction design, token histories, et al.) — see
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
| [research/cli-coverage-inventory-2026-09-30.md](research/cli-coverage-inventory-2026-09-30.md) | Which of 147 core functions the published `evolve` CLI runs today (64 fully, 36 partly, 47 not at all), with file:line evidence per row, and the 50 CLI requests it produced (PR #750). |
| [reports/inbox-review-2026-09-30.md](reports/inbox-review-2026-09-30.md) | Why the loop's queue planned 0 batches from 183 items, the per-item verdicts of a six-agent re-verification against main, and the drain that reopened it to 29 batches (PRs #745, #747). |
| [reports/comment-round-12-findings-2026-09-30.md](reports/comment-round-12-findings-2026-09-30.md) | The 43 findings of comment reduction round 12 (PR #749), including two `scopedelta` security holes, and the inbox item that carries each. |
| [reports/comment-strip-clean-code-review-2026-09-30.md](reports/comment-strip-clean-code-review-2026-09-30.md) | A Clean Code review of 64 stripped production files: none read fully without their comments, two thirds of what was removed carried intent, invariants or design, and the defects the comments were covering for. |
| [incidents/2026-09-30-a-lane-ship-grew-a-function-past-the-size-ratchet.md](incidents/2026-09-30-a-lane-ship-grew-a-function-past-the-size-ratchet.md) | Cycle 1779's lane ship turned main red through a whole-tree test its change scope never selected; the class fix (PR #751). |
| [architecture/adr/0110-whole-tree-tests-run-before-main.md](architecture/adr/0110-whole-tree-tests-run-before-main.md) | Decision: every test that reads the whole tree runs before main, and an AST detector keeps the scanner pack complete. |
| [architecture/adr/0111-code-carries-no-comments.md](architecture/adr/0111-code-carries-no-comments.md) | Decision: code carries no comments; what they said moves to code, tests, design notes and the history archive; the commit gate refuses an added comment (PRs #746, #749, #753). |
| [architecture/adr/0114-the-acs-verdict-is-always-written-and-complete.md](architecture/adr/0114-the-acs-verdict-is-always-written-and-complete.md) | Decision: every ACS run writes a complete verdict; a scope that could not run is a named `egps/` harness red carrying its cause, the lane budget is one shared deadline, and predicates run in the suite's own scrubbed environment. |
| [architecture/adr/0115-commit-gate-keeps-history-and-records-waivers.md](architecture/adr/0115-commit-gate-keeps-history-and-records-waivers.md) | Decision: the commit gate refuses a removed history comment its change does not record, records a review waiver as a `Review-waived` trailer, and keeps review for pure-docs changes; `docs/` is not code for the added-comment rule. |
| [architecture/adr/0116-inbox-curation-verbs.md](architecture/adr/0116-inbox-curation-verbs.md) | Decision: inbox items are curated through `evolve inbox show/list/edit/withdraw/verify`; each item field has one owner (authored, curated, or verb-owned), an edit is judged on the fields it writes, a withdrawal undoes a filing, a premise re-verification is a lifecycle stamp. |
| [architecture/adr/0117-pre-handoff-probes-run-the-build-floor.md](architecture/adr/0117-pre-handoff-probes-run-the-build-floor.md) | Decision: `evolve selfcheck build` and `evolve phase verify build` run the build handoff floor itself, one named check list with the cycle's own inputs, after three cycles (1763, 1764, 1788) failed on a floor check a probe had reported green. |

## 2026-10-07 — convergence policy (ADR-0126)

| Document | What it records |
|---|---|
| [research/review-convergence-limited-rounds-2026-10.md](research/review-convergence-limited-rounds-2026-10.md) | Research: the best policy to converge review feedback within limited rounds. Two rounds capture 76–95% of the gain; stop on marginal gain and keep the best (VRR-Stop); LLM reviewers vary across runs, so verify rather than re-hunt (OpenCodeReview); more revisions do not converge without reduced uncertainty (33,596 agent PRs); Google's "better, not perfect"; effort before model, for reasoning failures only. Mapped to refinements CR1–CR8. |
| [incidents/2026-10-07-l2-seven-review-rounds.md](incidents/2026-10-07-l2-seven-review-rounds.md) | The example issue: the L2 routing lane needed seven review fix rounds on the deepest Claude model. Four causes: a review added a capability, a heuristic over an unbounded input, delta checks that hunted instead of verifying, and nothing changing between rounds. |
| [architecture/convergence-policy.md](architecture/convergence-policy.md) | Design: one pure `convergence.Decide` for every repeat-until-accepted loop. A three-rung ladder: change the feedback, then the strategy, then the scope. Stop on marginal gain, land the best round, and never run a fourth. Output first, with deferrals filed as follow-ups. Headroom is read from the manifests. |
| [plans/convergence-policy-2026-10.md](plans/convergence-policy-2026-10.md) | Plan: strategies A–C, decisions V-D1 to V-D11 and CQ1 (the operator: top = Opus 5.5 at xhigh effort), components V0–V9 plus V3b (console first), and the L2 replay as the acceptance test (it lands at round 3 through rung 2). |
| [architecture/adr/0126-every-iterative-loop-converges-or-escalates.md](architecture/adr/0126-every-iterative-loop-converges-or-escalates.md) | Decision: every iterative loop converges or escalates (feedback, then strategy, then scope), lands the best round, and never runs a fourth; capability findings below CRITICAL never block; judges verify after `J_0`; CRITICAL is never deferred. |

## 2026-10-07 — the code-review loop and the audit evaluation contract

| Document | What it records |
|---|---|
| [research/audit-evaluation-contract-prior-art-2026-10.md](research/audit-evaluation-contract-prior-art-2026-10.md) | How current AI software factories (Anthropic, OpenAI, StrongDM, Stripe, Augment) and 2026 evaluator research (SpecBench, ImpossibleRubrics, PROCTOR, Agentic Rubrics, self-preference bias) handle publishing an evaluator's standard to the agents it judges. The verdict is "contract before code, criteria public, instruments private, evidence as certificates, facts over the judge", mapped to refinements AR1–AR9. |
| [architecture/review-loop-and-quality-index.md](architecture/review-loop-and-quality-index.md) | Design: the independent `code-review` loop between build and audit, and the ten-dimension quality index the review and the audit share (ADR-0124). |
| [plans/code-review-phase-2026-10.md](plans/code-review-phase-2026-10.md) | Plan: the code-review phase's strategies (S1–S3), operator decisions, components and landings. |
| [architecture/adr/0124-code-review-phase.md](architecture/adr/0124-code-review-phase.md) | Decision: an independent code-review loop sits between build and audit, and the audit qualifies its result on one shared quality index. |
| [architecture/audit-evaluation-contract.md](architecture/audit-evaluation-contract.md) | Design: the audit's published standard (dimensions plus `G-…` gate criteria with verifiers and certificates), the `audit-plan` phase that writes each cycle's expectations before build, delivery as capped digests, and the final audit held to its plan with a kernel cross-check (ADR-0125). |
| [plans/audit-evaluation-contract-2026-10.md](plans/audit-evaluation-contract-2026-10.md) | Plan: approaches A–C, decisions E-D0 to E-D9 (E-D1–E-D4 by the operator), refinements AR1–AR9, components E0–E7 (E0 its own docs landing, E1–E6 inside the code-review plan's landing 2, E7 at its flip). |
| [architecture/adr/0125-audit-publishes-its-evaluation-contract.md](architecture/adr/0125-audit-publishes-its-evaluation-contract.md) | Decision: the audit publishes its standard and, before build, this cycle's expectations; its verdicts are keyed to published criteria; criteria are public and probe instances private. |

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
