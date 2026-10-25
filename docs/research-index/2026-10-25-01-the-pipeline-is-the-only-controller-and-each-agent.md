## 2026-10-25 — the pipeline is the only controller, and each agent is an isolated, replaceable worker (ADR-0154), and the loop heals itself inside its boundaries (ADR-0155)

| Document | What it records |
|---|---|
| [research/llm-isolation-2026-10.md](research/llm-isolation-2026-10.md) | Research: the gaps G1 to G11 with re-verified file:line evidence. Current practice for hermetic environments, agent sandboxes, mediated communication, OS exec rules, structured outputs and instruction files. |
| [plans/llm-isolation-2026-10.md](plans/llm-isolation-2026-10.md) | Plan: five designs with scores, the decisions D1 to D20 and the components X1 to X16. |
| [architecture/adr/0154-the-pipeline-is-the-only-controller-and-each-agent-is-an-isolated-replaceable-worker.md](architecture/adr/0154-the-pipeline-is-the-only-controller-and-each-agent-is-an-isolated-replaceable-worker.md) | Decision: the invariants I-1 to I-9, with lazy access to earlier deliverables and to the lessons of the item lineage. |
| [architecture/adr/0155-the-loop-heals-itself-inside-its-boundaries-an-agent-proposes-and-the-pipeline-disposes.md](architecture/adr/0155-the-loop-heals-itself-inside-its-boundaries-an-agent-proposes-and-the-pipeline-disposes.md) | Decision: the invariants I-10 to I-14. An orchestrator agent proposes from a closed catalog, and the pipeline disposes. |
| [architecture/llm-isolation.md](architecture/llm-isolation.md) | Spec of record: the agent cell, the artifact index and the sandbox rules. Also the routing data, the manifest fields, the conformance matrix, the signals and the runbook. Part II: self-healing within boundaries. |
