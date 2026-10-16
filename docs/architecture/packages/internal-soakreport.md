# internal/soakreport

> The decisions it serves: [ADR-0044](../adr/0044-unified-phase-recovery-protocol.md) and [ADR-0045](../adr/0045-corrective-interaction-protocol.md). Its command: `evolve soak-report` (`cmd/evolve/cmd_soak_report.go`).

## Purpose

`internal/soakreport` is the read-only soak evidence reporter. It is the instrument that the `EVOLVE_PHASE_RECOVERY` enforce flip is gated on. Every ADR-0044 and ADR-0045 component already leaves durable shadow records. The reporter aggregates them per batch and renders one evidence table per component against the evidence bars of the plan (section 6).

## Design

- **Where the evidence lives.** Interaction ledgers (`*-interactions.ndjson`) carry the `fatal_pane_shadow` (C2), `salvage` (I2), `kernel_answer` (I3) and `rule_shadow_fire` (I4) records. The phase observer's events (`*-observer-events.ndjson`) carry the C4/C3 incident envelopes, with the action and the reason that the policy decided. `cyclehealth.ClassifyOutcome` gives the per-cycle outcome (R6).
- **`bars` is the one copy of the evidence-bar text**, in the words of the plan. The renderer prints it verbatim.
- **`componentOrder` fixes the order** of the sections (C2, C4/C3, I2, I3, I4), so two soak reports diff cleanly.
- **Only INCIDENT-severity observer lines count.** `collectObserverIncidents` skips every other severity.
- **`Render` sorts the counters** of each component by key.

## Invariants

- **The reporter never mutates anything.** The sweep (`cmd_loop`) and the flip (the operator) act. The reporter only accounts. No file in the package calls a writing function of `os`. Pinned by `TestSoakReport_NeverMutatesTheFilesystem` and `TestCollect_LeavesTheWorkspaceUntouched`.
- **Missing evidence is absent evidence, never an error.** `Collect` has no error result. An unreadable workspace adds a note, and the report must render for any historical batch. Pinned by `TestCollect_EmptyWorkspacesDegradeToNotes` and `TestReport_RenderFromConstructedRows`.
- **The report shape is stable.** Every component appears, with zero counts when it has no evidence, so soak comparisons never diff against a missing section. Pinned by `TestCollect_EmptyWorkspacesDegradeToNotes` and `TestRender_TableCarriesBarsAndCounts`.
- **Only incidents count as C4/C3 evidence.** Pinned by `TestCollect_AggregatesAllComponentEvidence`, where an INFO line is excluded.
- **A repeated note is written once.** Cycles with an unreadable workspace do not repeat the note. Pinned by `TestAppendOnce_DedupesNotes`.

## Findings

- None open.
