> **Agent Lifecycle Management** — Reference doc on managing agents from creation to retirement. Covers the seven-stage lifecycle, monitoring beyond latency, mapping to the evolve-loop pipeline, and retirement protocols for agents, skills, and instincts.

## Table of Contents

- [Seven-Stage Lifecycle](#seven-stage-lifecycle)
- [Stage Details](#stage-details)
- [Monitoring Beyond Latency](#monitoring-beyond-latency)
- [Mapping to Evolve-Loop](#mapping-to-evolve-loop)
- [Retirement Protocol](#retirement-protocol)
- [Prior Art](#prior-art)
- [Anti-Patterns](#anti-patterns)

---

## Seven-Stage Lifecycle

| # | Stage | Purpose | Key Question | Duration |
|---|---|---|---|---|
| 1 | **Design** | Define agent role, capabilities, constraints | What problem does this agent solve? | 1-2 cycles |
| 2 | **Develop** | Implement prompt, tools, orchestration logic | Does the implementation match the design? | 1-3 cycles |
| 3 | **Test** | Validate correctness, safety, edge cases | Does it work reliably across scenarios? | 1-2 cycles |
| 4 | **Deploy** | Ship to production with rollback capability | Can we safely roll back if needed? | < 1 cycle |
| 5 | **Monitor** | Track quality, cost, drift in production | Is it still performing as designed? | Ongoing |
| 6 | **Improve** | Iterate on prompt, tools, or architecture | What changed since last improvement? | Periodic |
| 7 | **Decommission** | Retire agent, archive artifacts, redirect consumers | Is anything still depending on this agent? | 1 cycle |

---

## Stage Details

### 1. Design

| Aspect | Detail |
|---|---|
| **Inputs** | Problem statement, user needs, existing agent inventory |
| **Outputs** | Agent spec (role, tools, constraints, success criteria) |
| **Key Activities** | Define single responsibility; identify required tools; set guardrails; specify input/output contracts |
| **Success Criteria** | Clear role statement; no overlap with existing agents; measurable success metrics defined |

### 2. Develop

| Aspect | Detail |
|---|---|
| **Inputs** | Agent spec, tool definitions, prompt templates |
| **Outputs** | Working agent implementation, unit tests |
| **Key Activities** | Write system prompt; configure tool access; implement orchestration logic; write unit tests |
| **Success Criteria** | All unit tests pass; prompt follows template conventions; tool permissions scoped to minimum required |

### 3. Test

| Aspect | Detail |
|---|---|
| **Inputs** | Agent implementation, eval dataset, baseline scores |
| **Outputs** | Eval results, regression report, safety assessment |
| **Key Activities** | Run eval graders; test edge cases; verify safety constraints; benchmark against baseline |
| **Success Criteria** | Eval scores meet or exceed baseline; no safety violations; edge cases handled gracefully |

### 4. Deploy

| Aspect | Detail |
|---|---|
| **Inputs** | Tested agent, deployment config, rollback plan |
| **Outputs** | Live agent, deployment record, monitoring dashboards |
| **Key Activities** | Publish agent; configure monitoring; verify health checks; document rollback procedure |
| **Success Criteria** | Agent responds to health checks; monitoring active; rollback tested and documented |

### 5. Monitor

| Aspect | Detail |
|---|---|
| **Inputs** | Production telemetry, eval baselines, cost budgets |
| **Outputs** | Health reports, drift alerts, cost reports |
| **Key Activities** | Track reasoning quality; detect drift; measure cost per decision; alert on anomalies |
| **Success Criteria** | No undetected drift > 1 cycle; cost within budget; quality metrics stable |

### 6. Improve

| Aspect | Detail |
|---|---|
| **Inputs** | Monitoring data, user feedback, new requirements |
| **Outputs** | Updated agent, new eval baselines, changelog entry |
| **Key Activities** | Analyze failure modes; update prompt or tools; re-run evals; deploy updated version |
| **Success Criteria** | Improvement measurable in eval scores; no regressions; changelog updated |

### 7. Decommission

| Aspect | Detail |
|---|---|
| **Inputs** | Retirement decision, dependency audit, replacement plan |
| **Outputs** | Archived artifacts, redirected consumers, incident postmortem |
| **Key Activities** | Audit dependents; migrate consumers; archive prompts and evals; document lessons learned |
| **Success Criteria** | Zero active consumers; artifacts archived; no shadow usage detected after 2 cycles |

---

## Monitoring Beyond Latency

Track these metrics to detect degradation before users notice.

| Metric Category | Metric | What It Measures | Alert Threshold |
|---|---|---|---|
| **Reasoning Quality** | Chain-of-thought coherence score | Logical consistency of reasoning steps | < 0.85 coherence |
| **Reasoning Quality** | Hallucination rate | Frequency of fabricated claims or references | > 2% of outputs |
| **Decision Accuracy** | Eval pass rate | Percentage of correct decisions on eval set | < baseline - 5% |
| **Decision Accuracy** | False positive/negative ratio | Balance of error types | Ratio shift > 20% |
| **Tool Use Efficiency** | Tool calls per task | Number of tool invocations to complete work | > 2x baseline |
| **Tool Use Efficiency** | Unnecessary tool call rate | Tools called but results unused | > 10% of calls |
| **Cost Per Decision** | Tokens per successful output | Total token spend per useful result | > 1.5x budget |
| **Cost Per Decision** | Retry rate | Frequency of re-runs due to failure | > 15% of tasks |
| **Drift Detection** | Output distribution shift | Statistical divergence from baseline outputs | KL divergence > 0.1 |
| **Drift Detection** | Prompt adherence score | How closely outputs follow system prompt instructions | < 0.90 adherence |

---

## Mapping to Evolve-Loop

Map each lifecycle stage to existing evolve-loop infrastructure.

| Lifecycle Stage | Evolve-Loop Component | Artifact / Tool | Responsibility |
|---|---|---|---|
| **Design** | `docs/reference/agent-templates.md` | Agent spec template, role definitions | Define agent contract and constraints |
| **Develop** | Builder agent | `build-report.md`, agent implementation | Implement agent per spec; write tests |
| **Test** | Auditor agent, eval graders | `audit-report.md`, eval results | Validate correctness and safety |
| **Deploy** | `scripts/publish.sh` | Published skill/agent, deployment record | Ship agent to production |
| **Monitor** | Operator, `scripts/cycle-health-check.sh` | Health reports, drift alerts | Track quality, cost, drift continuously |
| **Improve** | Meta-cycle (Scout re-evaluation) | Updated agent, new baselines | Identify improvement opportunities |
| **Decommission** | Instinct archival process | Archived instincts, migration records | Retire agent and redirect dependents |

### Stage Transitions

| Transition | Gate | Enforced By |
|---|---|---|
| Design -> Develop | Agent spec reviewed and approved | Scout report includes spec validation |
| Develop -> Test | All unit tests pass; code review complete | Builder phase gate (`scripts/phase-gate.sh`) |
| Test -> Deploy | Eval scores meet baseline; no safety violations | Auditor phase gate (`scripts/phase-gate.sh`) |
| Deploy -> Monitor | Health checks pass; monitoring configured | Deployment verification in `publish.sh` |
| Monitor -> Improve | Drift or regression detected | Operator alerts trigger Scout re-evaluation |
| Improve -> Test | Updated agent ready for re-evaluation | Cycle loops back through Test stage |
| Monitor -> Decommission | Retirement criteria met | Manual decision with dependency audit |

---

## Retirement Protocol

### When to Retire

| Entity | Retire When | Example |
|---|---|---|
| **Agent** | Role absorbed by another agent; eval scores permanently below threshold; no consumers for 5+ cycles | Scout subsumes a specialized search agent |
| **Skill** | Functionality superseded; usage drops to zero; underlying API deprecated | A skill wrapping a removed tool |
| **Instinct** | Behavior now encoded in agent prompt; contradicts updated guidelines; causes regressions | An instinct that conflicts with a new safety rule |

### Retirement Checklist

| Step | Action | Verification |
|---|---|---|
| 1 | Audit all dependents | Run dependency graph analysis; confirm zero active consumers |
| 2 | Notify consumers | Alert dependent agents/workflows of retirement timeline |
| 3 | Provide migration path | Document replacement agent/skill/instinct and migration steps |
| 4 | Archive artifacts | Move prompts, evals, and configs to `archive/` directory |
| 5 | Remove from active registry | Delete from agent index; remove from routing tables |
| 6 | Monitor for shadow usage | Check logs for 2 cycles post-retirement; alert on any invocations |
| 7 | Write postmortem | Document why the entity was retired and lessons learned |

### Preventing Shadow Systems

| Risk | Mitigation |
|---|---|
| Hardcoded agent references | Use agent registry for all lookups; never hardcode agent names |
| Cached prompts referencing retired agents | Invalidate caches on retirement; include version checks |
| Undocumented integrations | Require all integrations to register in dependency graph |
| Copy-pasted agent logic | Detect duplicated prompt fragments in code review |

---

## Prior Art

| Source | Contribution | Relevance to Agent Lifecycle |
|---|---|---|
| **AgentOps** | Observability platform for LLM agents | Monitoring stage: session tracking, cost analysis, replay debugging |
| **MLOps Lifecycle** (Google, AWS) | Standardized ML model lifecycle (train, validate, deploy, monitor, retrain) | Direct analog: replace "model" with "agent"; same stage gates apply |
| **DevOps / SRE** (Google SRE Book) | Service lifecycle with SLOs, error budgets, incident response | Monitor/Improve stages: apply SLO thinking to agent quality metrics |
| **Anthropic Deployment Guidelines** | Safety-focused deployment for AI systems | Test/Deploy stages: eval-gated deployment, safety benchmarks before ship |
| **LangSmith / LangFuse** | Tracing and evaluation for LLM applications | Monitor stage: trace-level debugging, eval dataset management |
| **Weights & Biases** | Experiment tracking and model registry | Improve stage: track prompt iterations, compare eval runs across versions |
| **DORA Metrics** (Accelerate) | Deployment frequency, lead time, MTTR, change failure rate | Adapt for agent systems: cycle frequency, time-to-ship, agent failure rate |

---

## Anti-Patterns

| Anti-Pattern | Problem | Fix |
|---|---|---|
| **No Retirement Plan** | Agents accumulate indefinitely; routing becomes complex; costs grow | Define retirement criteria at Design stage; review every 10 cycles |
| **Shadow Agents** | Unofficial agents bypass monitoring and safety checks | Require registry for all agents; audit for unregistered tool usage |
| **Monitoring Only Throughput** | Misses reasoning quality degradation, hallucination drift, cost bloat | Track all metrics in [Monitoring Beyond Latency](#monitoring-beyond-latency) |
| **Manual Lifecycle Management** | Error-prone; stages skipped under time pressure; no audit trail | Automate stage gates with `scripts/phase-gate.sh`; enforce via CI |
| **Immortal Instincts** | Outdated instincts conflict with current guidelines; cause regressions | Review instincts every meta-cycle; archive stale ones per retirement protocol |
| **Big-Bang Deployment** | Deploying untested agents directly to production without canary | Use shadow or canary deployment; gate on eval scores before full rollout |
| **Monolithic Agents** | Single agent handles too many responsibilities; hard to test and retire | Follow single-responsibility principle; split into focused agents |
| **Eval-Free Improvement** | Updating agents without re-running evals; regressions go undetected | Require eval pass before every deployment; automate in phase gate |
