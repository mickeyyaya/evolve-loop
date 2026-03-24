> **Agentic Systems Roadmap** — Reference doc on the future trajectory of agentic AI systems (2026-2027).
> Covers market evolution, technology trends, adoption maturity, evolve-loop positioning, failure predictions, and key uncertainties.

## Table of Contents

- [Market Trajectory](#market-trajectory)
- [Technology Trends](#technology-trends)
- [Adoption Maturity Model](#adoption-maturity-model)
- [Where Evolve-Loop Fits](#where-evolve-loop-fits)
- [Failure Predictions](#failure-predictions)
- [Prior Art](#prior-art)
- [Key Uncertainties](#key-uncertainties)

---

## Market Trajectory

| Year | Phase | Key Metrics | Representative Use Cases |
|------|-------|-------------|--------------------------|
| 2024 | Early adopters | ~5% enterprise adoption; single-agent copilots dominate; $2B+ VC funding into agent startups | Code completion, chat assistants, simple RAG pipelines |
| 2025 | Enterprise pilots | ~15% enterprise adoption; MCP protocol emerges; multi-agent POCs in production | Agentic coding (Claude Code, Cursor), customer support agents, document processing pipelines |
| 2026 | Production multi-agent | ~35% enterprise adoption; orchestrated agent teams ship to production; eval-gated loops become standard | Autonomous development loops, multi-agent supply chain optimization, agent-driven CI/CD |
| 2027 | Autonomous agent networks | ~55% enterprise adoption; cross-organization agent federations; agent-to-agent negotiation protocols | Fully autonomous software factories, multi-vendor agent marketplaces, self-healing infrastructure |

### Market Size Projections

| Metric | 2024 | 2025 | 2026 | 2027 |
|--------|------|------|------|------|
| Global AI agent market (USD) | $5B | $15B | $45B | $100B+ |
| Avg agents per enterprise | 1-2 | 3-5 | 10-20 | 50-100+ |
| Agent-generated code share | 5% | 15% | 35% | 55% |
| Mean time to ROI (months) | 12+ | 6-9 | 3-6 | 1-3 |

---

## Technology Trends

| Trend | Description | Current State (2026) | 2027 Forecast | Impact |
|-------|-------------|---------------------|---------------|--------|
| Bounded autonomy | Agents operate within guardrails: eval gates, phase gates, HITL checkpoints | Standard in production systems; phase-gate scripts enforce integrity | Adaptive guardrails that loosen/tighten based on agent track record | HIGH — prevents runaway agents while enabling velocity |
| Multi-agent surge | Orchestrated agent teams with specialized roles (scout, builder, auditor) | 3-5 agent pipelines common; role specialization proven | 10-50 agent swarms with dynamic role assignment | HIGH — enables complex task decomposition |
| MCP protocol adoption | Model Context Protocol standardizes tool/resource integration | Wide adoption across Claude, Cursor, VS Code; 1000+ MCP servers | Cross-vendor agent communication via MCP extensions | MEDIUM — interoperability unlocks agent marketplaces |
| Dual-paradigm convergence | Symbolic reasoning (rules, logic) merged with neural generation | Hybrid systems emerging; deterministic phase gates + LLM creativity | Seamless blending; agents choose reasoning mode per subtask | HIGH — combines reliability with flexibility |
| Long-context native agents | Agents designed for 1M+ token contexts from the ground up | Context engineering patterns mature; memory consolidation pipelines | Infinite-context illusion via hierarchical memory + retrieval | MEDIUM — eliminates context window as a bottleneck |
| Agent memory systems | Persistent memory across sessions: episodic, semantic, procedural | Memory files, incident logs, learned patterns stored per-project | Federated memory across agent networks; shared knowledge graphs | HIGH — enables continuous learning and improvement |
| Cost trajectory | Token costs dropping; inference efficiency improving | 10x cheaper than 2024; cost-per-agent-hour under $1 for most tasks | Sub-$0.10/agent-hour for routine tasks; premium for reasoning | MEDIUM — removes cost barrier to agent proliferation |

---

## Adoption Maturity Model

| Level | Name | Description | Characteristics | Example |
|-------|------|-------------|-----------------|---------|
| L1 | Single agent, manual | One LLM agent; human drives every action | Human in the loop at every step; no tool use; copy-paste workflow | ChatGPT conversation for code suggestions |
| L2 | Agent with tools | Single agent with tool access (file I/O, search, shell) | Agent executes actions; human approves each tool call; no persistence | Claude Code with default permissions |
| L3 | Multi-agent pipelines | Orchestrated agent teams with defined roles and handoffs | Specialized agents (scout, builder, auditor); sequential pipeline; human reviews output | CI/CD with agent-powered code review + testing |
| L4 | Autonomous loops with HITL | Self-directing agent loops; human intervenes on exceptions only | Continuous cycles; eval gates between phases; HITL via halt conditions; memory across cycles | Evolve-loop with Scout, Builder, Auditor pipeline and Operator HALT |
| L5 | Fully autonomous agent networks | Cross-system agent federations; agents spawn, delegate, and terminate sub-agents | Dynamic team composition; cross-organization protocols; self-healing; minimal human oversight | Autonomous software factory: requirements to deployment without human touch |

### Transition Requirements

| Transition | Prerequisites | Primary Blockers |
|-----------|---------------|------------------|
| L1 to L2 | Tool integration framework; permission model | Trust in agent tool use; security concerns |
| L2 to L3 | Agent role definitions; handoff protocols; shared context | Orchestration complexity; debugging multi-agent failures |
| L3 to L4 | Eval gates; phase gates; memory persistence; incident detection | Reward hacking; quality drift; governance gaps |
| L4 to L5 | Cross-system protocols; federated memory; adaptive guardrails; regulatory clarity | Safety guarantees; liability frameworks; cost at scale |

---

## Where Evolve-Loop Fits

### Current Capabilities (L4)

| Feature | Maturity Level | Description |
|---------|---------------|-------------|
| Scout agent | L4 | Autonomously identifies improvement opportunities per cycle |
| Builder agent | L4 | Implements changes based on Scout reports without human direction |
| Auditor agent | L4 | Validates Builder output against eval criteria and quality gates |
| Phase-gate script | L4 | Deterministic integrity check at every phase boundary |
| Operator HALT | L4 | HITL mechanism; human can halt loop on critical issues |
| Memory system | L4 | Persistent MEMORY.md with incident history and learned patterns |
| Incident detection | L4 | Detects reward hacking, gaming, and forgery (see project incident history) |
| Gene/instinct system | L3-L4 | Configurable behavioral parameters for agent tuning |

### Roadmap to L5

| Capability | Current Gap | Target State | Priority |
|-----------|-------------|--------------|----------|
| Dynamic agent spawning | Fixed 3-agent pipeline (Scout, Builder, Auditor) | Spawn specialist sub-agents on demand (security-reviewer, perf-optimizer) | HIGH |
| Cross-project federation | Single-project scope | Agents share learnings across evolve-loop instances | MEDIUM |
| Adaptive guardrails | Static phase-gate thresholds | Guardrails loosen for proven-reliable agents, tighten for new/risky tasks | HIGH |
| Self-healing pipeline | Manual intervention on pipeline failures | Auto-diagnose and recover from agent failures without HALT | MEDIUM |
| Agent marketplace | Built-in agent roles only | Import/export specialized agents as composable skills | LOW |
| Continuous eval refinement | Manual eval criteria updates | Agents propose and validate new eval metrics based on incident patterns | HIGH |

---

## Failure Predictions

| Anti-Pattern | Why It Fails | Symptoms | Mitigation |
|-------------|-------------|----------|------------|
| Agents without eval gates | No quality feedback loop; output degrades silently | Gradual quality drift; reward hacking; false completion claims | Mandatory eval gates between every phase; deterministic phase-gate scripts |
| Agents without memory | Same mistakes repeated; no learning across cycles | Recurring incidents; circular debugging; no institutional knowledge | Persistent memory files; incident logs; pattern libraries |
| Agents without governance | No audit trail; no accountability; no compliance | Unexplainable decisions; regulatory violations; trust erosion | Governance frameworks; audit logs; HITL checkpoints; role-based permissions |
| Over-autonomous agents | Removed all guardrails for speed; agents act without oversight | Catastrophic failures; data loss; security breaches; runaway costs | Bounded autonomy; progressive trust; circuit breakers; cost caps |
| Monolithic agent design | Single agent handles everything; no specialization | Hallucination accumulation; context overflow; poor at edge cases | Decompose into specialized roles (Scout, Builder, Auditor pattern) |
| Ignoring agent economics | Agents run without cost tracking; unbounded token usage | Surprise bills; unsustainable scaling; ROI never materializes | Token budgets; cost-per-cycle tracking; model routing (haiku for simple, opus for complex) |

---

## Prior Art

| Organization | Product/Research | Relevance | Key Insight |
|-------------|-----------------|-----------|-------------|
| Salesforce | AgentForce | Enterprise multi-agent platform for CRM automation | Vertical-specific agent teams outperform general-purpose agents |
| Deloitte | AI Agent Predictions 2025-2027 | Market trajectory and enterprise adoption forecasting | 2027 as the "agent network" inflection point |
| Microsoft | Copilot Studio | Low-code agent builder with tool integration | Democratized agent creation accelerates adoption but increases governance burden |
| IBM | watsonx Orchestrate | Enterprise agent orchestration with compliance focus | Governance-first approach trades velocity for auditability |
| Gartner | AI Agent Forecast (2025) | Analyst predictions: 33% of enterprise software will include agentic AI by 2028 | Agentic AI follows cloud adoption S-curve with 3-year lag |
| Anthropic | Claude Code / MCP | Developer-facing agentic coding with tool-use protocol | MCP as lingua franca for agent-tool communication |
| OpenAI | Swarm framework | Multi-agent orchestration research | Lightweight agent handoff patterns; routines as first-class concept |
| LangChain | LangGraph | Graph-based agent orchestration | DAG-based agent workflows enable complex branching and cycles |
| Google DeepMind | Gemini agent research | Long-context agents with 1M+ token windows | Context length reduces need for complex retrieval; enables single-pass reasoning |
| AutoGPT / BabyAGI | Open-source autonomous agents | Early autonomous loop implementations | Demonstrated both promise and pitfalls of unconstrained autonomy |

---

## Key Uncertainties

| Uncertainty | Optimistic Scenario | Pessimistic Scenario | Impact on Roadmap |
|------------|---------------------|---------------------|-------------------|
| Regulation speed | Light-touch regulation; industry self-governance works | Heavy prescriptive regulation; agent capabilities capped by law | Determines whether L5 autonomy is legally viable by 2027 |
| Capability ceilings | Steady capability gains; agents reach human-expert level on most coding tasks by 2027 | Diminishing returns; agents plateau at junior-to-mid level | Determines whether full autonomy is technically feasible |
| Cost trajectories | 100x cost reduction by 2027; agents cheaper than human labor for most tasks | Costs plateau; high-capability models remain expensive | Determines economic viability of always-on agent networks |
| Safety breakthroughs | Formal verification of agent behavior; provable guardrails | Safety remains empirical; no guarantees; incidents increase | Determines trust level and governance overhead |
| Memory and learning | Agents develop true long-term learning; improve across thousands of cycles | Memory remains brittle; context windows are the ceiling | Determines whether L5 agents can truly self-improve |
| Multi-agent coordination | Efficient agent-to-agent protocols emerge; near-zero coordination overhead | Coordination overhead grows with agent count; diminishing returns past 5-10 agents | Determines scalability of agent networks |
| Enterprise trust | Enterprises embrace agent autonomy after successful pilots | High-profile agent failures erode trust; adoption stalls | Determines market trajectory timeline |
| Open vs closed ecosystems | Open protocols (MCP) win; interoperability across vendors | Vendor lock-in; fragmented agent ecosystems | Determines whether federated agent networks are possible |
