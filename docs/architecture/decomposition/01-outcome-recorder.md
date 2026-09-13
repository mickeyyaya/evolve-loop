# Unit 01 — Phase-outcome recorder (`internal/core/outcome`)

- **Program:** [ADR-0103](../adr/0103-component-breakdown-program.md) · order 1 of design §12
- **Extracted from:** `go/internal/core/failure_learning.go` (the C1 recording chokepoint,
  ADR-0044) — `recordPhaseOutcome` (Orchestrator method), `contextFillFor`,
  `composePhaseTimings`, `writePhaseTimings`, `phaseUsageSidecar`
- **Module tag:** `outcome` · **Status:** landed 2026-09-13

## 1. Context

Every terminal disposition of a dispatched phase — a happy advance and each abort return —
funnels through one chokepoint so that the cycle result's `PhasesRun`, the `phase-timing.json`
log and the `<phase>-usage.json` sidecar always reflect what actually ran (cycle 262 is the
incident that made the chokepoint mandatory). The chokepoint also stamps the end-of-dispatch
clock, the phase archetype and the context-window fill, and raises the Signal Center's
`phase.outcome` event. It lived as a 100-line `Orchestrator` method beside the failure-learning
machinery, with six hand-written `[orchestrator] WARN …` lines for its own failure modes and no
way to test the recording without a whole orchestrator.

## 2. Boundary

| Moves to `outcome` | Stays in `core` | Why |
|---|---|---|
| the in-memory record (`PhasesRun`, the timing entry), the clock/archetype/fill stamping | `phaseOutcomeFrom` (projects `core.PhaseResponse`, a core type) | the projection depends on core's response type; the record depends only on leaf types |
| the usage sidecar (`UsageSidecar`, its file) | the ADR-0048 shadow grade of the abort reason | graduated enforcement is orchestrator policy, not the record |
| the phase-timing log (compose + atomic write) | the failure-diag sidecar and `DeliveryFailureCause` | they need the core sentinel `ErrArtifactTimeout` — unit 02 |
| the context fill (`contextFillFor`, derived in `Record` alone) | the `phase.outcome` **emission** (module `orchestrator`) | the producer keeps its module; the recorder calls it through a hook at the same point |

The unit imports only leaf packages (`cyclestate`, `phasetiming`, `recovery`, `contextfill`,
`signalcenter`); `core` imports the unit. No import cycle, no core type in the API.

## 3. API

```go
package outcome

type Recorder struct{ /* now, archetype, emit, signals */ }
type Option func(*Recorder)

func NewRecorder(now func() time.Time, archetype func(phase string) string,
    emit func(cycle int, out recovery.PhaseOutcome), opts ...Option) *Recorder
func WithSignals(c func() *signalcenter.Center) Option   // read live at every use; nil = Null Object (tests only)
func (r *Recorder) SignalsWired() bool

// Record is the chokepoint: stamps EndedAt/Archetype/fill, appends to result.PhasesRun and
// *timings, calls emit, then writes <phase>-usage.json (skipped, with a signal, when the
// workspace is empty).
func (r *Recorder) Record(result *cyclestate.CycleResult, timings *[]phasetiming.Entry,
    workspace string, out recovery.PhaseOutcome)

// WritePhaseTimings persists phase-timing.json once per cycle with append-merge semantics and
// returns the composed set every consumer must project.
func (r *Recorder) WritePhaseTimings(workspace string, live []phasetiming.Entry) []phasetiming.Entry
func ComposePhaseTimings(workspace string, live []phasetiming.Entry) []phasetiming.Entry
func ContextFillFor(out recovery.PhaseOutcome) (ratio float64, hot bool)

type UsageSidecar struct{ … } // the declared on-disk contract of <phase>-usage.json
func UsageSidecarPath(workspace, phase string) string
```

The orchestrator keeps the seam every caller uses — `(*Orchestrator).recordPhaseOutcome(result,
timings, workspace, out)` — as a one-call facade plus the shadow grade; `cycleRun.flushPhaseTimings`
calls `WritePhaseTimings` once per cycle and caches the composed set the dossier writers project.

## 4. Patterns

