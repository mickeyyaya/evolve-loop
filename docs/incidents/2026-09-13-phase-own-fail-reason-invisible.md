# 2026-09-13 — a phase's own FAIL reason was persisted nowhere; the seal called it "phase-infra class"

## Impact
Two-wave health batch (2026-09-12/13), cycles **1634** (wave 1) and **1636**
(wave 2): triage FAILed on both. Each seal read

```
"FailReasons": ["phase triage: verdict FAIL with no recorded abort reason (phase-infra class)"]
```

and `phase-timing.json` held only `verdict: FAIL` for triage — no reason
anywhere on disk or in the loop log. Each cycle cost an operator investigation
(cycle 1634 first, then 1636 the same night) to discover that the FAIL was not
infrastructure at all: triage's Classify had refused the `top_n` card because
its `files=` named a `guards.ProtectedSurfaceManifest` path
(`go/internal/phases/audit/audit.go`, then `go/internal/phases/ship/gitops.go`)
— the F4 admission rule, working as designed. Classify had said so, as an
error-severity diagnostic:

```
top_n card "<id>" names protected surface "<path>" — control-plane changes go through the console route (operator-gated), not lane top_n
```

Consequences of the mislabel: the retro fingerprinted an infra identity, the
identical-fingerprint breaker could not see the recurrence, the batch report's
FAILED_EXPLAINED detail said only "phase triage recorded verdict FAIL", and two
inbox items that can never ship through a lane kept being claimed until an
operator routed them `console-manual` by hand.

## Root cause
The C1 single-source outcome record (`recovery.PhaseOutcome` → `phasetiming.Entry`,
ADR-0044) carried the host's `abort_reason` but had no field for the phase's
OWN diagnostics: `phaseOutcomeFrom` copied verdict, cost, duration, tokens and
model provenance from the `PhaseResponse` and dropped `Diagnostics`. Floor
phases (tdd/build/audit/ship) never noticed, because their FAIL diagnostics
ride a side channel (`persistFloorFailReasons` → `cs.AuditFailReasons` /
`ShipFailReasons`, `floorVerdictError` → the FailedRecord). Judgment phases
feed `recordJudgmentLesson`. Every other phase — triage, scout, intent, the
scans — lost its reason at `phase_completion.persist()`. `backfillFailReasons`
then saw a FAIL with no abort reason and synthesized the infra marker;
`cyclehealth.ClassifyOutcome` produced a verdict-only detail; nothing logged.

## Fix (PR: fix/classify-diag-persisted — the first producer into the recorder the seal reads)
- **The record carries the phase's own diagnostics.** `phasetiming.Entry.Diagnostics`
  (`json:"diagnostics,omitempty"`, `[]cyclestate.Diagnostic` — severity +
  message, exactly as reported; legacy logs parse to nil) and
  `recovery.PhaseOutcome.Diagnostics`. `phaseOutcomeFrom` relays
  `resp.Diagnostics`; `recordPhaseOutcome` — the one chokepoint, on both
  dispatch roots — writes them to the entry. The usage sidecar stays a usage
  record; the timing record is what the seal, cyclehealth and the dossier read.
