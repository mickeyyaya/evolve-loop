# Agent Role Specialization

> Reference document for designing specialized agent roles in multi-agent systems.
> Apply single-responsibility principles, formal persona schemas, and hierarchical
> architectures to maximize agent effectiveness and minimize role confusion.

## Table of Contents

1. [Role Design Principles](#role-design-principles)
2. [Persona Schema](#persona-schema)
3. [Hierarchical Role Architectures](#hierarchical-role-architectures)
4. [Mapping to Evolve-Loop](#mapping-to-evolve-loop)
5. [Emergent Role Formation](#emergent-role-formation)
6. [Prior Art](#prior-art)
7. [Anti-Patterns](#anti-patterns)

---

## Role Design Principles

| Principle | Definition | Rationale | Enforcement Mechanism |
|---|---|---|---|
| **Single Responsibility** | Each agent owns exactly one concern (e.g., search, build, audit) | Reduces context bloat; enables independent scaling and replacement | Define one primary verb per role; reject tasks outside that verb |
| **Minimal Overlap** | No two agents share the same write target or decision authority | Prevents conflicting actions and duplicated work | Assign exclusive file ownership; use artifact boundaries |
| **Clear Interfaces** | Define explicit inputs, outputs, and handoff contracts between roles | Enables composition without tight coupling | Use typed artifacts (scout-report, build-report) as interface contracts |
| **Trust Boundaries** | Restrict each role's tool access and file permissions to its scope | Limits blast radius of errors or adversarial behavior | Allowlist tools per role; enforce read-only vs read-write access |
| **Composability** | Design roles that combine into pipelines without modification | Enables reuse across different orchestration patterns | Standardize input/output formats; avoid role-specific protocols |
| **Graceful Degradation** | Each role must handle missing or malformed inputs without crashing | Prevents cascade failures in multi-agent pipelines | Validate inputs at role boundary; emit structured error artifacts |

---

## Persona Schema

Define every agent role using this formal structure.

| Field | Type | Description | Example |
|---|---|---|---|
| `name` | string | Unique identifier for the role | `scout` |
| `purpose` | string | One-sentence mission statement (imperative voice) | "Identify the highest-value task for the next cycle" |
| `capabilities` | string[] | Exhaustive list of what this role can do | `["codebase search", "risk assessment", "task ranking"]` |
| `tools` | string[] | Allowlisted tools this role may invoke | `["Grep", "Glob", "Read", "WebSearch"]` |
| `inputs` | artifact[] | Named artifacts this role consumes (read-only) | `["genes.md", "instincts.md", "ledger.jsonl"]` |
| `outputs` | artifact[] | Named artifacts this role produces (exclusive write) | `["scout-report.md"]` |
| `constraints` | string[] | Hard rules this role must never violate | `["Never modify source code", "Never write to ledger"]` |
| `escalation_rules` | rule[] | Conditions under which this role must defer to another | `["If risk > HIGH, escalate to Auditor before Builder"]` |
| `context_budget` | number | Maximum tokens this role may consume per invocation | `32000` |
| `success_criteria` | string[] | Measurable outcomes that define task completion | `["Exactly one task selected", "Risk assessment included"]` |

### Persona Template

```yaml
name: <role-name>
purpose: <imperative one-liner>
capabilities:
  - <capability-1>
  - <capability-2>
tools:
  - <tool-1>
  - <tool-2>
inputs:
  - <artifact-name>
outputs:
  - <artifact-name>
constraints:
  - <hard-rule>
escalation_rules:
  - condition: <trigger>
    target: <role-name>
context_budget: <tokens>
success_criteria:
  - <measurable-outcome>
```

---

## Hierarchical Role Architectures

| Architecture | Structure | Strengths | Weaknesses | Best For |
|---|---|---|---|---|
| **Orchestrator-Specialist** | Central orchestrator dispatches to specialist agents | Clear control flow; easy to reason about; deterministic sequencing | Orchestrator becomes bottleneck; single point of failure | Sequential pipelines (evolve-loop, CI/CD) |
| **Peer Collaboration** | Agents communicate directly via shared artifacts | No single bottleneck; agents self-coordinate | Risk of circular dependencies; harder to debug | Brainstorming, research, code review |
| **Dynamic Role Assignment** | Meta-agent assigns roles based on task requirements | Adapts to novel tasks; efficient resource use | Requires role-assignment intelligence; risk of misassignment | Variable workloads, heterogeneous tasks |
| **Hierarchical Delegation** | Tree of orchestrators; each level delegates to sub-specialists | Scales to large agent counts; mirrors org structure | Deep trees add latency; complex error propagation | Large-scale systems (100+ agents) |
| **Blackboard** | Agents read/write to a shared knowledge store; act when preconditions are met | Decoupled; agents join/leave dynamically | Race conditions; requires conflict resolution | Event-driven systems, monitoring |

### Orchestrator-Specialist Flow

```
Orchestrator
  ├── Scout (search, analyze, rank)
  ├── Builder (implement, test, document)
  └── Auditor (verify, score, approve/reject)
```

| Concern | Orchestrator Responsibility | Specialist Responsibility |
|---|---|---|
| **Task selection** | Invoke Scout; receive ranked task list | Scout searches codebase; emits scout-report |
| **Implementation** | Pass selected task to Builder | Builder writes code; emits build-report |
| **Verification** | Pass build artifacts to Auditor | Auditor runs checks; emits audit-report |
| **Decision** | Accept/reject based on audit score | None; specialists do not make pipeline decisions |
| **Error handling** | Retry, reassign, or abort cycle | Report failure in output artifact; never self-retry |

---

## Mapping to Evolve-Loop

| Role | Evolve-Loop Agent | Primary Verb | Exclusive Artifacts | Tools Allowed | Trust Level |
|---|---|---|---|---|---|
| **Scout** | Scout agent | Search | `scout-report.md` | Read, Grep, Glob, WebSearch | Read-only codebase access |
| **Builder** | Builder agent | Implement | `build-report.md`, source files | Read, Edit, Write, Bash, LSP | Read-write codebase access |
| **Auditor** | Auditor agent | Verify | `audit-report.md` | Read, Grep, Bash (test only) | Read-only codebase + test execution |
| **Operator** | Orchestrator (main) | Coordinate | `ledger.jsonl`, `cycle-log/` | All (delegates to specialists) | Full pipeline control |

### Persona Definitions in Evolve-Loop

| Concern | Implementation | File |
|---|---|---|
| Role personas | YAML templates defining each agent's schema | `agent-templates.md` |
| Role constraints | Shared values enforced across all roles | `instincts.md` (instincts = hard constraints) |
| Role capabilities | Gene-defined behaviors that evolve over cycles | `genes.md` (genes = evolvable capabilities) |
| Role interfaces | Artifact contracts between phases | `phases.md` (phase = role boundary) |

### Shared Values as Role Constraints

| Instinct | Constraint on Scout | Constraint on Builder | Constraint on Auditor |
|---|---|---|---|
| **Integrity** | Never fabricate task candidates | Never skip tests | Never inflate scores |
| **Determinism** | Produce reproducible search results | Produce deterministic builds | Produce reproducible audits |
| **Transparency** | Log search rationale in report | Log implementation decisions | Log scoring rationale |

---

## Emergent Role Formation

Roles need not be statically defined. Performance data can drive role creation and refinement.

| Mechanism | Description | Trigger Condition | Outcome |
|---|---|---|---|
| **Performance Clustering** | Group agent behaviors by outcome similarity | Agents with >80% task overlap detected | Merge redundant roles; split overloaded roles |
| **Capability Discovery** | Track which tools each role actually uses vs allowlisted | Role consistently uses <50% of allowed tools | Narrow tool allowlist; tighten trust boundary |
| **Specialization Pressure** | Reward narrow expertise over generalist performance | Specialist outperforms generalist on domain tasks | Fork generalist into domain-specific sub-roles |
| **Role Retirement** | Detect roles that never produce unique value | Role output is always overridden or ignored | Remove role from pipeline; redistribute responsibilities |
| **Dynamic Spawning** | Create new roles when existing roles hit context limits | Single role exceeds context budget repeatedly | Split role by sub-concern; assign new persona schema |

### Project Sid Findings

| Finding | Implication for Role Design |
|---|---|
| 1000 agents formed emergent social structures without explicit role assignment | Roles can self-organize given shared objectives and communication channels |
| Agents developed specialized behaviors through repeated interaction | Performance feedback loops drive natural specialization |
| Social norms emerged to resolve conflicts between agents | Shared constraints (instincts) serve as synthetic social norms |
| Hierarchies formed based on demonstrated competence | Trust levels should be earned, not only statically assigned |
| Role stability increased over time as agents found niches | Allow role definitions to stabilize through evolution, not just top-down design |

---

## Prior Art

| Project | Architecture | Role Design Approach | Key Insight |
|---|---|---|---|
| **Project Sid** (Altera, 2024) | 1000-agent Minecraft society | Emergent roles from social interaction; no predefined specializations | Roles emerge naturally when agents have shared goals and communication |
| **CrewAI** | Orchestrator-specialist | Explicit role/goal/backstory per agent; sequential or parallel task execution | Backstory field improves role adherence by providing motivation context |
| **AutoGen** (Microsoft) | Conversational agents | Roles defined by system message; agents converse to solve tasks | Conversation-based coordination enables flexible role boundaries |
| **MetaGPT** | Software company simulation | Predefined roles (PM, Architect, Engineer, QA); waterfall pipeline | Structured output schemas (SRS, design doc) enforce role boundaries better than instructions alone |
| **ChatDev** | Software company simulation | Roles mirror real job titles; chat-chain coordination | Phase-based handoffs reduce role confusion vs free-form conversation |
| **CAMEL** | Two-agent role-play | Instructor and assistant roles; inception prompting | Minimal role count (2) with clear power asymmetry produces focused output |
| **AgentVerse** | Dynamic group chat | Roles recruited dynamically based on task requirements | Dynamic recruitment outperforms static role assignment on diverse tasks |

### Comparison of Role Definition Mechanisms

| Mechanism | Used By | Pros | Cons |
|---|---|---|---|
| System message only | AutoGen, CAMEL | Simple; easy to modify | Weak enforcement; role drift over long conversations |
| Structured schema (YAML/JSON) | Evolve-loop, MetaGPT | Machine-readable; validatable; composable | Requires tooling to interpret and enforce |
| Backstory narrative | CrewAI | Improves adherence; provides motivation | Consumes tokens; harder to validate programmatically |
| Emergent from interaction | Project Sid, AgentVerse | Adapts to novel situations; no upfront design needed | Unpredictable; slow convergence; hard to debug |

---

## Anti-Patterns

| Anti-Pattern | Description | Symptoms | Fix |
|---|---|---|---|
| **Role Bloat** | Too many narrowly-defined roles that fragment simple tasks | High coordination overhead; most roles idle; excessive handoffs | Merge roles with >70% tool/artifact overlap; aim for 3-5 roles per pipeline |
| **God Agent** | Single agent handles all concerns with no delegation | Context window exhaustion; inconsistent quality; no error isolation | Decompose by concern; assign one primary verb per agent |
| **Unclear Boundaries** | Two or more roles write to the same artifact or make the same decision | Conflicting outputs; race conditions; duplicated work | Enforce exclusive artifact ownership; one writer per file |
| **Role Confusion from Shared Context** | Agent receives context intended for a different role | Agent acts outside its scope; produces irrelevant output | Isolate context per role; use per-phase context matrices |
| **Static Role Ossification** | Roles never change despite shifting requirements | Pipeline handles old task shapes well but fails on new ones | Review role definitions quarterly; use performance data to trigger role evolution |
| **Premature Specialization** | Creating specialized roles before understanding the problem domain | Roles misaligned with actual needs; frequent redesigns | Start with generalist roles; specialize only after performance data justifies it |
| **Circular Delegation** | Agent A delegates to Agent B, which delegates back to Agent A | Infinite loops; stack overflow; wasted tokens | Enforce acyclic delegation graphs; orchestrator breaks cycles |
| **Trust Escalation** | Agent accumulates permissions beyond its original scope over time | Security boundary erosion; blast radius grows silently | Audit tool allowlists per cycle; reset permissions on role redefinition |
