# internal/phaseoutputs

## Purpose

`internal/phaseoutputs` accounts, per cycle and per completed phase, for the data a reviewer needs. That data is the report, the dispatched prompt, the event stream and the usage record. It reports the gaps. It also decides if a cycle's reasoning chain is compliant, and if that cycle is abnormal enough to signal.

## Design

- **Pure core, one reader.** `Survey`, `CycleChainStatus` and `Signal` take what the caller read and return rows, statuses and lines. `io.go` is the only workspace reader. The CLI (`evolve cycle outputs`) and the post-cycle signal emitter share it, so they read the same files and classify a read the same way.
- **Three-outcome shadow read.** A read of the shadow record is absent, corrupt or parsed. A corrupt record is a recorder defect, not a missing one, so it is never folded into absent.
- **Typed chain states.** `ChainStatus` is an enum with six values: audit not run, record missing, record corrupt, absent, present and inconsistent. Absent means the auditor ran and did not comply. Inconsistent means a record exists with no audit run. Only states whose audit ran are in the compliance denominator. `InDenominator` lives beside the enum so no call site re-derives it.
- **Narrow wire view.** `ShadowView` re-declares the `chain_present` JSON tag instead of importing the auditchain package. A test marshals the real producer type and decodes it through the view, so a rename upstream fails a test.
- **Native phases.** `nativePhases` lists phases that complete in Go with no agent dispatch. They owe no prompt and no events, and they always owe usage. Ship is the only entry, cited from its live cycles. Adding a phase needs a live citation and a deliberate change to its pin test.
- **Naming fallback.** Report names come from the contract resolver. A nil resolver degrades to the builtin map. A builtin entry for a spec-derived phase is forbidden, because it will shadow the spec and drop its required sections. Prompt and events are looked up by phase name, then by agent name. Retro is the case where the report and usage follow the phase but the prompt follows the agent.
- **Owed and satisfied.** A phase owes an artifact or not. An owed artifact is satisfied only when it has content. A zero-byte file is not data. `NotOwed` is set on the row, so a consumer reading rows cannot re-derive a gap that the summary does not show.
- **Duplicates.** A phase that a run re-dispatched appears once per completion in `completed_phases`. `Survey` collapses those to one row, so the denominator does not count the same files twice.

## Invariants

- Absent, corrupt and parsed shadow records never merge. Pinned by `chainstatus_test.go`.
- The compliance denominator holds only cycles whose audit ran. Pinned by the `InDenominator` cases in `chainstatus_test.go`.
- The `chain_present` wire tag matches the producer. Pinned by `TestShadowView_DecodesTheAuditchainWireTag`.
- A missing events stream for retro is a reported gap, not a naming artifact. Pinned by the retro case in `phaseoutputs_test.go`.
- The native register grows only with a cited live cycle. Pinned by the native-phase test in `phaseoutputs_test.go`.
- The wire names of `Row`, `Artifact` and `Result` are part of the CLI JSON contract. Pinned by `TestRowArtifactResult_AreTheWireShape`.
