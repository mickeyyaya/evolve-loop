> **Workflow DAG Patterns** — Reference for DAG-based workflow orchestration in agent pipelines. Use this document to evaluate topology trade-offs, select execution patterns, and map DAG concepts to evolve-loop phases.

## Table of Contents

1. [Workflow Topologies](#workflow-topologies)
2. [DAG Execution Patterns](#dag-execution-patterns)
3. [Mapping to Evolve-Loop](#mapping-to-evolve-loop)
4. [State-Delta Passing](#state-delta-passing)
5. [Checkpoint/Resume](#checkpointresume)
6. [Prior Art](#prior-art)
7. [Anti-Patterns](#anti-patterns)

---

## Workflow Topologies

| Topology | Description | Latency | Complexity | Failure Recovery |
|----------|-------------|---------|------------|-----------------|
| **Linear Pipeline** | Sequential stages; each stage feeds the next | High (sum of all stages) | Low | Restart from failed stage or beginning |
| **DAG (Directed Acyclic Graph)** | Stages form a graph with parallel branches and merge points | Low (longest path only) | Medium | Retry failed node; resume from checkpoint |
| **State Machine** | Nodes represent states; transitions depend on conditions and events | Variable (depends on path taken) | High | Re-enter last known state; replay transitions |
| **Hybrid (DAG + State Machine)** | DAG for main flow; state machine for error handling and retries | Low-Medium | High | Combines checkpoint resume with state-based retry |

### When to Use Each Topology

| Topology | Best For | Avoid When |
|----------|----------|------------|
| Linear Pipeline | Simple sequential tasks; prototyping | Stages are independent; latency matters |
| DAG | Parallel-capable workloads; build systems; agent orchestration | Every node depends on the previous one |
| State Machine | Long-running workflows with branching conditions | All paths are predictable at compile time |
| Hybrid | Production agent systems requiring both parallelism and resilience | Simplicity is a hard constraint |

---

## DAG Execution Patterns

| Pattern | Mechanism | Use Case | Key Constraint |
|---------|-----------|----------|----------------|
| **Parallel Fan-Out** | Dispatch N independent tasks simultaneously from a single node | Run Scout sub-agents across multiple gene categories in parallel | Bound concurrency to avoid resource exhaustion |
| **Fan-In (Barrier)** | Wait for all upstream nodes to complete before proceeding | Merge parallel Scout reports into a unified task list | Handle partial failures (fail-fast vs wait-all) |
| **Conditional Branching** | Evaluate a predicate at a node; route to one of N downstream paths | Skip Audit phase when build produces no code changes | Ensure all branches eventually converge or terminate |
| **Dependency Resolution** | Topological sort of the DAG; execute nodes whose dependencies are satisfied | Resolve file-level build order before launching parallel builds | Detect and reject circular dependencies at graph construction time |
| **Speculative Execution** | Start downstream node before upstream confirms success; discard on failure | Begin Auditor while Builder finalizes non-critical outputs | Limit speculative work to idempotent, low-cost operations |
| **Checkpoint/Resume** | Persist node outputs at defined points; resume from last checkpoint on failure | Recover a multi-cycle evolve-loop run after crash | Balance checkpoint frequency against I/O overhead |
| **Map-Reduce** | Fan-out identical task across partitioned input; reduce outputs into single result | Score multiple candidate implementations in parallel, select best | Ensure reduce function is associative and commutative |

### Fan-Out/Fan-In Diagram

```
         [Coordinator]
        /      |       \
   [Task-A] [Task-B] [Task-C]     ← fan-out (parallel)
        \      |       /
         [Aggregator]              ← fan-in (barrier)
```

---

## Mapping to Evolve-Loop

### Current Linear Pipeline

```
Scout → Builder → Auditor → Ship → Learn
```

| Phase | Input | Output | Parallelizable |
|-------|-------|--------|----------------|
| **Scout** | Gene pool, metrics, codebase | scout-report.md (ranked task list) | Yes (split by gene category) |
| **Builder** | scout-report.md | Code changes, build-report.md | Yes (independent tasks) |
| **Auditor** | Code changes, build-report.md | audit-report.md, pass/fail verdict | Partially (lint + test in parallel) |
| **Ship** | Audit pass, code changes | Git commit | No (atomic operation) |
| **Learn** | Cycle metrics, audit results | Updated genes, memory | No (sequential reflection) |

### Potential DAG Layout

```
                 [Scout]
                /       \
     [Scout-Genes]   [Scout-Metrics]     ← parallel sub-scouts
                \       /
              [Task Ranker]
             /      |      \
    [Build-A]  [Build-B]  [Build-C]      ← parallel builds
         |         |          |
   [Audit-A]  [Audit-B]  [Audit-C]      ← speculative audit per build
         \         |         /
          [Gate Aggregator]              ← fan-in: all audits must pass
                  |
               [Ship]
                  |
               [Learn]
```

### DAG Migration Path

| Step | Action | Risk | Mitigation |
|------|--------|------|------------|
| 1 | Parallelize Scout sub-tasks (gene scan + metric scan) | Low | Both are read-only; no shared state |
| 2 | Parallelize independent Builder tasks | Medium | Ensure builds target non-overlapping files |
| 3 | Enable speculative Auditor start during Builder finalization | Medium | Discard audit results if build changes after audit start |
| 4 | Add conditional branch to skip Learn on zero-change cycles | Low | Gate on commit existence |

---

## State-Delta Passing

### Principles

| Principle | Description |
|-----------|-------------|
| **Minimize payload** | Pass only what changed, not full state; use diffs and references |
| **Use handoff files** | Write structured artifacts (JSON/YAML) to a workspace directory; downstream reads on demand |
| **Schema-version handoffs** | Include a version field in every handoff file; reject incompatible versions |
| **Immutable artifacts** | Never mutate a handoff file after writing; create a new versioned file instead |

### Handoff File Convention

| Field | Type | Purpose |
|-------|------|---------|
| `version` | string | Schema version for forward compatibility |
| `source_node` | string | DAG node that produced this artifact |
| `timestamp` | ISO 8601 | Creation time |
| `delta` | object | Changed fields only (not full state) |
| `refs` | string[] | Paths to large artifacts (avoid inlining) |

### Example Handoff Structure

```
workspace/
  cycle-142/
    scout-report.md          ← Scout output
    scout-delta.json          ← Minimal state delta for Builder
    build-report.md           ← Builder output
    build-delta.json          ← Changed files list for Auditor
    audit-report.md           ← Auditor output
```

---

## Checkpoint/Resume

### Checkpoint Strategy

| Strategy | Granularity | Storage Cost | Resume Speed |
|----------|-------------|-------------|--------------|
| **Per-Phase** | Save after each major phase (Scout, Build, Audit) | Low | Restart from last completed phase |
| **Per-Node** | Save after every DAG node completes | Medium | Resume from exact failure point |
| **Per-Step** | Save after every sub-step within a node | High | Minimal rework on resume |

### Checkpoint Contents

| Component | Include | Exclude |
|-----------|---------|---------|
| DAG state | Node statuses, edge completion flags | Transient logs |
| Artifacts | Handoff files, reports | Intermediate scratch files |
| Environment | Config snapshot, gene pool version | Full model weights |
| Metrics | Cycle counters, timing data | Raw telemetry streams |

### Resume Protocol

| Step | Action |
|------|--------|
| 1 | Load latest checkpoint file from `workspace/checkpoints/` |
| 2 | Validate checkpoint integrity (schema version, required fields) |
| 3 | Identify incomplete nodes from DAG state |
| 4 | Re-execute only incomplete nodes and their downstream dependents |
| 5 | Merge resumed outputs with existing checkpoint artifacts |

---

## Prior Art

| System | Type | Key Concept | Relevance to Evolve-Loop |
|--------|------|-------------|--------------------------|
| **LangGraph** | Agent framework | Graph-based agent orchestration with cycles and persistence | Direct model for DAG-based agent pipelines; supports checkpointing |
| **CrewAI** | Agent framework | Role-based agents with task delegation and sequential/parallel flows | Validates multi-agent role pattern (Scout, Builder, Auditor) |
| **Prefect** | Workflow orchestrator | Task DAGs with automatic retry, caching, and state management | Production-grade checkpoint/resume and failure recovery patterns |
| **Temporal** | Workflow engine | Durable execution with event sourcing and replay | Long-running workflow resilience; state recovery after crashes |
| **Apache Airflow** | DAG scheduler | DAG definition as code; dependency resolution; backfill support | Mature DAG scheduling patterns; operator/sensor model |
| **Dagger** | CI/CD engine | Container-based DAG pipelines with content-addressed caching | Cache invalidation strategies for artifact-based pipelines |
| **Metaflow** | ML pipeline | Step-based DAGs with automatic data versioning and resume | Data artifact passing between DAG nodes; resume from failure |

### Key Takeaways from Prior Art

| Lesson | Source | Application |
|--------|--------|-------------|
| Define DAGs as code, not config | Airflow, Prefect | Express evolve-loop DAG in a script, not YAML |
| Content-address artifacts for cache hits | Dagger | Hash handoff files to skip unchanged nodes on resume |
| Separate orchestration from execution | Temporal | Keep DAG scheduler independent of agent implementation |
| Version all inter-node schemas | Metaflow | Prevent silent breakage when handoff format changes |

---

## Anti-Patterns

| Anti-Pattern | Description | Consequence | Mitigation |
|--------------|-------------|-------------|------------|
| **Over-Complex DAG** | Add nodes for every micro-step; DAG has 50+ nodes for a simple pipeline | Scheduling overhead exceeds task execution time; hard to debug | Collapse tightly-coupled steps into single nodes; limit DAG to 10-15 nodes |
| **Missing Error Edges** | DAG defines only happy-path edges; no explicit failure/retry transitions | Silent failures; stuck pipelines; orphaned downstream nodes | Define explicit error edges and timeout transitions for every node |
| **Checkpoint Bloat** | Checkpoint every sub-step; never prune old checkpoints | Disk fills up; resume becomes slow scanning large checkpoint directories | Checkpoint per-phase (not per-step); prune checkpoints older than N cycles |
| **Circular Dependencies** | Node A depends on Node B which depends on Node A (violates DAG constraint) | Deadlock; infinite loop; topological sort fails | Validate acyclicity at graph construction time; reject cycles with clear error |
| **God Aggregator** | Single fan-in node merges all outputs and runs all validation | Bottleneck; single point of failure; hard to parallelize | Split aggregation into typed sub-aggregators (lint-check, test-check, etc.) |
| **Implicit State Sharing** | Nodes communicate through shared mutable state instead of explicit handoffs | Race conditions; non-deterministic results; impossible to checkpoint | Use immutable handoff files; pass state deltas explicitly |
| **Speculative Waste** | Speculatively execute expensive downstream nodes that are frequently discarded | Wasted compute; increased cost; resource contention | Limit speculation to cheap, idempotent operations; track discard rate |
| **Monolithic Handoff** | Pass entire pipeline state between every node instead of minimal deltas | Token/memory bloat; slow serialization; unnecessary coupling | Pass only changed fields; reference large artifacts by path |