| Pattern | Force it answers |
|---|---|
| **Chokepoint object** (`Recorder`) | one owner of the end-of-dispatch stamps; call sites cannot drift on clock, taxonomy or fill |
| **Explicit DI at construction** (`NewRecorder(now, archetype, emit)`, functional `Option`) | the clock and the archetype lookup are the orchestrator's (the catalog changes mid-cycle, so the lookup is a live closure, never a snapshot); the emission stays the orchestrator's producer |
| **Hook / Strategy** (`emit`) | the `phase.outcome` event is raised at the same instant as before, by the same module, without the recorder importing core |
| **Null Object** (`WithSignals(nil)`) | tests build a recorder without a Center; every production root that records an outcome wires one (the `--simulate` root builds the production topology too) |
| **Late-bound collaborators** (closure clock, method-value archetype, Center accessor) | the orchestrator's clock and Center are options tests apply after construction; a snapshot would silently report into nothing (architecture review MEDIUM-1) |
| **Strangler Fig facade** | ~30 call sites of `recordPhaseOutcome` stay untouched |

## 5. Module contract

| Item | Value |
|---|---|
| Module | `outcome` (closed set, design §5.1) |
| `OUTCOME_SIDECAR_SKIPPED` WARN | the workspace was empty: the in-memory record stands, no `<phase>-usage.json` (the CWD-relative leak guard) |
| `OUTCOME_SIDECAR_WRITE_FAILED` WARN | `<phase>-usage.json` could not be encoded or written; the reason names the step (marshal, write) and the error, fields name the phase and path; nothing is written on an encode failure |
| `OUTCOME_TIMING_SKIPPED` WARN | the workspace was empty: `phase-timing.json` not written |
| `OUTCOME_TIMING_WRITE_FAILED` WARN | `phase-timing.json` marshal, temp write or rename failed; the reason names the step and the error; the log on disk is left untouched |
| Origin | `Recorder.Record` / `Recorder.WritePhaseTimings` |
| Kind | `phase.outcome` / `phase.aborted` stay the orchestrator's events; the unit's four conditions ride one kind of its own, **`outcome.warning`**, added to the closed kind set (the bridge's `bridge.warning` is the precedent: a module's operational warnings under its own kind) |
| Removed lines | the six `[orchestrator] WARN …` stderr lines of the chokepoint and the timing writer; the two `marshal` branches are kept and covered — a non-finite float is the one encode failure a plain struct has, the pre-extraction code guarded it, and writing the nil result would have truncated the record silently (Go review) |

## 6. Tests (red first)

1. `TestRecorder_Record_AppendsThePhaseAndTheTimingEntry` — order into `PhasesRun` and `*timings`; EndedAt from the injected clock; Archetype from the injected lookup; fill ratio and hot flag.
2. `TestRecorder_Record_CallsEmitOnceAfterTheInMemoryRecord` — the hook sees the stamped outcome and the cycle.
3. `TestRecorder_Record_EmptyWorkspaceKeepsTheMemoryRecordAndSignalsSkipped` — `OUTCOME_SIDECAR_SKIPPED`, no file.
4. `TestRecorder_Record_WritesTheUsageSidecarByteIdentical` — the file equals the pre-extraction rendering (indent-2 JSON, field order, omitempty).
5. `TestRecorder_Record_SidecarWriteFailureIsAWarnSignal` — `OUTCOME_SIDECAR_WRITE_FAILED`.
6. `TestRecorder_WritePhaseTimings_AppendMergesAndWritesAtomically` — disk entries first, then live; temp+rename; the composed set returned.
7. `TestRecorder_WritePhaseTimings_EmptyWorkspaceSignalsSkipped`, `…RenameFailureSignals` (a directory at the target path).
8. `TestComposePhaseTimings_DiskFirstThenLive_GarbageFallsBackToLive`.
9. `TestContextFillFor_UnknownTierIsAbsent`.
10. `TestWithSignals_NilIsTheNullObject`, `TestOutcomeCodes_AreRegisteredWithDocs`, `TestModule_ClosedSetHasOutcome`.
11. `TestRecorder_Record_MarshalFailureIsAWarnSignalAndWritesNothing`, `TestRecorder_WritePhaseTimings_MarshalFailureSignalsAndLeavesTheLogUntouched` — a NaN/Inf float is `OUTCOME_*_WRITE_FAILED` naming the step; nothing is written.
12. `TestWithSignals_ReadsTheCenterLive` (unit) and `TestRecorder_SeesASignalCenterAppliedAfterConstruction` (core) — the accessor, not a snapshot.
13. `TestWireSimulateOrchestrator_SignalCenterWired` (cmd) — the `--simulate` root is wired; `TestNilSignalCenterRootsArePinned` keeps ONE pinned nil root, `internal/routingtest` (test machinery taking a `*testing.T`).
14. Core keeps its ten chokepoint tests unchanged through the facade; `TestRecordPhaseOutcome_…` proofs still pass.

