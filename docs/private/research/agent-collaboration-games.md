# Agent Collaboration Games

> Reference document on competitive and cooperative dynamics between agents in
> multi-agent systems. Use game-theoretic framing to design agent interactions
> that maximize collective output while preventing degenerate equilibria.

## Table of Contents

1. [Game Dynamics Taxonomy](#game-dynamics-taxonomy)
2. [Collaboration Patterns](#collaboration-patterns)
3. [Mapping to Evolve-Loop](#mapping-to-evolve-loop)
4. [Implementation Patterns](#implementation-patterns)
5. [Prior Art](#prior-art)
6. [Anti-Patterns](#anti-patterns)

---

## Game Dynamics Taxonomy

| Dynamic | Structure | Payoff Distribution | Example | When to Use |
|---|---|---|---|---|
| **Pure Cooperation** | All agents share a single reward signal | Maximize joint utility; no agent benefits from another's loss | Ensemble voting on code quality | Tasks where consensus improves accuracy |
| **Pure Competition** | Zero-sum; one agent's gain is another's loss | Winner-take-all selection of best output | Multiple builders racing to produce the best implementation | Tasks where diversity of attempts increases hit rate |
| **Hybrid Cooperative-Competitive** | Agents cooperate within teams but compete across teams | Team reward + individual bonus for best contribution | Scout-Builder pairs competing against alternative pairs | Tasks requiring both specialization and selection pressure |
| **Asymmetric (Pioneer/Observer)** | One agent explores; another critiques or refines | Pioneer earns discovery reward; observer earns refinement reward | Pioneer generates code, observer identifies flaws and improves | Tasks where exploration and exploitation must be separated |
| **Iterated Game** | Repeated interactions with memory of past rounds | Agents adapt strategies based on history; reputation matters | Multi-cycle evolve-loop where agents learn from prior audits | Long-running systems where trust accumulates over time |
| **Mechanism Design** | Orchestrator sets rules; agents optimize within constraints | Orchestrator maximizes system utility via incentive alignment | Phase-gate script enforcing quality thresholds | Tasks where a central authority can define and enforce rules |

---

## Collaboration Patterns

| Pattern | Agents | Interaction Mode | Mechanism | Strengths | Weaknesses |
|---|---|---|---|---|---|
| **CORY Dual-LLM** | Pioneer + Observer | Asymmetric cooperative | Pioneer explores solution space; observer refines and validates | Separates exploration from exploitation; reduces hallucination | Observer bottleneck; pioneer may overfit to observer preferences |
| **MACC Competitive Exploration** | N parallel explorers | Competitive with shared memory | Agents explore independently; blackboard aggregates findings | Maximizes coverage of solution space; fault-tolerant | Duplication of effort; coordination overhead |
| **Debate Protocol** | Proponent + Opponent | Adversarial reasoning | One agent argues for a solution; the other argues against; judge decides | Surfaces hidden flaws; reduces overconfidence | May devolve into rhetoric; judge must be competent |
| **Ensemble Voting** | N parallel solvers | Cooperative aggregation | Each agent proposes a solution; majority vote or weighted consensus selects | Reduces variance; robust to individual errors | Groupthink if agents share biases; cost scales linearly |
| **Mixture of Agents (MoA)** | N generators + 1 aggregator | Layered cooperative | Layer 1 generates diverse proposals; layer 2 synthesizes best elements | Combines diversity with synthesis; proven quality gains | Aggregator may introduce its own biases; latency |
| **Red Team / Blue Team** | Attacker + Defender | Adversarial cooperative | Red team finds vulnerabilities; blue team patches them | Thorough security/quality coverage | Expensive; red team may miss novel attack vectors |
| **Chain of Specialists** | Ordered pipeline of N agents | Sequential cooperative | Each agent handles one phase; output feeds the next | Clear responsibility; easy debugging | Pipeline latency; error propagation |

---

## Mapping to Evolve-Loop

### Agent Pair Dynamics

| Agent Pair | Game Dynamic | Interaction | Rationale |
|---|---|---|---|
| **Scout + Builder** | Cooperative | Scout discovers tasks; Builder implements them | Shared goal: produce high-quality code changes |
| **Builder + Auditor** | Adversarial (debate) | Builder produces code; Auditor critiques it | Tension improves quality; Auditor catches Builder blind spots |
| **Scout + Auditor** | Asymmetric observer | Scout provides context; Auditor validates alignment | Auditor verifies Scout's task selection was sound |
| **Orchestrator + All Agents** | Mechanism design | Orchestrator defines rules, gates, and rewards | Central authority enforces quality thresholds and process integrity |

### Evolve-Loop as Tournament

| Concept | Tournament Analog | Implementation |
|---|---|---|
| **Single Cycle** | One match | Scout-Build-Audit-Ship-Learn pipeline |
| **Multi-Cycle Run** | Tournament round | N cycles with accumulated fitness scores |
| **Gene Pool** | Player roster | Configuration parameters that evolve across cycles |
| **Phase Gate** | Referee | Deterministic script enforcing pass/fail at each boundary |
| **Fitness Score** | Match score | Composite metric from Auditor evaluation |
| **Selection Pressure** | Elimination | Low-fitness genes are replaced; high-fitness genes propagate |

### Self-MoA Variants as Competitive Dynamics

| Variant | Competition Type | Selection Mechanism |
|---|---|---|
| **Parallel Builders** | Pure competition | Auditor selects best implementation from N candidates |
| **Diverse Scout Strategies** | Competitive exploration | Orchestrator picks highest-value task from N scout reports |
| **Adversarial Auditing** | Red team | Multiple auditors with different focus areas (security, perf, correctness) |
| **Meta-Cycle Tournament** | Iterated competition | Gene configurations compete across cycles; fittest survive |

---

## Implementation Patterns

### Reward Structure Design

| Principle | Description | Example |
|---|---|---|
| **Align individual and collective** | Each agent's reward correlates with system-level outcome | Builder reward = f(audit score, ship success) |
| **Separate process from outcome** | Reward good process even when outcome is unlucky | Scout earns credit for thorough exploration regardless of task difficulty |
| **Use relative scoring** | Rank agents against each other, not absolute thresholds | In parallel-builder mode, score relative to median quality |
| **Decay stale rewards** | Recent performance matters more than historical | Exponential decay on fitness contributions from old cycles |
| **Penalize gaming** | Detect and punish reward hacking behaviors | Auditor flags suspiciously high self-reported scores |

### Preventing Groupthink

| Technique | Mechanism | When to Apply |
|---|---|---|
| **Enforce diversity** | Require agents to use different prompts, models, or temperature settings | Ensemble voting, parallel builders |
| **Devil's advocate role** | Assign one agent to argue the opposite position | Consensus decisions, architecture reviews |
| **Anonymous submission** | Hide agent identity during evaluation | Competitive selection to prevent reputation bias |
| **Minority report** | Require dissenting opinions to be recorded and addressed | Any voting or consensus pattern |
| **Rotation** | Rotate agent roles across cycles | Long-running systems to prevent entrenchment |

### Balancing Cooperation and Competition

| Strategy | Description | Trade-off |
|---|---|---|
| **Cooperation floor** | Set minimum collaboration requirements before competition begins | Ensures baseline coordination; limits competitive pressure |
| **Competition ceiling** | Cap competitive rewards to prevent destructive rivalry | Prevents sabotage; may reduce motivation |
| **Shared context pool** | All agents read from common blackboard; compete on interpretation | Reduces duplication; may homogenize approaches |
| **Phased transition** | Start cooperative (exploration), shift competitive (selection) | Matches natural task progression; adds orchestration overhead |
| **Team-based competition** | Cooperative within teams, competitive across teams | Combines benefits; requires team formation logic |

---

## Prior Art

| Work | Year | Key Contribution | Relevance to Evolve-Loop |
|---|---|---|---|
| **CORY** (Cooperative Refinement) | 2025 | Dual-LLM pioneer/observer pattern for collaborative reasoning | Direct model for Scout/Builder cooperation |
| **MACC** (Multi-Agent Competitive Cooperation) | 2026 | Competitive exploration with shared memory in AAMAS proceedings | Framework for parallel-builder tournament dynamics |
| **AI Safety via Debate** (Irving et al.) | 2018 | Adversarial debate between agents judged by human or AI | Foundation for Builder/Auditor adversarial dynamic |
| **CAMEL** (Communicative Agents for Mind Exploration) | 2023 | Role-playing framework for autonomous agent cooperation | Inception prompting technique for role assignment |
| **ChatArena** | 2023 | Multi-agent language game environment with Elo rating | Tournament and ranking mechanics for agent evaluation |
| **AutoGen** (Microsoft) | 2023 | Multi-agent conversation framework with flexible topologies | Group chat patterns for agent coordination |
| **CrewAI** | 2024 | Role-based multi-agent orchestration with process models | Sequential and hierarchical process patterns |
| **Mixture-of-Agents** (Together AI) | 2024 | Layered LLM collaboration outperforming single models | Self-MoA architecture for quality amplification |
| **Constitutional AI** (Anthropic) | 2022 | Self-critique and revision using principles | Model for Auditor's principle-based evaluation |
| **LLM Debate** (Du et al.) | 2023 | Multi-agent debate improves factuality and reasoning | Evidence that adversarial dynamics reduce errors |

---

## Anti-Patterns

| Anti-Pattern | Description | Symptom | Mitigation |
|---|---|---|---|
| **Groupthink** | All agents converge on the same flawed solution | Unanimous agreement on incorrect output; no dissent | Enforce diversity; add devil's advocate role; use different model temperatures |
| **Race to Bottom** | Competitive pressure causes agents to cut corners | Faster but lower-quality outputs; gaming of speed metrics | Include quality gates; penalize low audit scores; reward thoroughness |
| **Collusion** | Agents coordinate to game the reward system together | Suspiciously high mutual ratings; audit scores disconnected from actual quality | Use independent evaluation; rotate pairings; add external validation |
| **Ignoring Minority Signals** | Majority vote suppresses valid minority objections | Rare but critical bugs slip through; dissenting agents learn to stay silent | Require minority reports; weight novel objections higher; log all dissent |
| **Sycophantic Cooperation** | Agents agree with each other to avoid conflict | Auditor consistently passes low-quality code; no substantive critique | Set minimum critique requirements; reward finding real issues; adversarial scoring |
| **Destructive Competition** | Agents sabotage each other instead of producing better work | Agents waste tokens attacking competitors; overall quality drops | Cap competitive incentives; enforce cooperation floor; penalize sabotage |
| **Free Riding** | Some agents contribute minimally while benefiting from group reward | Uneven contribution distribution; some agents produce near-empty outputs | Track individual contributions; require minimum output quality per agent |
| **Oscillation** | Adversarial pair alternates between opposing positions without converging | Builder and Auditor cycle endlessly; no stable solution reached | Set maximum debate rounds; use escalation to tiebreaker; require convergence criteria |
