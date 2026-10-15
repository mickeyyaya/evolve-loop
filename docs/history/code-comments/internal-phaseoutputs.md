# Comment history: `internal/phaseoutputs`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero h2

### `go/internal/phaseoutputs/phaseoutputs.go:56` — above `var nativePhases = map[string]bool{"ship": true}`

```text
// nativePhases lists phases that complete natively in Go with no agent
// dispatch: they owe no prompt and no events (nothing was dispatched to
// stream), and their no-report fact comes from the registry's NoArtifact
// contract. They ALWAYS owe usage — the C1 chokepoint writes a sidecar for
// every recorded phase, native included, so a missing one is a real recording
// failure. Admission requires a cited live cycle: ship enters on cycles
// 1452/1453 (in completed_phases with ship-usage.json and nothing else; the
// earlier whole-row exemption guesses — inherited-defect-reconcile,
// coverage-gate — were refuted by cycles 1432/1442 and stay OUT).
```

### `go/internal/phaseoutputs/phaseoutputs.go:67` — above `func Survey(completedPhases []string, listing map[string]int64, resolver phasecontract.Resolver) Result {`

```text
// Survey accounts for every completed phase against the workspace listing
// (name → size in bytes). Duplicate completed_phases entries (a re-dispatched
// phase records once per completion — cycle-1452 carried audit twice) collapse
// to one row, or the completeness denominator counts the same files twice.
//
// resolver is the SAME contract-resolution vocabulary the contract gate and
// bridge use — callers pass a catalog-aware phasecontract.CatalogResolver so
// spec-derived phases (memo, minted phases) resolve their real report names.
// Cycle-1452's false `memo-report.md missing` gap came from resolving through
// the compiled map alone; the tempting alternative — adding a builtin memo
// entry — was caught in review as WORSE: builtins shadow spec-derived
// contracts, so it would have silently dropped memo's required sections from
// the live gate. A nil resolver degrades to builtin-only.
```

### `go/internal/phaseoutputs/phaseoutputs.go:103` — above `Report: observe(listing, report),`

```text
// Live naming truth (cycles 1432/1441/1442/1452, corrected by
// review + first monitored wave): the REPORT follows the resolved
// contract (retrospective-report.md, test-report.md, memo.md).
// Usage is PHASE-named everywhere — the C1 chokepoint writes
// %s-usage.json from the phase. Prompt and events are PHASE-named
// for runner-dispatched phases (the runner sets Agent = phase),
// but retro dispatches outside that path and its prompt is
// AGENT-named (retrospective-prompt.txt) — the fallback
// candidate's one live consumer. Retro has NO events stream under
// either name live (retro-observer-events.ndjson is the
// observer's, a different mechanism): that gap is REAL and
// intentionally reported — fix queued as retro-events-stream item.
```

### `go/internal/phaseoutputs/phaseoutputs_test.go:82` — above `func TestSurvey_RetroLiveShapeExercisesTheAgentFallbackAndSurfacesTheEventsGap(t *testing.T) {`

```text
// Phase→artifact naming is not uniform, and retro is the proof — its LIVE
// shape (verified in cycles 1432/1441/1442, corrected by review after a wrong
// first guess) is: registry-named report (retrospective-report.md), AGENT-named
// prompt (retrospective-prompt.txt — retro dispatches outside the runner path
// that keys streams on the phase name), PHASE-named usage (retro-usage.json —
// the C1 chokepoint writes %s-usage.json from the phase), and NO events stream
// under either name (retro-observer-events.ndjson exists but is the observer's,
// a different mechanism). This test pins all three namings — including the
// agent-named fallback candidate, whose only live consumer is retro's prompt —
// and pins that the missing events stream IS reported: it is a real
// instrumentation gap this tool exists to surface, not a naming artifact.
```

### `go/internal/phaseoutputs/phaseoutputs_test.go:114` — above `func TestSurvey_NoPhaseIsExemptWithoutLiveEvidence(t *testing.T) {`

```text
// No phase gets a blanket exemption: both development-time guesses
// (inherited-defect-reconcile, coverage-gate) turned out to be fully-dispatched
// phases with complete outputs in live workspaces (cycles 1432/1442). The only
// trimming is PER-ARTIFACT, for phases with cited live evidence (nativePhases)
// — and even those always owe usage.
```

### `go/internal/phaseoutputs/phaseoutputs_test.go:128` — above `func TestSurvey_NativeShipOwesOnlyUsage(t *testing.T) {`

```text
// Ship is a NATIVE phase — it completes in Go with no agent dispatch. Live
// citation (the register's admission requirement): cycles 1452/1453, ship in
// completed_phases with ship-usage.json only. The registry already carries the
// no-report fact as NoArtifact:true; prompt/events come from the native
// register. Usage is ALWAYS owed — the C1 chokepoint writes it for every
// recorded phase, so its absence is a real recording failure even for ship.
```

### `go/internal/phaseoutputs/phaseoutputs_test.go:164` — above `func TestSurvey_ReportNameComesFromTheResolver(t *testing.T) {`

```text
// Memo's report is memo.md — declared by its spec, written live every PASS
// cycle. Cycle-1452 surveyed a FALSE gap (memo-report.md) because resolution
// went through the compiled map alone and fell back to the convention. The fix
// is the RESOLVER, not a builtin entry: review caught that a builtin memo
// entry would shadow the spec-derived contract and silently drop its required
// sections from the live gate. This pins both directions — a catalog-aware
// resolver finds memo.md, and the builtin-only degrade still names the
// conventional fallback rather than inventing anything.
```

### `go/internal/phaseoutputs/phaseoutputs_test.go:184` — above `func TestNativePhases_RegisterIsExactlyShip(t *testing.T) {`

```text
// The native register grows ONLY with a cited live cycle — enforced by a pin,
// not a comment. Ship entered on cycles 1452/1453; anything else appearing
// here must bring its own citation and change this test deliberately.
```

### `go/internal/phaseoutputs/phaseoutputs_test.go:194` — above `func TestSurvey_DuplicateCompletedPhasesCollapseToOneRow(t *testing.T) {`

```text
// run.json records a re-dispatched phase once per completion (cycle-1452:
// audit appears twice). One phase = one row — duplicate rows inflate the
// completeness denominator and double-count the same files.
```
