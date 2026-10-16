# internal/cyclehealth

## Purpose

`internal/cyclehealth` does two jobs. First, `Check` runs an integrity check on a completed cycle's workspace and writes `cycle-health.json`. The orchestrator and Scout read that file before the next phase. A fatal anomaly halts the cycle. Second, `ClassifyOutcome` gives every cycle one ending from a fixed taxonomy. 

It is the measurement instrument of the `EVOLVE_PHASE_RECOVERY` soak. Every cycle must deliver a result, and that claim is auditable only when every ending is classified.

## Design

- `Check` runs the signals in a fixed order. Each signal returns the anomalies it found, possibly none. A signal never errors. A missing file or a parse failure is itself an anomaly.
- `Report` is the public JSON contract for Scout and the orchestrator. `Check` returns it if anomalies exist. The caller decides to halt from `OverallFatal`.
- The signals are listed below. Each one checks one property of the workspace.
  Ledger completeness, ledger timestamps and the hash chain are ledger checks. Artifact checksums, substance and workspace artifacts check the reports. Challenge tokens, velocity, canary files, duplicate ledger entries, phase latency, self-heal events and dossier commitment check the run.
- Two severities exist. `warn` is advisory, and the loop continues. `fatal` halts the cycle.
- `ClassifyOutcome` reads the two C1 records of a cycle workspace: `phase-timing.json` and `interaction-summary.json`. It needs no new instrumentation and works on any historical run directory.
- The required artifacts and the required ledger roles come from the phase-contract registry. The ledger and red-team checks read the same accessor, so the lists cannot drift from the contracts.

## Invariants

- Outcome taxonomy: SHIPPED is any ship dispatch with verdict PASS. DEFERRED is a quota exhaustion. FAILED_EXPLAINED is an abort reason, or a recorded phase verdict of FAIL. SALVAGED means the salvage rung ran and an artifact appeared. FAILED_UNEXPLAINED is every other case.
- SHIPPED scans every timing entry, not one. The timing file is an append-merge log. A failed try followed by a repaired PASS is the reality, and the PASS wins.
- DEFERRED is checked before the generic explained-failure branch. A quota defer must never be reported as a failed cycle.
- FAILED_EXPLAINED also covers an audit FAIL. The audit-FAIL, retro, end path is a normal completion with no abort reason, so the recorded verdict is the explanation. The scan runs in timing order, so the detail names the first failing phase, which is the causal one.
- An unreadable record degrades toward FAILED_UNEXPLAINED, because an unreadable record explains nothing. A present but unparseable record still marks the cycle as having run phases.
- SALVAGED is a conjunction heuristic. The v1 rollup has independent counters and no rung-by-result join. The salvage rung and the artifact are therefore tested separately. This is good enough for the soak instrument. Tighten it when the rollup gains a joined counter.
- The C1 record is read through `phasetiming.Entry`, its one schema home. A hand-typed mirror once parsed a renamed key to nil, reverted the detail to verdict-only, and kept every test green.
- A `stop_review` with action `pause` is an anomaly. A reviewer's `extend` means progress, so it emits nothing.
- A phase longer than 30 minutes between consecutive timing entries is a latency warning. The phase is likely stuck.
- The ledger may sit in the workspace or at the project root. The workspace path is checked first, so cycle-scoped tests keep their own ledger.

## Findings

- The batch targets are: SHIPPED at 60% or more, the five endings summing to 100%, and FAILED_UNEXPLAINED at zero. A non-zero unexplained count means a terminal path escaped the C1 chokepoint. File it as a defect.
- `abortReasonDeferredPrefix` mirrors the core's all-families-exhausted reason. The two packages cannot import each other, so the C1 JSON record is the contract between them.
