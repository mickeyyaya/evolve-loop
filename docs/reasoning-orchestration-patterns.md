> **Reasoning Orchestration Patterns** — Reference for selecting and applying reasoning techniques in LLM agent pipelines. Use this document to choose the right reasoning strategy for each agent phase based on task complexity, token budget, and accuracy requirements.

## Table of Contents

1. [Reasoning Techniques Taxonomy](#reasoning-techniques-taxonomy)
2. [Selection Matrix](#selection-matrix)
3. [Mapping to Evolve-Loop](#mapping-to-evolve-loop)
4. [Prior Art](#prior-art)
5. [Anti-Patterns](#anti-patterns)

---

## Reasoning Techniques Taxonomy

| Technique | Mechanism | Strengths | Weaknesses | Token Cost |
|-----------|-----------|-----------|------------|------------|
| **Chain-of-Thought (CoT)** | Sequential step-by-step reasoning in natural language | Simple to implement; broadly effective; transparent trace | Verbose; linear exploration only; quality degrades on branching problems | Medium |
| **Tree-of-Thought (ToT)** | Branch and evaluate multiple reasoning paths in parallel | Explores alternatives; backtracks from dead ends; higher accuracy on complex tasks | High token cost (multiple branches); requires evaluation heuristic; slower | High |
| **PDDL-Instruct** | Translate task into formal Planning Domain Definition Language, then solve with logical planner | 94% planning accuracy on benchmarks; handles dependencies and preconditions; verifiable plans | Requires domain formalization; brittle on ill-defined problems; setup overhead | Medium |
| **Latent Reasoning (Coconut/Heima)** | Compress reasoning into continuous embedding space instead of discrete tokens | Dramatically lower token usage; maintains reasoning quality; faster inference | Requires specialized training; not interpretable; limited to supported models | Low |
| **CPO (Consistency Preference Optimization)** | Distill Tree-of-Thought quality into efficient Chain-of-Thought via preference training | CoT-level token cost with ToT-level quality; no branching at inference time | Requires offline training phase; quality bounded by ToT teacher; model-specific | Low-Medium |

### Technique Details

| Technique | How to Apply | Key Constraint |
|-----------|-------------|----------------|
| CoT | Prepend "Think step by step" or use structured prompt template | Keep chain under 10 steps to avoid drift |
| ToT | Generate N candidate paths, score each, expand best candidates | Cap branch factor at 3-5; limit depth to 3-4 levels |
| PDDL-Instruct | Define domain predicates and actions, state initial/goal conditions, invoke planner | Ensure domain definition covers all relevant actions |
| Latent Reasoning | Use Coconut/Heima-trained model; no prompt change needed | Verify model supports latent reasoning mode |
| CPO | Fine-tune base model on ToT-generated preference pairs | Collect 1000+ preference pairs for stable training |

---

## Selection Matrix

| Task Type | Complexity | Recommended Technique | Rationale |
|-----------|------------|----------------------|-----------|
| Simple single-step (S-task) | Low | Standard CoT | Sufficient accuracy; minimal token overhead |
| Multi-step with known path | Medium | CoT with structured template | Step-by-step trace keeps execution on track |
| Complex with alternatives (M-task) | High | ToT or CPO | Explore branches to find optimal solution path |
| Planning with dependencies | High | PDDL-Instruct | Formal planning handles preconditions and ordering |
| Budget-constrained inference | Any | Latent Reasoning | Minimize token usage while preserving reasoning quality |
| High-volume repeated tasks | Medium-High | CPO | Amortize ToT quality over many fast inferences |
| Verification and review | Medium | CoT | Systematic checklist-style reasoning is transparent and auditable |
| Creative exploration | High | ToT | Breadth-first exploration surfaces non-obvious solutions |

---

## Mapping to Evolve-Loop

| Phase | Agent | Recommended Technique | Application |
|-------|-------|-----------------------|-------------|
| Scout | Scout | ToT | Explore multiple candidate tasks; score and rank alternatives before committing |
| Build | Builder | CoT | Execute implementation step-by-step; maintain clear reasoning trace for audit |
| Audit | Auditor | CoT | Walk through code systematically; verify each requirement against checklist |
| Ship | Operator | CPO | Summarize cycle results efficiently; distill complex outcomes into concise reports |
| Learn | Operator | CoT | Extract lessons learned sequentially; update memory with structured reflections |

### Phase-Specific Guidance

| Phase | Avoid | Reason |
|-------|-------|--------|
| Scout | Pure CoT without alternatives | Misses better task candidates; linear thinking creates selection bias |
| Builder | ToT during implementation | Branching mid-implementation wastes tokens; commit to one path after Scout |
| Auditor | Latent Reasoning | Audit requires transparent, inspectable reasoning traces |
| Ship | ToT | Summarization does not benefit from branching; use efficient single-pass |

---

## Prior Art

| Reference | Year | Contribution | Key Finding |
|-----------|------|-------------|-------------|
| Wei et al. — Chain-of-Thought Prompting | 2022 | Introduced CoT prompting for LLMs | Step-by-step reasoning dramatically improves accuracy on math and logic tasks |
| Yao et al. — Tree of Thoughts | 2023 | Generalized CoT to tree-structured exploration | ToT solves problems where CoT fails by exploring and backtracking |
| PDDL-Instruct | 2025 | Formal planning language integration with LLMs | Achieves 94% planning accuracy by leveraging classical AI planning |
| CPO (Consistency Preference Optimization) | 2025 | Distill ToT into CoT via preference optimization | Matches ToT quality at CoT inference cost |
| Coconut (Continuous Chain-of-Thought) | 2024 | Reasoning in latent embedding space | Reduces token usage while maintaining reasoning performance |
| Heima | 2025 | Extended latent reasoning with improved training | Improves on Coconut with better training stability and broader task coverage |

---

## Anti-Patterns

| Anti-Pattern | Description | Consequence | Mitigation |
|-------------|-------------|-------------|------------|
| Reasoning overkill | Apply ToT to simple tasks that CoT handles well | Wasted tokens and latency with no accuracy gain | Match technique to task complexity using the selection matrix |
| CoT verbosity | Allow unbounded chain length without step limits | Context window exhaustion; reasoning drift in later steps | Cap chain length; use structured templates with explicit step counts |
| Unconstrained search | Run ToT without branch factor or depth limits | Exponential token explosion; timeout on complex problems | Set branch factor (3-5) and depth limit (3-4) before invoking ToT |
| Ignoring reasoning costs | Select technique without considering token budget | Budget overrun; fewer cycles per session | Track token usage per technique; prefer CPO or latent reasoning when budget-constrained |
| Opaque reasoning in audits | Use latent reasoning for verification tasks | Cannot inspect or validate the reasoning trace | Reserve latent reasoning for non-critical paths; use CoT for all audit and review |
| Static technique selection | Use the same technique for every task regardless of complexity | Suboptimal cost-quality tradeoff across diverse tasks | Re-evaluate technique choice per phase and per task type |
