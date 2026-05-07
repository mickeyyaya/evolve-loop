---
name: reference
description: Reference doc.
---

> Read this file during Phase 1 scan. Module graph analysis for circular dependencies, boundary validation, fan-in/fan-out centrality, and orphan detection.

# Architecture Analysis

Detect structural problems in the module dependency graph during Phase 1 scan.

## Circular Dependency Detection

Use depth-first search on the module import graph:

1. Build directed graph: each file is a node, each import is an edge
2. Run DFS, track visited and in-stack nodes
3. When a back-edge is found (visiting a node already in the stack), record the cycle
4. Report all cycles with the full path

| Cycle length | Severity | Action |
|--------------|----------|--------|
| 2 (A↔B) | Critical | Break immediately — extract shared interface |
| 3-4 | High | Refactor — introduce mediator or event bus |
| 5+ | High | Architectural redesign needed |

## Architecture Boundary Validation

Define allowed dependency directions using configurable path rules:

| Source pattern | May depend on | Must NOT depend on |
|----------------|--------------|-------------------|
| src/ui/** | src/services/**, src/types/** | src/db/**, src/infra/** |
| src/services/** | src/db/**, src/types/** | src/ui/** |
| src/db/** | src/types/** | src/ui/**, src/services/** |

Flag any import that violates the boundary rules as an architecture smell.

## Fan-in / Fan-out Centrality Analysis

Compute centrality metrics for each module to identify high-risk refactoring targets:

| Metric | Definition | Significance |
|--------|-----------|--------------|
| Fan-in | Number of modules that import this module | High fan-in = many dependents, high-risk changes |
| Fan-out | Number of modules this module imports | High fan-out = high coupling, smell indicator |
| Instability | Fan-out / (Fan-in + Fan-out) | 0 = stable (many dependents), 1 = unstable (many dependencies) |

**Prioritization rule:** Modules with high fan-in (>10) should be refactored with extreme care. Modules with high fan-out (>10) are prime candidates for decomposition.

## Orphan Module Detection

Identify modules with zero fan-in (no other module imports them) that are not entry points:

1. Build the full import graph
2. Find all nodes with fan-in = 0
3. Exclude known entry points (main, index, test files, config)
4. Remaining nodes are orphan candidates — likely dead code
