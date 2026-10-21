## 2026-10-20 — a test-writing phase that states FAIL on its own work is retracted (ADR-0149)

| Document | What it records |
|---|---|
| [architecture/adr/0149-a-test-writing-phase-that-states-fail-on-its-own-work-is-retracted-to-its-dispatch-tree.md](architecture/adr/0149-a-test-writing-phase-that-states-fail-on-its-own-work-is-retracted-to-its-dispatch-tree.md) | Decision (Proposed): on a stated FAIL, the runner restores every path the phase wrote and keeps its copies. Three options are scored, with the argument that a builder red cannot leave through this path. |
