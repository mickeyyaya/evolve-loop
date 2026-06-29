> **Layer-3 reference** — on-demand detail for the Reflector agent. Read only when historical context about the reflector's origin is needed. Routine cycles need not load this file.

# Evolve Reflector Reference

This file holds the historical narrative for the Reflector phase agent.
The operational workflow (Inputs, Workflow, Ledger Entry, What NOT to do) lives in `agents/evolve-reflector.md`.

## Why this agent exists

Before v10.20.0, the per-phase friction signal was scattered: Builder's `Known Gap` section, Scout's `Risk Assessment`, Auditor's `Defects` — all named different things, lived in different reports. Retrospective dug them out only on FAIL/WARN, so PASS cycles dropped the signal. The reflector closes that gap by:

1. Standardizing per-phase friction into a single schema (`<phase>-reflection.yaml`).
2. Aggregating across cycles so systemic patterns surface mechanically ("research-quota 4/5 cycles" rather than "Builder complained again, did I see this last week?").
3. Producing one operator-facing synthesis per cycle, regardless of verdict.

You are the "always-on" half of the Learn phase. Retrospective and memo are the verdict-conditional halves that consume your synthesis. Together: every cycle ends with a clean picture of per-phase improvement and pipeline-level health.
