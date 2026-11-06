# The goal of the loop, and how each decision serves it

> Decision: [ADR-0164](adr/0164-all-decisions-serve-one-goal-and-one-amendment-resolves-their-conflicts.md). Evidence: [decision-reconciliation-2026-11](../history/decision-reconciliation-2026-11.md).

The operator, on 2026-11-06: "a self healing ai driven development loop which continue to deliver the the best quality features with top design and top quality deliverables. It relentless building features and is capable of fixing any issues it faced with logic and research to build the best practical options".

## 1. The parts of the goal

| Part | Words of the goal | What it means here |
|---|---|---|
| Drive | "ai driven development loop" | Agents do the work, and deterministic Go owns the flow. |
| Heal | "self healing", "fixing any issues" | The loop finds its faults and repairs them inside its limits. |
| Deliver | "relentless building features" | Work keeps flowing to `main`. |
| Quality | "best quality features", "top design" | Gates and independent reviews prove each change. |
| Research | "logic and research", "best practical options" | A decision rests on evidence and compares options. |

The map below also has **Support**: visibility and agent instructions. It serves each part.

## 2. Each ADR by part

A number sits under one part only, the part that it serves most. This grouping is a synthesis from the status and the decision of each record.

| Part | ADRs | Count |
|---|---|---|
| Drive | 0001 0002 0003 0004 0005 0006 0019 0021 0022 0023 0024 0028 0029 0031 0032. 0035 0038 0051 0052 0058 0060 0099 0104 0119 0136 0137 0138 0140 0150 0152. 0154 0156 0163 | 33 |
| Heal | 0026 0039 0044 0045 0063 0072 0076 0079 0085 0086 0089 0090 0105 0106 0107. 0113 0114 0117 0118 0122 0126 0130 0142 0146 0151 0155 0157 0158 0160 0161 | 30 |
| Deliver | 0043 0048 0049 0054 0057 0059 0073 0078 0080 0081 0083 0094 0108 0112 0116. 0121 0127 0128 0129 0131 0134 0135 0139 0143 0144 0147 0148 0159 | 28 |
| Quality | 0007 0008 0012 0025 0027 0033 0034 0042 0046 0047 0050 0053 0055 0061 0062. 0064 0065 0069 0074 0075 0077 0082 0084 0087 0088 0091 0092 0093 0096 0097. 0098 0100 0102 0103 0110 0111 0115 0123 0124 0125 0141 0149 0153 0162 | 44 |
| Research | 0010 0011 0017 0056 0132 0133 | 6 |
| Support | 0009 0013 0014 0015 0016 0018 0020 0030 0036 0037 0040 0041 0066 0067 0068. 0070 0071 0095 0101 0145 | 20 |

ADR-0164 records this map. The numbers 0109 and 0120 do not exist.

## 3. Each plan by part

| Part | Plans (`docs/plans/`, without the date suffix) | Count |
|---|---|---|
| Drive | agent-run-standalone, bridge-factory, cell-context-index, cli-routing-table, code-index-backends, code-index-cli, deep-tier-share, fable-simulation, guidance-layers, llm-isolation, model-currency, model-level-benches, model-tier-policy, opus55-effort-config, seat-login-isolation, token-telemetry (two plans) | 17 |
| Heal | `attempt-postmortem`, build-blocker-exit, convergence-policy, cost-aware-repair, debug-cli, dispatch-process-hygiene, finder-fix-ownership, inbox-stale-claims, logging-and-process-hygiene, narrow-fence, pipeline-blockers, queue-authority-from-host-facts, recovery-retrospective, retry-ladder, workspace-hygiene | 15 |
| Deliver | agent-messaging, ci-wall-time, concurrent-cycle-landing, event-notification-protocol, evolve-wave-cli, fleet-landing-at-scale, inbox-prioritization, latency-gates, ledger-restructure, pool-resupply, pr-merge-latency, skills-drift-host-stamp, skills-drift-worktree-generator, test-system-efficiency, wave-integration, wave-latency, wave-orchestrator | 17 |
| Quality | audit-evaluation-contract, code-review-phase, comment-reduction, cover-changed-gate, harness-practices, llm-quality-indexes, phase-handoff-contract, ste100-docs-rewrite | 8 |
| Research | docs-write-skill, model-capability-practice | 2 |

## 4. What the map shows

- **No record opposes the goal.** The conflicts are between older and newer means, and ADR-0164 resolves them or names them as open.
- **Research is the thinnest part.** Six records serve it, and three of them (ADR-0010, ADR-0011, ADR-0017) limit the turns or the searches of a phase. No record asked for research at a repair. ADR-0164 decision 12 starts to close this at the redesign rung.
- **Heal is designed, and its authority is not yet Accepted.** ADR-0155, ADR-0157 and ADR-0126 read Proposed.
- **The rank can starve features.** Of 58 ready items on 2026-11-06, 29 have kind `feature` and 4 have class `feature`. The operator decides if the rank changes.
- **Shipping is low.** Wave 156 shipped 3 of 9 cycles. One wave shows a level and not a trend.

## 5. What runs today

| Question | Answer | Source |
|---|---|---|
| What lands on `main` today? | A lane lands its own work. The console lands through `evolve integration batch`. | [wave-integration plan](../plans/wave-integration-2026-10.md), ADR-0164 decision 11 |
| What is the target? | The wave orchestrator owns integration. Its driver runs at shadow, and the flip to enforce is the operator's. | ADR-0147, [integration-driver.md](integration-driver.md) |
| Which records run at `enforce` and read Proposed? | ADR-0139, ADR-0151 and ADR-0154 (invariant I-2). | `runtime/.evolve/policy.json`, 2026-11-06 |
