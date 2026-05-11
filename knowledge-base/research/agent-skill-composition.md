# Agent Skill Composition

> Reference document for combining atomic skills into complex agent behaviors.
> Covers composition patterns, phase transition problems at scale, and mapping
> to evolve-loop's instinct/gene architecture. Use tables not prose; imperative voice.

## Table of Contents

1. [Skill Engineering Evolution](#skill-engineering-evolution)
2. [Composition Patterns](#composition-patterns)
3. [Phase Transition Problem](#phase-transition-problem)
4. [Mapping to Evolve-Loop](#mapping-to-evolve-loop)
5. [Implementation Patterns](#implementation-patterns)
6. [Prior Art](#prior-art)
7. [Anti-Patterns](#anti-patterns)

---

## Skill Engineering Evolution

| Era | Unit | Characteristics | Limitations |
|---|---|---|---|
| **Prompts** | Raw text instructions | Freeform, no versioning, no composition | Non-deterministic; drift across invocations; no reuse boundary |
| **Tools** | Function-call schemas | Typed inputs/outputs, discoverable, invocable by model | Stateless; no orchestration; no dependency declaration |
| **Skills** | Versioned, composable, parameterized units | Encapsulate prompt + tool + validation; declare dependencies; carry metadata (version, confidence, cost) | Require composition framework; selection accuracy degrades at scale |

| Skill Property | Definition | Example |
|---|---|---|
| **Versioned** | Each skill carries a semver tag; breaking changes increment major | `code-review@2.1.0` |
| **Composable** | Skills declare input/output types enabling pipeline assembly | `lint-check` outputs `LintReport` consumed by `fix-lint` |
| **Parameterized** | Accept runtime configuration without modifying skill definition | `test-runner(framework="vitest", coverage=80)` |
| **Idempotent** | Re-execution with same inputs produces same outputs | `format-code` applied twice yields identical result |
| **Observable** | Emit structured telemetry (duration, token cost, success/failure) | `{ skill: "scout-scan", tokens: 1200, status: "pass" }` |

---

## Composition Patterns

| Pattern | Description | When to Use | Example |
|---|---|---|---|
| **Sequential (Pipeline)** | Output of skill N feeds input of skill N+1 | Ordered transformations with data dependency | Scout scan → prioritize → select task |
| **Parallel (Fan-out)** | Execute multiple skills concurrently; merge results | Independent analyses of same input | Security review ∥ performance review ∥ style review |
| **Conditional (Branching)** | Select skill path based on runtime predicate | Different handling per input type or state | If test fails → debug skill; else → ship skill |
| **Hierarchical (Sub-skill)** | Parent skill delegates to child skills; aggregates results | Complex operations decomposable into reusable parts | Builder calls `write-code`, `run-tests`, `format` as sub-skills |
| **Recursive** | Skill invokes itself with reduced problem scope | Tree traversal, iterative refinement, retry with backoff | `refine-plan` calls itself until quality score ≥ threshold |
| **Event-driven** | Skill triggers on artifact change or external event | Reactive pipelines; watch-mode development | File change triggers `lint` → `test` → `report` |

### Pipeline Composition Example

```
scout-scan → task-prioritize → task-select → build-implement → audit-verify → ship-commit
     ↓              ↓               ↓               ↓                ↓             ↓
  scan-report   ranked-tasks   selected-task   build-report    audit-report    commit-hash
```

### Parallel Fan-out Example

```
                    ┌─ security-review ──┐
input-artifact ─────┤─ perf-review ──────┤─── merge-reviews → decision
                    └─ style-review ─────┘
```

---

## Phase Transition Problem

Selection accuracy collapses when the skill library exceeds a critical size threshold.
The model cannot reliably choose the correct skill from a large unstructured set.

| Library Size | Selection Accuracy | Failure Mode | Observed Behavior |
|---|---|---|---|
| 1–20 skills | >95% | Rare | Model selects correct skill consistently |
| 20–50 skills | 80–95% | Occasional misselection | Wrong skill chosen for edge cases |
| 50–100 skills | 50–80% | Frequent misselection | Model conflates similar skills; applies wrong one |
| 100+ skills | <50% | Systematic collapse | Model hallucinated skills or picks randomly |

### Mitigation Strategies

| Strategy | Mechanism | Effect on Accuracy | Implementation Cost |
|---|---|---|---|
| **Categorization** | Group skills into domains (scout, build, audit, ship) | Reduce effective search space per query | Low — add `category` field to skill metadata |
| **Relevance Scoring** | Score each skill against current context before selection | Filter to top-K candidates before model chooses | Medium — requires embedding or keyword matching |
| **Hierarchical Menus** | Present category first, then skills within category | Two-step selection reduces combinatorial space | Low — restructure skill registry as tree |
| **Pruning** | Remove low-confidence, low-usage skills periodically | Keep library below critical threshold | Low — add lifecycle hooks to meta-cycle |
| **Skill Aliases** | Map natural-language intents to canonical skill names | Reduce ambiguity in skill selection | Medium — maintain intent→skill mapping |
| **Context Injection** | Include only relevant skills in prompt based on phase | Eliminate irrelevant candidates entirely | Medium — dynamic prompt assembly per phase |

---

## Mapping to Evolve-Loop

| Evolve-Loop Concept | Skill Composition Equivalent | Relationship |
|---|---|---|
| **Instincts** | Atomic skills | Each instinct is a single, self-contained skill (e.g., `validate-json`, `check-coverage`) |
| **Genes** | Composed skill sequences | A gene chains multiple instincts into a behavior (e.g., Scout gene = `scan` + `prioritize` + `select`) |
| **Plan Cache** | Memoized skill pipelines | Cached sequences of skill invocations keyed by task signature; skip re-planning |
| **Confidence Scores** | Skill reliability metrics | Track success rate per skill; weight selection toward high-confidence skills |
| **Meta-cycle** | Skill evolution operator | Evaluate, mutate, and prune skills based on cumulative performance data |
| **Phase Gate** | Composition validator | Verify skill outputs meet quality gates before passing to next pipeline stage |

### Agent Role to Skill Mapping

| Agent | Atomic Skills (Instincts) | Composed Behavior (Gene) |
|---|---|---|
| **Scout** | `scan-codebase`, `identify-gaps`, `rank-tasks`, `estimate-complexity` | Scan → Rank → Select pipeline producing `scout-report.md` |
| **Builder** | `write-code`, `run-tests`, `fix-lint`, `format-code`, `update-state` | Implement → Test → Format pipeline producing `build-report.md` |
| **Auditor** | `verify-tests`, `check-coverage`, `validate-state`, `score-quality` | Verify → Score → Gate pipeline producing `audit-report.md` |

### Skill Evolution Through Confidence Scoring

| Confidence Range | Action | Trigger |
|---|---|---|
| 0.9–1.0 | Promote to default selection | 10+ successful invocations |
| 0.7–0.9 | Keep active; monitor | Normal operation |
| 0.5–0.7 | Flag for review | 3+ failures in recent window |
| 0.0–0.5 | Retire or rewrite | Consistent failure; replaced by better skill |

---

## Implementation Patterns

### Skill Interface Schema

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | `string` | Yes | Unique identifier (kebab-case) |
| `version` | `string` | Yes | Semver tag |
| `category` | `enum` | Yes | `scout` \| `build` \| `audit` \| `ship` \| `meta` |
| `description` | `string` | Yes | One-line purpose statement |
| `inputs` | `object[]` | Yes | Typed input parameters with defaults |
| `outputs` | `object[]` | Yes | Typed output artifacts |
| `dependencies` | `string[]` | No | Skills that must execute before this one |
| `confidence` | `float` | No | Current reliability score (0.0–1.0) |
| `costEstimate` | `object` | No | Expected token usage and latency |
| `retryPolicy` | `object` | No | Max retries, backoff strategy |

### Dependency Declaration

| Declaration Type | Syntax | Semantics |
|---|---|---|
| **Hard dependency** | `requires: ["lint-check@^2.0"]` | Fail if dependency unavailable or incompatible |
| **Soft dependency** | `enhancedBy: ["type-check"]` | Skip if unavailable; degrade gracefully |
| **Mutual exclusion** | `conflicts: ["legacy-lint"]` | Never execute both in same pipeline |
| **Ordering constraint** | `after: ["write-code"]` | Execute after named skill; no data dependency |

### Composition Validation

| Validation Rule | Check | Error |
|---|---|---|
| **Type compatibility** | Output type of skill N matches input type of skill N+1 | `TypeError: lint-check outputs LintReport but fix-code expects SourceFile` |
| **Cycle detection** | No circular dependencies in skill graph | `CycleError: a → b → c → a` |
| **Version compatibility** | Dependency version ranges overlap | `VersionError: skill-a requires lint@^2.0 but lint@1.3.0 installed` |
| **Category coherence** | Skills in a pipeline belong to compatible categories | `CategoryWarning: audit skill in build pipeline` |
| **Completeness** | All declared dependencies present in composition | `MissingError: required skill format-code not found` |

### Runtime Skill Selection

| Step | Action | Input | Output |
|---|---|---|---|
| 1 | **Parse intent** | Natural-language task description | Structured intent with category and verb |
| 2 | **Filter by category** | Intent category + skill registry | Candidate skill subset |
| 3 | **Score relevance** | Intent embedding vs. skill description embeddings | Ranked candidate list |
| 4 | **Apply constraints** | Dependency graph + conflict rules | Valid candidate list |
| 5 | **Select** | Top candidate by relevance × confidence | Chosen skill with parameters |
| 6 | **Execute** | Skill + parameters + context | Skill output artifact |

---

## Prior Art

| System | Contribution | Relevance to Evolve-Loop |
|---|---|---|
| **Voyager** (Wang et al., 2023) | Skill library for Minecraft agents; auto-generates and stores reusable skills | Demonstrates skill accumulation and retrieval; maps to instinct/gene evolution |
| **OpenAI Function Calling** | Typed function schemas exposed to LLM for structured invocation | Foundation for skill interface design; tool-use as atomic skill |
| **Claude Tool Use** | Multi-tool orchestration with parallel execution support | Parallel fan-out pattern; native composition within single turn |
| **DSPy Modules** | Declarative, composable LLM program modules with optimization | Module composition patterns; automatic prompt optimization maps to gene tuning |
| **SkillCoder** (Chen et al., 2024) | Generates reusable code skills from task demonstrations | Skill synthesis from examples; confidence-based skill selection |
| **AutoGPT Plugins** | Community-contributed capability extensions for autonomous agents | Plugin as skill pattern; demonstrates skill explosion anti-pattern |
| **LangGraph** | Stateful, multi-actor orchestration with conditional edges | Graph-based composition; maps to conditional and hierarchical patterns |
| **CrewAI** | Role-based multi-agent framework with task delegation | Agent-as-skill-executor pattern; hierarchical composition |

---

## Anti-Patterns

| Anti-Pattern | Description | Symptom | Mitigation |
|---|---|---|---|
| **Skill Explosion** | Create a new skill for every minor variation | Library exceeds critical threshold; selection accuracy collapses | Parameterize skills; merge similar skills; enforce minimum reuse count |
| **Monolithic Skills** | Single skill performs entire complex workflow | Cannot reuse sub-steps; testing requires full pipeline; high token cost | Decompose into atomic skills; compose via pipeline pattern |
| **Unvalidated Composition** | Chain skills without type-checking outputs/inputs | Runtime failures from incompatible data; silent data corruption | Enforce composition validation rules; type-check at assembly time |
| **Circular Dependencies** | Skill A requires B, B requires C, C requires A | Infinite loop or deadlock at runtime | Run cycle detection during registration; reject circular graphs |
| **Implicit Ordering** | Rely on execution order without declaring dependencies | Breaks when parallelized or reordered; non-deterministic results | Declare explicit `after` or `requires` constraints |
| **Stale Skills** | Keep unused skills in library indefinitely | Bloats selection space; increases misselection risk | Prune skills with zero usage over N cycles; enforce lifecycle policy |
| **God Orchestrator** | Central coordinator with knowledge of all skill internals | Tight coupling; single point of failure; context window overload | Use hierarchical composition; delegate sub-orchestration to category leads |
| **Copy-Paste Skills** | Duplicate skill logic instead of parameterizing | Maintenance burden; inconsistent behavior across copies | Extract shared logic; use parameterized base skill with configuration |
