# internal/dag

## Purpose

`internal/dag` levels a dependency graph with Kahn's algorithm. It is the one home of that topological logic. The campaign wave planner needs concurrent waves from it. The swarm merge-train ordering needs the flattened serial order.

## Design

- `Levels` puts every node without dependencies at level 0 and a node at level k when all its dependencies resolve in levels below k. Nodes in a level are independent and may run concurrently. `Flatten` concatenates the levels into one serial order.
- A node with no dependencies still appears, because `Levels` seeds every known node with an in-degree of zero.

## Invariants

- **Deterministic.** Order inside a level is ascending, so the result is stable across runs.
- **Four errors, because each makes a leveling impossible.** They are a dependency on an unknown node, a key that is not a node, a self-dependency, and a cycle.
