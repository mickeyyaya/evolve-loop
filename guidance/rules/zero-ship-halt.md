---
id: zero-ship-halt
statement: "Stop after two consecutive cycles that ship nothing, and find the root cause in the pipeline."
layer: L0
order: 16
audience: [all]
enforced_by: []
source: "the instructions digest of AGENTS.md (X10); memory feedback_zero_ship_halt_and_gate_red_protocol; docs/operations/operating-policy.md §4.1"
status: active
added: 2026-10-26
evidence: "the operator rule of 2026-08-10 (memory feedback_zero_ship_halt_and_gate_red_protocol); docs/operations/operating-policy.md §4.1"
---
Why: Two empty cycles point to a pipeline defect, not to a task. More cycles repeat the defect.
