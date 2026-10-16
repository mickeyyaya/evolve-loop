# Comment history: `internal/phasetiming`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d3

### `go/internal/phasetiming/apicover_named_test.go:3` — above `import (`

```text
// apicover_named_test.go — ADR-0050 Phase 5 public-API coverage: name and
// exercise every exported phasetiming symbol by identifier (apicover counts
// field access as "uses", not "names"). Each test asserts a REAL contract.
```

### `go/internal/phasetiming/diagnostics_test.go:13` — above `func TestEntry_DiagnosticsRideTheRecordAndAreOmittedWhenAbsent(t *testing.T) {`

```text
// Diagnostics is the phase's OWN structured notes (severity + message), carried
// on the C1 record so a phase that returns FAIL by its own Classify (triage's
// protected-surface admission rejection, cycles 1634/1636) seals with the
// reason it gave, not with a synthesized "phase-infra class" marker. omitempty:
// a legacy log without the key parses to nil — absent, never fabricated.
```

### `go/internal/phasetiming/phasetiming.go:25` — above `type Entry struct {`

```text
// Entry is one phase dispatch's terminal latency + outcome record. The JSON
// tags are the on-disk contract (ADR-0044 C1); core's recordPhaseOutcome is the
// sole writer. StartedAt/EndedAt are the wall-clock anchors; Archetype is the
// composition class (plan/build/evaluate/control) the Rollup buckets by.
```

### `go/internal/phasetiming/phasetiming.go:47` — above `ModelSource   string 'json:"model_source,omitempty"'`

```text
// ModelSource + ResolvedModel (T3, cycle-463) mirror core.PhaseResponse's
// per-phase model provenance into the timing log, so the dossier (which
// ingests this file, not the live PhaseResponse) can project WHICH
// resolution path won — "profile"|"pin"|"advisor" — plus the concrete
// resolved model/tier. Both empty on a legacy log written before this
// change; that degrades to "absent", never a fabricated claim.
```

### `go/internal/phasetiming/phasetiming.go:62` — above `ContextFillRatio float64 'json:"context_fill_ratio,omitempty"'`

```text
// ContextFillRatio/ContextWindowHot (cycle-1271) are the derived twin of
// Tokens: how full the model's context window got, from the counts above
// over contextfill.WindowSizeForTier(ResolvedModel). Both omitempty, so an
// entry whose tier was not resolvable emits NEITHER key — a reader can tell
// "unknown fill" from a genuine 0.0. A legacy log written before this change
// parses to the zero value: absent, never a fabricated ratio.
```

### `go/internal/phasetiming/phasetiming.go:70` — above `Diagnostics []cyclestate.Diagnostic 'json:"diagnostics,omitempty"'`

```text
// Diagnostics (cycles 1634/1636) are the phase's OWN structured notes —
// severity + message, exactly as its PhaseResponse reported them — carried on
// the C1 record so a phase that returns FAIL by its own Classify (triage's
// protected-surface admission rejection) seals with the reason it gave. The
// seal (core.backfillFailReasons) and cyclehealth read the error-severity
// ones; warnings ride along as the durable trail the runner already meant to
// leave (reconciliation, ACS-floor overrides). omitempty: a legacy log
// without the key parses to nil — absent, never fabricated.
```

### `go/internal/phasetiming/phasetiming.go:119` — above `HotPhaseCount int      'json:"hot_phase_count"'`

```text
// Hot-phase rollup (cycle-1271): the cycle-level twin of the per-entry
// context-fill fields — how many phases ran at/above contextfill.HotThreshold
// and which ones, so "the build was starved for room" is answerable from the
// durable record. Derived through contextfill.IsHot rather than a re-declared
// comparison, so the INCLUSIVE boundary has exactly one definition. An entry
// with no resolvable tier (ratio absent ⇒ 0) is never reported hot.
```

### `go/internal/phasetiming/phasetiming_contextfill_test.go:1` — above `package phasetiming`

```text
// phasetiming_contextfill_test.go — cycle-1271 RED tests for the contextfill
// telemetry wiring (tasks wire-contextfill-into-phasetiming-entry and
// contextfill-rollup-hot-phase-summary).
//
// Entry gains ContextFillRatio/ContextWindowHot — the durable per-phase record
// of how full the model's context window got — and Summary gains the cycle-level
// twin (HotPhaseCount/HotPhases), exactly mirroring the Tokens (S4) / TotalTokens
// (S6) precedent already in this file's package.
//
// Legacy compat is the same contract the Tokens field documents: a timing log
// written before these fields existed must still parse, leaving both at their
// zero value — "absent", never a fabricated ratio, never an error.
```

### `go/internal/phasetiming/phasetiming_contextfill_test.go:23` — above `func TestEntry_ContextFillFieldsRoundTripJSON(t *testing.T) {`

```text
// TestEntry_ContextFillFieldsRoundTripJSON pins the on-disk contract (ADR-0044
// C1): the two new fields marshal under the snake_case keys the dossier and the
// `evolve cycle timing` CLI will read, and survive a marshal→unmarshal round
// trip with their values intact.
```
