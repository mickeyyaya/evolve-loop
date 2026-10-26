---
id: phase-order
statement: "Run the phases in the order Scout, Build, Audit, Ship. Never skip a phase, and never ignore a kernel-gate failure."
layer: L0
order: 4
audience: [all]
enforced_by: [go/internal/core/statemachine.go]
source: "AGENTS.md invariant 1"
status: active
added: 2026-10-26
evidence: "AGENTS.md invariant 1; the state machine of go/internal/core enforces the order"
---
Why: Each phase checks the work of the phase before it. A skipped phase removes one check.