- **One rule for "which diagnostics are reasons".** `cyclestate.ErrorMessages(diags)`
  beside the `Diagnostic` type and its severity vocabulary (`SeverityError`,
  `SeverityWarning`) is the ONE projection from a phase's diagnostics to its
  FAIL reasons: error-severity messages, in order; warnings are a trail. core's
  five consumers (the FailedRecord, floor fail reasons, the chokepoint log line,
  the seal's backfill, judgment lessons) and cyclehealth all call it; the
  former package-local copy in core was deleted. The ledger's verdict summary
  (`ReasonFromDiagnostics`, `core/verdict.go`) projects the same function — the
  re-review found it re-deriving the rule with bare literals — and the stale
  pointers to the deleted function in the audit constitution, `audit.go` and
  twelve test comments were swept to `cyclestate.ErrorMessages`.
- **One rendering of "why FAIL".** `verdictFailReason(diags)` (`core/cyclerun.go`)
  joins those messages ("verdict=FAIL: m1; m2", bare "verdict=FAIL" when none).
  `floorVerdictError` now projects it (byte-identical output), the chokepoint
  log line uses it, and the seal uses it — one field, one rule, rendered per
  surface (the floor side channel still carries bare messages into
  `FailReasons` for the ADR-0072 trust boundary; content agrees because the
  projection is shared).
- **The seal names the reason.** `backfillFailReasons` priority is now: abort
  reasons; else each failing phase's own error-severity diagnostics
  (`phase triage: verdict=FAIL: top_n card … names protected surface …`); else
  the explicit infra marker — which is now a true statement, reserved for a
  FAIL that recorded nothing. Warnings are not reasons.
- **The chokepoint logs it once,** module-tagged:
  `[orchestrator] phase triage verdict=FAIL: top_n card … names protected surface …`.
  A PASS keeps its warning trail on the record (reconciliation, ACS-floor
  overrides — the runner already meant those to be visible) without a failure
  line.
- **The batch report says why — from the same schema home.** `cyclehealth.ClassifyOutcome`
  now reads `phase-timing.json` through `phasetiming.Read` / `phasetiming.Entry`
  and projects `cyclestate.ErrorMessages` for its FAILED_EXPLAINED detail. Its
  hand-typed C1 mirror struct is gone: the architecture review found that a
  renamed key or a changed severity vocabulary would have parsed to nil there
  and silently reverted the detail to verdict-only with every test green
  (`cyclestate` and `phasetiming` are leaves outside cyclehealth's import
  closure, so the import creates no cycle; cyclehealth still imports no core).
  The composed regression test now runs `ClassifyOutcome` over the workspace
  core wrote, so writer/reader parity is guarded, not assumed.

## Regression
- `core/phase_diagnostics_seal_test.go::TestRunCycle_TriageOwnFailReasonReachesTheSealAndTheRecord`
  — cycles 1634/1636's shape through the composed `RunCycle` path: a triage
  runner returning FAIL with the F4 diagnostic; asserts the seal names it, does
  not say "phase-infra class", and `phase-timing.json` on disk carries the
  diagnostics verbatim. RED on the old code with the exact live marker
  (observed before the fix), GREEN after.
- `::TestRecordPhaseOutcome_CarriesThePhaseDiagnosticsAndNamesAReasonedFail` —
  the chokepoint: `phaseOutcomeFrom` relays, the entry carries, the log line is
  emitted once with the seal's wording; a PASS with warnings is recorded, not
  logged as a failure.
- `core/failreasons_backfill_test.go::TestBackfillFailReasons_PhaseOwnDiagnosticsNameTheReason`
  — the seal's priority rule, including the warning-only shape that keeps the
  infra marker.
- `phasetiming/diagnostics_test.go::TestEntry_DiagnosticsRideTheRecordAndAreOmittedWhenAbsent`
  — the wire format: persisted under `diagnostics`, omitted when absent, legacy
  logs parse to nil.
- `cyclehealth/outcome_diagnostics_test.go::TestClassifyOutcome_FailVerdictDetailNamesThePhaseOwnReason`
  — the batch-report detail names the reason; warnings excluded.
- `cyclestate/diagnostic_test.go::TestErrorMessages_ProjectsOnlyErrorSeverityInOrder`
  — the one projection: error-severity messages in order, nil for none, warnings
  never; names the severity vocabulary.
- The composed test's parity assertion: `cyclehealth.ClassifyOutcome` over the
  workspace core wrote names the reason (writer and reader share the schema).
- Mutation proof (each mutant confirmed to BUILD before its run): `phaseOutcomeFrom`
  drops the diagnostics; the entry omits them; the chokepoint never logs; the
  seal ignores them; `verdictFailReason` drops the messages;
  `cyclestate.ErrorMessages` counts warnings (killed by eight tests across
  core, cyclehealth and cyclestate — the rule has one home and many guards);
  cyclehealth's detail ignores the projection; cyclehealth treats a missing
  record as present (the `phasetiming.Read` contract, killed by the
  pre-existing empty-workspace case) — all killed by the named tests above
  (plus the pre-existing `TestFloorVerdictError_JoinsErrorSeverityDiagnostics`,
  `TestDetectVerdictIncoherence_WarningOnlyDiags_StillHalts` and the
  `TestVerdictConflict_*` pair, which guard the shared formatter and the
  severity rule).

## Campaign note
This is the first producer wired into the recorder the seal reads: a phase's
own structured diagnostics now travel on the durable per-phase record on every
terminal path, with one module-tagged log line at the chokepoint. The Signal
Center work that follows (one event schema, every phase a producer, the
orchestrator a registered listener) should subscribe at this same chokepoint
rather than mint a second recorder.

## Follow-ups (filed, not done here)
- The two console-manual inbox items surfaced by these cycles
  (`self-consistency-on-decision-phases`, `phase-stub-shape-rule-at-ship-staging`)
  are real bugs whose fix lives on the protected surface; they are the console
  owner's to land.
- Triage admission could refuse a protected-surface card at SELECTION time
  (before the agent writes the decision) instead of at Classify, saving the
  dispatch; that is a persona/gate change, not a recorder change.
