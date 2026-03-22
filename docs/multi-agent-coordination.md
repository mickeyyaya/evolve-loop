# Multi-Agent Coordination

How the evolve-loop's agents coordinate across phases, and how research on multi-agent topology can inform future improvements.

---

## Current Topology

The evolve-loop uses a **sequential pipeline with embedded parallel eval execution**:

```
Scout → Builder → Auditor → Ship → Learn
         ↓ (per task)          ↓
    [worktree isolation]  [parallel eval graders]
```

This is the correct topology for a strict dependency chain — each phase depends on the previous phase's output. The sequential design prevents information leakage and ensures the Auditor reviews work it did not create.

---

## Topology-Aware Routing (AdaptOrch-Inspired)

AdaptOrch (arXiv:2602.16873) demonstrates that topology-aware routing outperforms static single-topology baselines by 12-23%. The key insight: decompose tasks into a DAG (directed acyclic graph) before choosing how to execute them.

### Four Canonical Topologies

| Topology | When to Use | Evolve-Loop Application |
|----------|-------------|------------------------|
| **Sequential** | Strict dependency chain | Scout → Builder → Auditor (current design) |
| **Parallel** | Independent subtasks | Multiple eval graders, multi-task builds in separate worktrees |
| **Hierarchical** | Coordinator + specialists | Orchestrator dispatching to Scout/Builder/Auditor agents |
| **Hybrid** | Mixed dependencies | Inline S-tasks (sequential) + worktree M-tasks (parallel) per cycle |

### DAG-Based Task Routing

Before executing a cycle's task list, model dependencies as a DAG:

```
Tasks: [A, B, C]
Dependencies: A→C (C reads A's output file), B independent

DAG:
  A ──→ C
  B (independent)

Routing: Execute A and B in parallel. Execute C after A completes.
```

The orchestrator already does this informally (inline tasks first, worktree tasks in parallel). AdaptOrch formalizes the decision:
- Nodes with in-degree 0 → eligible for parallel execution
- Chains (linear dependencies) → sequential execution
- Fan-out nodes (one output feeds multiple consumers) → hierarchical coordination

### Parallel Eval Execution

Eval graders are independent by design — each checks a different aspect of the build. Running graders in parallel reduces the eval phase wall-clock time proportionally to the number of graders.

```bash
# Sequential (current): ~N * grader_time
for grader in graders; do bash "$grader"; done

# Parallel (optimized): ~max(grader_times)
for grader in graders; do bash "$grader" & done; wait
```

### Adaptive Synthesis for Multi-Task Cycles

When multiple tasks ship in a single cycle, the Operator synthesizes lessons across all tasks. AdaptOrch's consistency voting pattern applies: if two tasks produced contradictory instincts, the Operator should flag the inconsistency rather than silently keeping both.

---

## Hierarchical Decomposition with Prompt Evolution (arXiv:2602.21670)

A two-layer hierarchy (coordinator decomposes → specialists execute) outperforms flat single-agent planners by +59pp on vague tasks. When combined with TextGrad-style prompt optimization (failure critiques backpropagate as natural-language updates), gains increase by an additional +37pp.

**Application to evolve-loop:**

| Hierarchy Concept | Evolve-Loop Mapping |
|-------------------|---------------------|
| Coordinator decomposes vague goal | Scout breaks goal into milestones (GoalAct) then into tasks |
| Specialists generate PDDL plans | Builder generates implementation plans per task |
| TextGrad failure backpropagation | Auditor FAIL verdict feeds back as targeted retry context |
| Meta-prompt sharing across same-layer agents | Instincts shared across Builder invocations within a cycle |

**TextGrad-style prompt evolution:** When a Builder fails audit, the Auditor's critique should be structured as a prompt refinement signal — not just "this failed" but "the design step should have X." This critique feeds into the meta-cycle prompt evolution mechanism (Phase 6), where agent prompts are updated based on accumulated failure patterns.

**Meta-prompt sharing:** When multiple tasks run in parallel, successful Builder approaches from task A can inform task B's prompt. The `experiments.jsonl` journal enables this: the orchestrator reads recent successful experiment entries and injects them as meta-prompts for subsequent Builder invocations.

---

## Multi-Agent Coordination Anti-Patterns

| Anti-Pattern | Description | Mitigation |
|-------------|-------------|------------|
| Over-parallelization | Running dependent tasks in parallel causes conflicts | DAG analysis before execution |
| Coordinator bottleneck | Orchestrator becomes a single point of failure | Delegate synthesis to Operator agent |
| Communication overhead | Agents exchange more context than needed | Handoff files (compact contracts, not full reports) |
| Topology lock-in | Always using the same topology regardless of task shape | DAG-based routing per cycle |

---

## Research References

- AdaptOrch (arXiv:2602.16873): Task-adaptive topology routing with DAG decomposition
- RAPS (arXiv:2602.08009): Adaptive, scalable coordination with robustness guarantees
- MultiAgentBench (arXiv:2503.01935): Graph topology outperforms star/chain/tree for coordination

See [research-paper-index.md](research-paper-index.md) for the full citation index.
