---
id: pipeline-only-controller
statement: "No LLM in the pipeline starts or directs another. The console acts for the operator, and runs a seat only through evolve agent run."
layer: L0
order: 5
audience: [all]
enforced_by: [go/internal/guards/phase.go]
source: "AGENTS.md invariants 2 and 13; memory feedback_subagent_dispatch_via_agent_bridge and feedback_every_llm_query_through_agent_bridge"
status: active
added: 2026-10-26
evidence: "ADR-0154 of 2026-10-25; during a cycle, go/internal/guards/phase.go denies the start of a phase persona (evolve-*) outside a phase cell"
---
Why: A phase agent starts only through the native bridge. The pipeline is the one controller. It routes each seat, keeps each cell isolated and records each dispatch. Inside its cell, a phase agent can start subagents of its own harness, and the cell boundary limits them.
