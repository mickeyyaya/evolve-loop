# Enterprise Agent Evaluation

Multi-dimensional evaluation frameworks for production agent deployment. Accuracy alone is insufficient — enterprise agents must also optimize for cost, latency, reliability, and assurance.

---

## CLEAR Framework (arXiv:2511.14136)

CLEAR replaces accuracy-only benchmarking with 5 dimensions. Key finding: accuracy-only agents are **4.4-10.8x more expensive** than cost-aware alternatives with comparable performance. CLEAR correlates with production success at 0.83 vs 0.41 for accuracy-only.

### Five Dimensions

| Dimension | What It Measures | Evolve-Loop Equivalent |
|-----------|-----------------|----------------------|
| **Cost** | Token spend per task | `tokenBudget.perTask`, ledger token tracking |
| **Latency** | Response timing | Cycle wall-clock time, agent invocation duration |
| **Efficacy** | Task completion | Ship rate, eval pass rate |
| **Assurance** | Security/policy compliance | Auditor verdicts, eval tamper detection |
| **Reliability** | Consistency across runs | AgentAssay behavioral fingerprinting, multi-run eval |

### Application to Evolve-Loop Self-Evaluation

The Phase 5 LLM-as-a-Judge currently scores 4 dimensions (Correctness, Completeness, Novelty, Efficiency). CLEAR suggests expanding to include:
- **Reliability** — does the same task type produce consistent results across cycles? (Track via `taskArms` variance)
- **Assurance** — did the cycle comply with all safety constraints? (Track via health check pass rate)

---

## AgencyBench (arXiv:2601.11044)

Real-world agent evaluation benchmark: 138 tasks, 32 scenarios, average 90 tool calls per task. Key findings:
- Closed-source models: 48.4% success rate
- Open-source models: 32.1% success rate
- Gap narrows on structured tasks, widens on ambiguous ones

### Relevance to Evolve-Loop

AgencyBench validates the evolve-loop's approach of **structured task specifications**: tasks with clear acceptance criteria and eval graders (structured) consistently outperform vague task descriptions. The Scout's task sizing and eval writing are the primary levers for improving the loop's "AgencyBench-equivalent" performance.

---

## Cost-Aware Agent Design

CLEAR's most actionable finding: cost varies 50x across agent configurations for similar accuracy. The evolve-loop mitigates this through:

| Mechanism | Cost Savings |
|-----------|-------------|
| Model tier routing (tier-1/2/3) | 40-60% per cycle |
| Plan template caching | 30-50% on repeated task patterns |
| BATS budget-aware scaling | Dynamic strategy switching |
| Inline S-tasks (inst-007) | 30-50K tokens per small task |

The key insight: cost optimization is not about using cheaper models everywhere — it's about routing the right model tier to the right task complexity.

---

## Research References

- CLEAR Framework (arXiv:2511.14136): 5D enterprise evaluation
- AgencyBench (arXiv:2601.11044): real-world agent benchmark with 138 tasks
- BATS (arXiv:2511.17006): budget-aware scaling (documented in `docs/performance-profiling.md`)
- DAAO (arXiv:2509.11079): difficulty-aware routing (documented in `docs/self-learning.md`)

See [research-paper-index.md](research-paper-index.md) for the full citation index.