Coverage bar: 100 % lines (`.cover-strict`), every export named (apicover). Mutants: emit dropped, EndedAt not stamped, PhasesRun not appended, sidecar written on an empty workspace, compose order reversed, rename skipped, a skipped-sidecar signal not raised.

## 7. Why it is better than the original

| Dimension | Before | After |
|---|---|---|
| Observability | six prose lines, no code, tagged `[orchestrator]` | four registered codes under module `outcome`, origin `Recorder.<method>`, on the Signal Center stream and the console at WARN |
| Isolation | recording testable only through a full `Orchestrator` (10 test files drive it indirectly) | a 150-line package with an injected clock, archetype and hook; 100 % covered directly |
| Changeability | the stamps, the sidecar shape and the timing composition interleaved with failure-learning code in a 1068-line file | one type owns the record; the on-disk contract is an exported struct with its path function |
| Measurement | none per unit | line + API coverage gates and signal counts by code, per unit |
| Callers | 30 call sites of the chokepoint | unchanged (facade) |

## 8. Risks and byte-identity guarantees

- `PhasesRun` and `*timings` are mutated in place through pointers exactly as before (test 1).
- `<phase>-usage.json` and `phase-timing.json` bytes are identical (tests 4 and 6; the dossier
  producer and `evolve cycle timing` read them).
- The `emit` hook fires at the same point (after the in-memory record, before the sidecar).
- The archetype closure must read the orchestrator's catalog live (a snapshot at construction
  would miss mid-cycle mints) — the facade passes `o.phaseArchetype`, a method value.
- The recorder is constructed after the orchestrator's options are applied, and reads the clock
  and the Center live, so an option applied later (as `resume_lifecycle_test.go` applies
  `WithSignalCenter`) is in effect too.
- The six deleted stderr lines were unconditional; the signals are Center-conditional, so every
  root that records an outcome must wire a Center (architecture review HIGH-1): the `--simulate`
  root now builds the production topology (`newRootSignalCenter`), and the one remaining
  nil-Center root is the test-only `internal/routingtest` engine.

## 9. Migration

`failure_learning.go` loses the chokepoint body, the sidecar type and the timing writers;
`recordPhaseOutcome` becomes `o.outcome.Record(...)` plus the shadow grade; `flushPhaseTimings`
and the dossier call sites keep projecting the set `flushPhaseTimings` caches; four core tests that
called the old package-level helpers now call the unit. The single-writer guard scans the WHOLE
module for `.WritePhaseTimings(` (the writer is exported now, so visibility no longer makes "one
writer" structurally true — the guard does). No call site of `recordPhaseOutcome` changes.

Two details found while landing: (a) thirty-three core tests assemble an `Orchestrator` as a
literal (no constructor) and one composition test flushes a `cycleRun` with no orchestrator at all,
so the facade reaches the recorder through `(*Orchestrator).recorder()` — `NewOrchestrator` builds
the wired recorder eagerly, a literal orchestrator gets the Null-Object recorder on first use, and
a nil orchestrator gets a bare one; every path still has ONE writer, which the single-writer guard
now pins on `outcome.WritePhaseTimings`. (b) The recorder reads the orchestrator's clock through a
closure because those tests swap `o.now` after construction.

## 10. Verification (landed)

Red first (the package compiled against its tests before it existed); the unit and the Signal
Center at 100 % lines (`cover-strict`) and 100 % API (`apicover`); the facade and its accessor at
100 %. Fifteen build-confirmed mutants killed by name: emit dropped, EndedAt not stamped, PhasesRun
not appended, sidecar written on an empty workspace, compose order reversed, rename skipped, the
skipped-sidecar signal not raised, archetype not stamped, the facade dropping the record, the flush
not composing; after the review folds: the sidecar encode failure falling through to a nil write, the
timing encode failure falling through, the Center snapshotted at construction instead of read live,
the `--simulate` root without a Center, the `--simulate` ledger unobserved.

Review folds (fleet: code-simplifier → architecture-reviewer ∥ go-reviewer; both Warning, no
CRITICAL): the duplicated recorder construction folded into `wiredRecorder()`; HIGH-1 — the six
deleted stderr lines were unconditional and the signals are Center-conditional, so the `--simulate`
root now builds the production topology and the nil-Center pin keeps one test-only root; MEDIUM-1 —
the Center is read through an accessor like the clock; MEDIUM-2 — `composePhaseTimings` and
`contextFillFor` unexported (no consumer; the fill stays derived in `Record` alone); Go review —
the encode failures are signalled and write nothing, and the single-writer guard scans the whole
module because the writer is exported now.
