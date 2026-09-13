# Unit 02 — Failure diagnostics and delivery-failure classification (`internal/core/failurediag`)

- **Program:** [ADR-0103](../adr/0103-component-breakdown-program.md) · units table row 02 (ADR:60) · the "failure learning … follow as units 02–03" note on design §12 row 1 (signal-center-design.md:462) · the cluster unit 01 handed off by name ([01-outcome-recorder.md](01-outcome-recorder.md):26 — "they need the core sentinel `ErrArtifactTimeout` — unit 02")
- **Extracted from:** `go/internal/core/failure_learning.go:115-225` — `phaseFailureDiag` (:117-125), `DeliveryFailureCause` (:129-157), `artifactTimeoutTypedCause` (:159-173), `parseQuotedMarkerValue` (:175-189), `writePhaseFailureDiag` (:194-225)
- **Module tag:** `failurediag` · **Status:** landed 2026-09-13

## 1. Context

When a mandatory phase aborts — retries exhausted (cyclerun_dispatch.go:322), a non-canonical verdict exhausted (:372), an all-families quota pause (quota_pause.go:34) or the first failing evaluator of a parallel batch (evaluate_batch.go:183) — the orchestrator writes one terminal diagnostic, `<workspace>/<phase>-failure-diag.json`, on the line after the C1 record and before failure learning (cyclerun_dispatch.go:321-327). The file carries the phase error verbatim, an exit-code class that is 81 for an artifact timeout and nothing else (failure_learning.go:195-201), and — since cycle 1562 — the driver's classified prompt-delivery reason (`prompt submit_wedged (resends=3)`) parsed out of the bridge's `artifact-timeout: cause=… reason=…` marker (failure_learning.go:129-157). The classifier is also the seam retro keys its single bounded relaunch on (internal/phases/retro/retro.go:218).

Today the cluster is 111 lines of `failure_learning.go` (937 lines) beside the failure-learning engine and the carryover lifecycle: an unexported struct whose field order IS the on-disk wire order with no production reader in Go (the shape is pinned only by a doc example, phase-timing-and-diagnostics.md:124-135), a hand-rolled tmp+rename writer with three `[orchestrator] WARN failure-diag …` stderr lines (:213, :219, :223) that no test pins by text, the marker's four wire tokens spelled as eight literals (:135, :138, :144, :148, :153, :160, :161, :168, :171) with no shared constant against the bridge's producer (stopreview.go:101, driver_tmux_wait_diagnostic.go:16, :105-110, attempt_telemetry.go:264), and an `*exec.ExitError` branch (:199-200) that no test covers. The exit-81 gate is pinned as timeout-ONLY by a table test and a go/ast scan of `failure_learning.go` for `writePhaseFailureDiag` (infra_teardown_timeout_only_test.go:51-100, :121-150) — a pin whose own comment says to move it with the gate (:143).

## 2. Boundary

| Moves to `failurediag` | Stays in `core` | Why |
|---|---|---|
| the on-disk contract `phaseFailureDiag` → exported `Sidecar` (same seven tags, same order, no omitempty; failure_learning.go:117-125) and its path rule `SidecarPath` (:216) | `ErrArtifactTimeout` (errors.go:33) — the Bridge PORT's sentinel by design (errors.go:26-32: "lives on the Bridge port … so the generic phase runner can errors.Is-match it WITHOUT importing a specific driver"), wrapped by bridge/engine.go:636-638, matched by failure_hook.go:79, IsInfraTeardownError errors.go:89-91 and 30+ test files | the unit cannot import core; it receives the ONE core fact it needs as an injected predicate `isTimeout func(error) bool` — core's new 3-line `isArtifactTimeout` beside IsInfraTeardownError — so the var never moves, is never re-exported, and errors.Is pointer identity is untouched by construction |
| the exit-code projection (:195-201) → `exitCode(err, isTimeout)`; the diagnosis (:202-210) → pure `(*Writer).diagnose`; the marshal + tmp write + rename (:211-224) → `(*Writer).Write`; the two reachable stderr lines (:219, :223) → one `FAILUREDIAG_SIDECAR_WRITE_FAILED` signal each; the marshal branch (:212-215) REMOVED as unreachable | `phaseOutcomeFrom` (failure_learning.go:49-74) — already decided in 01 doc:24 (projects `core.PhaseResponse`; 27 core call sites; resume_parity_pin_test.go pins its unqualified spelling) | a struct of strings and ints has no `json.Marshal` error path (encoding/json fails only on unsupported types, non-finite floats, cycles or Marshaler errors; invalid UTF-8 is coerced), so the branch could never fire and the 100 % `.cover-strict` gate requires its removal, not its excuse (ADR:34-36) |
| `DeliveryFailureCause` (:129-157) → `DeliveryFailureCause(err, isTimeout)` with gates 2 and 3 byte-for-byte; `artifactTimeoutTypedCause` (:159-173) → unexported `typedCause`; `parseQuotedMarkerValue` (:175-189) → unexported `quotedValue`; the nine token literals → four exported consts `MarkerPrefix`, `CauseField`, `ReasonField`, `CauseSubmitWedged` + `ExitCodeArtifactTimeout = 81` | the four abort call sites (cyclerun_dispatch.go:322, :372; quota_pause.go:34; evaluate_batch.go:183) and their order `recordPhaseOutcome → diag → [adviseOnUnclassifiedFailure] → recordFailureLearning`; retro.go:218 and the three bridge binding tests (delivery_failure_format_binding_test.go:32, :55, :93) keep calling `core.DeliveryFailureCause` | Strangler Fig (ADR:37-39): the orchestrator keeps `writePhaseFailureDiag` under its old name as a method facade and `DeliveryFailureCause` under its exact exported signature; the four sites gain a `cr.o.` receiver and lose the trailing `cr.o.now` (see §9 — the one deviation, and why) |
| — | `adoptStructuredFailure` (:594-605) with `capRunes`/`capStrings`/`maxAdoptedDefects`/`maxAdoptedDefectRunes` (:607-633) | (1) it emits no log line, so the observability lens gains nothing; (2) its sole caller `recordFailedApproachState` (:238-279) reads the block ONCE at :258 and threads it to `writeDeterministicLearning` (:415-418) so state.json and the lesson corpus cannot diverge — splitting the read from its consumer across units risks exactly that; (3) `capRunes` is also spelled by carryover_merge.go:76, prescription_carryover.go:97, phase_advisor.go:860 and ApplyDefectsAsCarryoverTodos :889 (units 03/04) — hosting it in a diagnostics unit would make the carryover lifecycle and the advisor import failure diagnostics for a string cap. They move with the engine's state-recorder seam |
| — | the ADR-0048 shadow-grade stderr line inside the `recordPhaseOutcome` facade (:96, `[graduated-enforcement SHADOW]`) — orchestrator policy unit 01 left deliberately (01 doc:25) | out of scope; the other 19 `[orchestrator]` lines of the file belong to `recordFailureLearning` ×10 (:303-387), `writeDeterministicLearning` ×2 (:446, :455), `retroRemediationItems` ×2 (:473, :483), `recordRecurrenceClosure` ×3 (:576-584) and `writeFailureLearningState` ×2 (:674, :686) — the engine and unit 03 |

The "diagnostic half of the engine" question, answered with evidence rather than left to the reviewer: all 16 steps of `recordFailureLearning` (failure_learning.go:281-394) contain no sidecar or classification work, because every one of its four abort callers writes the sidecar BEFORE calling it (cyclerun_dispatch.go:322→:327, :372→:373, quota_pause.go:34→:35, evaluate_batch.go:183→:184). The diagnostic half is already outside the method; it is exactly what moves.

The unit imports only stdlib (`encoding/json`, `errors`, `os`, `os/exec`, `path/filepath`, `strconv`, `strings`, `time`) and `internal/signalcenter`, whose closure is `internal/log` + itself (`go list -deps ./internal/signalcenter`, verified 2026-09-13); `core` imports the unit beside `internal/core/outcome` (today `core`'s only `internal/core/*` import). `internal/bridge` and `internal/phases/retro` each depend on `internal/core` (go list count 1) and keep the facade; because the unit never imports core, bridge may later import the unit's tokens without a cycle — bridge already imports the leaf sub-package `internal/core/evidence`. No core type or var appears in the unit's API.

## 3. API

```go
package failurediag // go/internal/core/failurediag/{diag.go, delivery.go}

// The unit's one code, registered in init() with signalcenter.RegisterCode(signalcenter.ModuleFailureDiag, …).
const CodeSidecarWriteFailed signalcenter.Code = "FAILUREDIAG_SIDECAR_WRITE_FAILED"

// Sidecar is the declared on-disk contract of <workspace>/<phase>-failure-diag.json.
// Field order IS wire order (compact json.Marshal, no omitempty, every field always present).
type Sidecar struct {
    Phase           string `json:"phase"`
    Cycle           int    `json:"cycle"`
    ErrorMessage    string `json:"error_message"`    // phaseErr.Error() verbatim
    DeliveryFailure string `json:"delivery_failure"` // DeliveryFailureCause or ""
    ExitCode        int    `json:"exit_code"`        // 81 iff isTimeout; else *exec.ExitError.ExitCode(); else 1
    AttemptCount    int    `json:"attempt_count"`
    Timestamp       string `json:"timestamp"`        // now().UTC().Format(time.RFC3339)
}
func SidecarPath(workspace, phase string) string // filepath.Join(workspace, phase+"-failure-diag.json")

type Writer struct{ /* now func() time.Time; isTimeout func(error) bool; signals func() *signalcenter.Center */ }
type Option func(*Writer)

// NewWriter injects the orchestrator's clock and its artifact-timeout predicate — the ONE gate that
// decides both exit_code 81 and whether a delivery cause may be attributed. isTimeout must be non-nil;
// there is no guard (ADR-0103 item 3) — a nil func panics at first use, never "nothing is a timeout".
func NewWriter(now func() time.Time, isTimeout func(error) bool, opts ...Option) *Writer
func WithSignals(c func() *signalcenter.Center) Option // accessor read live; nil = Null Object (tests only)
func (w *Writer) SignalsWired() bool

// Write renders the Sidecar (diagnose) and lands it at SidecarPath via <path>.tmp + os.Rename, 0o644.
// Best-effort: a temp-write or rename failure is ONE FAILUREDIAG_SIDECAR_WRITE_FAILED WARN and returns;
// the caller's phase error is never masked. Never returns an error (the pre-extraction contract, :191-193).
func (w *Writer) Write(workspace, phase string, cycle int, phaseErr error, attempts int)

// DeliveryFailureCause classifies an evidenced prompt-delivery failure: "" unless isTimeout(err); with a
// host-owned cause= token the token is authoritative (any cause but submit_wedged → ""; submit_wedged →
// the quoted reason when it names the token, else the bare cause; reason= honoured only IMMEDIATELY after
// the cause token); without cause= (legacy) → the quoted reason= only when it names submit_wedged.
func DeliveryFailureCause(err error, isTimeout func(error) bool) string

// The artifact-timeout wire contract as the decoder spells it (the producer's copies: bridge
// stopreview.go:101, driver_tmux_wait_diagnostic.go:16 and :105-110, attempt_telemetry.go:264, exitcodes.go:31).
const (
    MarkerPrefix            = "artifact-timeout: "
    CauseField              = "cause="
    ReasonField             = "reason="
    CauseSubmitWedged       = "submit_wedged"
    ExitCodeArtifactTimeout = 81
)

// unexported, in-package tested
func (w *Writer) diagnose(phase string, cycle int, phaseErr error, attempts int) Sidecar // pure over the injected clock+predicate
func exitCode(err error, isTimeout func(error) bool) int
func typedCause(message string) (cause, reason string, present bool)
func quotedValue(value string) (string, bool)
func (w *Writer) warn(origin string, cycle int, phase string, code signalcenter.Code, reason string, fields map[string]string)
```

```go
package core // go/internal/core/errors.go (+3 lines) and go/internal/core/failure_diag.go (new, ~45 lines)

// isArtifactTimeout is the timeout-ONLY gate unit 02 injects (exit 81 and delivery attribution);
// never widen it to the union. Beside IsInfraTeardownError (errors.go:89-91).
func isArtifactTimeout(err error) bool { return errors.Is(err, ErrArtifactTimeout) }

// failureDiag returns the unit-02 writer: eager from NewOrchestrator, lazily built and cached for a
// literal Orchestrator. No nil-orchestrator branch: every abort site dereferences cr.o on the line
// before (cr.o.recordPhaseOutcome) and reads cr.o.now today, so a nil orchestrator already panics there.
func (o *Orchestrator) failureDiag() *failurediag.Writer {
    if o.diag == nil { o.diag = o.wiredFailureDiag() }
    return o.diag
}

// wiredFailureDiag is the ONE construction (pinned by TestFailureDiagWriter_OneConstructionSite and by
// the gate row of TestTimeoutOnlySites_NotWidenedToUnion): clock and Center read live, the gate isArtifactTimeout ONLY.
func (o *Orchestrator) wiredFailureDiag() *failurediag.Writer {
    return failurediag.NewWriter(func() time.Time { return o.now() }, isArtifactTimeout,
        failurediag.WithSignals(func() *signalcenter.Center { return o.signals }))
}

// writePhaseFailureDiag is the seam every abort site keeps; the clock is injected now.
func (o *Orchestrator) writePhaseFailureDiag(workspace, phase string, cycle int, phaseErr error, attempts int) {
    o.failureDiag().Write(workspace, phase, cycle, phaseErr, attempts)
}

// DeliveryFailureCause keeps its exported signature for retro.go:218 and the bridge binding tests.
func DeliveryFailureCause(err error) string { return failurediag.DeliveryFailureCause(err, isArtifactTimeout) }
```

`Orchestrator` gains `diag *failurediag.Writer // unit 02 (ADR-0103)` beside `outcome` (orchestrator.go:277); `NewOrchestrator` sets `o.diag = o.wiredFailureDiag()` on the line after `o.outcome = o.wiredRecorder()` (orchestrator.go:862), i.e. after the options loop (:856-858).

## 4. Patterns

| Pattern | Force it answers |
|---|---|
| **Extract Class** (`Writer`) / **Extract Function** (`DeliveryFailureCause`) — Fowler | the sidecar writer and the classifier lived beside the failure-learning engine in a 937-line file; one type owns the record, one function owns the classification, both testable without an `Orchestrator` |
| **Pure core / imperative shell** (`diagnose`, `exitCode`, `DeliveryFailureCause` pure; `Write` persists and signals) | the diagnosis is byte-testable with no filesystem and is the mutant seam; the shell owns the only side effects (one file, ≤ 1 signal) |
| **Explicit DI at construction** (`NewWriter(now, isTimeout, opts...)`, functional `Option`, `WithSignals`) | ADR:40-41; the same shape as `outcome.NewRecorder` (recorder.go:97-103) so the reviewer fleet and the next units have one construction idiom |
| **Predicate injection = interface at the point of use** (`isTimeout func(error) bool`) | the unit needs `ErrArtifactTimeout`, a core sentinel it cannot import; a func type is the smallest interface; the sentinel and its `errors.Is` identity stay on the Bridge port (errors.go:26-33) where the bridge, the runner and 30+ test files expect them; the gate has ONE core definition (`isArtifactTimeout`) pinned at its body and at both injection sites |
| **Late-bound collaborators** (closure clock, Center accessor) | thirty-three core tests build a literal `Orchestrator` and swap `o.now` after construction (01 doc:148-149; failure_learning.go:930-933); `core.WithSignalCenter` is an Option tests apply after `NewOrchestrator` (signal.go:37-46; phase_timing_composition_test.go:143-157) — a snapshot would report into nothing (01 review MEDIUM-1) |
| **Null Object** (`WithSignals(nil)`; `(*Center)(nil).Emit` is a no-op, center.go:130-133) | literal-orchestrator tests write the sidecar with no Center; production roots always wire one and `SignalsWired` proves it (cmd_cycle_signal_center_test.go:23-30, :232-256) |
| **Strangler Fig facade** (`(*Orchestrator).writePhaseFailureDiag`, `core.DeliveryFailureCause`) | ADR:37-39; retro.go:218, the three bridge binding tests and the five ACS-named core tests never learn the unit exists; removed only when the last caller migrates (ADR:52-53) |
| **Declared on-disk contract as an exported type** (`Sidecar` + `SidecarPath`) | the file has no production Go reader (grep: only a comment at bridge/engine.go:609), so its shape was pinned by a doc example alone; an exported struct with a byte-identity test is the single source the two operator docs project from |
| **Single-source-with-projection** for the wire tokens and exit code (`MarkerPrefix`, `CauseField`, `ReasonField`, `CauseSubmitWedged`, `ExitCodeArtifactTimeout`) | the producer (bridge) and the decoder (core) each spell the marker with no shared constant; the leaf is the one place both can import (follow-up F2 re-points the bridge token sites; the exit-code TABLE at bridge/exitcodes.go:31 is NOT projected — acs/cycle1580/predicates_test.go:98-122 parses it for `Exit<Name> = <int literal>` rows) |
| **One producer helper per unit** (`warn`) | module, kind and severity fixed in one place (recorder.go:220-225 precedent); call sites supply origin, code, a reason naming the step and the error, and `fields.path`; the unit never writes to `os.Stderr` |
| **Source-scan construction guard** (`TestFailureDiagWriter_OneConstructionSite`) | the writer is exported now, so visibility no longer makes "one wired writer" structurally true (phase_timing_single_writer_test.go:30-70 idiom); `.Write(` is too common a token, `failurediag.NewWriter(` is not |
| **Structural pin relocation** (`TestTimeoutOnlySites_NotWidenedToUnion` rows) | the timeout-only gate must never widen to the (timeout OR transient) union; after the move the unit cannot even import the transient sentinel, so the pin is stronger, and it stays in the core test file so acs/cycle1267 keeps its `corePkg` |

## 5. Module contract

| Item | Value |
|---|---|
| Module | `failurediag` (NEW in the closed set: event.go:68-92 const + knownModules :94-99; `TestModule_ClosedSet` event_test.go:39-49 lists it and its count literal moves 23 → 24; design §5.1 prose signal-center-design.md:164-166). Code prefix `FAILUREDIAG_` (event.go codePrefix). Rejected: riding `outcome` (unit 01's tag: package == tag, and ADR:47-48's "filter by module, then open ONE small package" breaks if two packages share a tag) |
| `FAILUREDIAG_SIDECAR_WRITE_FAILED` WARN | `<phase>-failure-diag.json` could not be written: the reason names the step (`failure-diag temp write failed: <err>` for `os.WriteFile` of `<path>.tmp`, `failure-diag rename failed: <err>` for `os.Rename`) and `fields.path` = `SidecarPath`; cycle and phase set. The phase abort proceeds unchanged (the diag is best-effort and never masks the phase error) and, as before, a `.tmp` may be left beside the target. Registered in the package `init()` with this doc; projected into `signal-codes.md` as a `### failurediag` block (between `### bridge` and `### gate.contract`) |
| Origin | `Writer.Write` |
| Kind | **`failurediag.warning`** (NEW, non-terminal: event.go:124-146 const + knownKinds :148-154, NOT terminalKinds :158-161; `TestKind_ClosedSetAndTerminal` `all` list event_test.go:69-72 derives its own count at :84; design §5.2 table row signal-center-design.md:171-185). Precedent: `bridge.warning` (:134) and `outcome.warning` (:138; 01 doc:86). Rejected: `outcome.warning`, whose §5.2 meaning is "the phase-outcome recorder could not persist a record" — a different unit |
| Console rendering | via the root's WARN-filtered `StderrSink` (sinks.go:117-150): `[failurediag] failurediag.warning WARN FAILUREDIAG_SIDECAR_WRITE_FAILED cycle=N phase=p seq=s origin=Writer.Write — failure-diag rename failed: <err> path=…`; the same event lands in `.evolve/signals.ndjson` |
| Removed lines | failure_learning.go:219 `[orchestrator] WARN failure-diag write: %v` and :223 `[orchestrator] WARN failure-diag rename: %v` → the one code above. failure_learning.go:213 `[orchestrator] WARN failure-diag marshal: %v` is DELETED WITHOUT A CODE with its unreachable branch (:212-215): `Sidecar` holds only strings and ints, so `data, _ := json.Marshal(diag)` with the reason in a comment (`.golangci.yml` runs the standard set; errcheck does not flag a blank assignment). Two of the file's 23 stderr lines become one code; the third could never fire |
| Not added in this slice | `FAILUREDIAG_DELIVERY_FAILURE` — the classified `submit_wedged` verdict projected onto the stream (WARN; reason = the classified cause; fields path/exit_code/attempt_count; origin `Writer.Write`; emitted BEFORE the persist so a disk failure cannot lose the classification; kind `failure.diagnosed`, new and non-terminal, reviewed in the follow-up because `failurediag.warning` means the unit's own persist failure). It is the one line that lets an on-call answer "why did delivery fail" with `grep FAILUREDIAG_DELIVERY_FAILURE signals.ndjson` instead of parsing `phase.aborted`'s `abort_reason` prose (signal.go:86-89) and opening a sidecar nobody reads. Deferred so this landing is byte-identical on disk AND on the signal stream (ADR:37-39); named as follow-up F1 in §8 with its ordering test |

## 6. Tests (red first)

Unit package `failurediag` (`delivery_test.go`, `diag_test.go`):

1. `TestWireTokens_AreTheDecodersSpellings` — `MarkerPrefix == "artifact-timeout: "`, `CauseField == "cause="`, `ReasonField == "reason="`, `CauseSubmitWedged == "submit_wedged"`, `ExitCodeArtifactTimeout == 81` (the literal values bridge spells at stopreview.go:101, driver_tmux_wait_diagnostic.go:16, :105-110, exitcodes.go:31); names every exported const for apicover.
2. `TestDeliveryFailureCause_PrefersTypedMarkerField` — typed `cause=submit_wedged reason=<strconv.Quote of a reason with embedded escaped quotes>` → the exact unquoted reason (exercises the escape walk).
3. `TestDeliveryFailureCause_DoesNotParseCauseTextInsideReason` — `cause=review_stop` whose reason quotes `cause=submit_wedged` → `""` (the host-owned token is authoritative; reviewer prose cannot forge it).
4. `TestDeliveryFailureCause_ReasonMustImmediatelyFollowCause` — `cause=submit_wedged phase=retro reason="prompt submit_wedged"` → bare `"submit_wedged"` (the position-strict rule at :168, pinned in the unit, not only through the facade).
5. `TestDeliveryFailureCause_TypedWedgedReasonWithoutTheTokenReturnsTheBareCause` — `cause=submit_wedged reason="resent thrice"` → `"submit_wedged"`; `cause=submit_wedged` with no `reason=` → `"submit_wedged"`.
6. `TestDeliveryFailureCause_LegacyReasonMarkerStillClassifies` — the pre-`cause=` shape of failure_delivery_evidence_test.go:44-47 → contains `submit_wedged` and `prompt`; the same shape with a generic silence reason → `""`.
7. `TestDeliveryFailureCause_GatesOnTheInjectedPredicate` — a perfect marker under a predicate returning false → `""` (the forge guard); a `fmt.Errorf("…: %w", sentinel)`-wrapped error under `errors.Is` against that sentinel → classified (wrapping is matched).
8. `TestDeliveryFailureCause_MalformedAndTruncatedMarkersAreEmpty` — table: no `artifact-timeout: `; marker without `reason=`; unquoted value; unterminated quote; a marker cut at 1024 runes with an ellipsis before the closing quote (bridge/engine.go:721 `boundArtifactTimeoutSummary`) → all `""`, silently by design. Runs under the real predicate (every row wraps the test sentinel with `%w`), so the legacy parser's and `quotedValue`'s early returns are reached — the 100 % gate proves it.
9. `TestWriter_Diagnose_IsByteIdenticalToThePreExtractionSidecar` — fixed clock `time.Unix(1_700_000_000, 0)` (the readFailureDiag clock, failure_delivery_evidence_test.go:81), cycle 1562, attempts 2, the wedged legacy error → `json.Marshal(diagnose(...))` equals the literal compact string with the seven keys in wire order, `delivery_failure` populated, `exit_code` 81; no filesystem.
10. `TestWriter_Diagnose_MapsExitCodeTimeoutOnly` — table: predicate true (bare) → 81; wrapped → 81; a real `*exec.ExitError` from `exec.Command("sh", "-c", "exit 7").Run()` → 7 (the branch no test covers today; `sh -c` precedent internal/evalqualitycheck/flakylint_test.go:370); `errors.New("index out of range")` → 1; an error the predicate rejects (the transient family's shape) → 1.
11. `TestWriter_Diagnose_TimestampIsUTCFromTheInjectedClock` — a `+09:00` clock renders `…Z` at second precision; `error_message` is `phaseErr.Error()` verbatim.
12. `TestWriter_Write_LandsTheSidecarAtomically` — the file at `SidecarPath` equals test 9's bytes, mode `0644`, and `<path>.tmp` is gone (rename happened).
13. `TestWriter_Write_TempWriteFailureIsAWarnSignal` — workspace is a regular FILE so `os.WriteFile` of `<path>.tmp` fails: exactly one event {Module `failurediag`, Kind `failurediag.warning`, Severity WARN, Code `FAILUREDIAG_SIDECAR_WRITE_FAILED`, Origin `Writer.Write`, Cycle, Phase, Reason has prefix `failure-diag temp write failed: `, `Fields["path"] == SidecarPath`}; nothing written.
14. `TestWriter_Write_RenameFailureIsAWarnSignalAndLeavesTheTemp` — a non-empty directory at `SidecarPath` so `os.Rename` fails: reason prefix `failure-diag rename failed: `; the `.tmp` EXISTS (today's leak, pinned deliberately as behaviour — follow-up F3).
15. `TestWithSignals_NilIsTheNullObject` — `WithSignals(nil)` → `SignalsWired` false; `Write` on the rename-failure path with no Center does not panic and still staged the file.
16. `TestWithSignals_ReadsTheCenterLive` — the accessor returns nil, then a Center installed later; the late Center receives the signal (the accessor, not a snapshot).
17. `TestFailureDiagCodes_AreRegisteredWithDocs` — `IsRegistered(CodeSidecarWriteFailed)` owner `ModuleFailureDiag` (recorder_test.go:243-249 pattern).
18. `TestSidecarPath_IsThePhaseFailureDiagFile` — `filepath.Join(ws, "scout-failure-diag.json")`.

Core (`go/internal/core`, package `core`):

19. Kept, names unchanged, receiver edit only (`(&Orchestrator{now: now}).writePhaseFailureDiag(...)` at infra_teardown_timeout_only_test.go:82 and failure_delivery_evidence_test.go:82; `o.writePhaseFailureDiag(...)` at failure_advisor_coverage_test.go:240, `//go:build integration`): `TestWritePhaseFailureDiag_TimeoutOnlyNotWidened` (the ErrTransientBridgeFailure → 1 row stays pinned where that sentinel lives), `TestWritePhaseFailureDiag_DeliveryFailure_IsMachineReadable`, `_GenericSilence_NoDeliveryFailureAttribution`, `_NonTimeoutFailure_NoDeliveryFailureAttribution`, `TestDeliveryFailureCause_PrefersTypedMarkerField`, `_DoesNotParseCauseTextInsideReason`, `TestOrchestratorForensicsHelpersCoverage`; the RunCycle integrations `TestFailureDiag_WrittenOnPhaseAbort` / `TestFailureDiag_NotWrittenOnPassingCycle` (orchestrator_timing_test.go:126, :167) and `TestTransientRetry_Exhausted_WritesFailureDiag` (orchestrator_transient_test.go:136) untouched.
20. `TestTimeoutOnlySites_NotWidenedToUnion` (edited pin) — `timeoutOnlySite` (infra_teardown_timeout_only_test.go:104-108) gains an optional `gate string` (empty → `"ErrArtifactTimeout"`); the row `{failure_learning.go, writePhaseFailureDiag}` (:130) becomes three: `{errors.go, isArtifactTimeout}` (body mentions ErrArtifactTimeout, none of the banned three), `{failure_diag.go, wiredFailureDiag, gate: "isArtifactTimeout"}`, `{failure_diag.go, DeliveryFailureCause, gate: "isArtifactTimeout"}`; `funcBodyText` (:157-181) matches `fn.Name.Name` so method rows need no helper change; the header comment (:20) is updated.
21. `TestFailureDiag_LiteralOrchestratorGetsTheWriterOnce` — a literal `&Orchestrator{now: fixed}`: `failureDiag()` unwired; `o.failureDiag() == first` on the second call (identity, not rebuilt); a sidecar written through the facade carries the swapped clock's timestamp; `NewOrchestrator(..., WithSignalCenter(signalcenter.New()))` → `failureDiag().SignalsWired()`.
22. `TestFailureDiag_SeesASignalCenterAppliedAfterConstruction` — `NewOrchestrator` without a Center → unwired; `WithSignalCenter(recordingCenter())` (signal_cycle_test.go:19-24) applied after → wired; a rename failure provoked through `o.writePhaseFailureDiag` → exactly one `eventsOfKind(KindFailureDiagWarning)` with `FAILUREDIAG_SIDECAR_WRITE_FAILED` and Origin `Writer.Write`.
23. `TestErrArtifactTimeout_KeepsTheWireMessage` — `ErrArtifactTimeout.Error() == "core: bridge artifact timeout"` (the sidecar's `error_message` embeds it; the sentinel text has one non-test spelling, errors.go).
24. `TestFailureDiagWriter_OneConstructionSite` — module walk (phase_timing_single_writer_test.go:30-70 idiom): every non-test file outside `internal/core/failurediag/` containing `failurediag.NewWriter(` must be `internal/core/failure_diag.go`.

Signal Center and roots:

25. `TestModule_ClosedSet` (`ModuleFailureDiag` listed, count 24) and `TestKind_ClosedSetAndTerminal` (`KindFailureDiagWarning` in `all`, non-terminal); `TestRegistryConflicts_RealRegistryIsClean` and `TestCode_FormatAndModulePrefix` stay green.
26. `TestWireSimulateOrchestrator_FailureDiagWarningRenders` (cmd/evolve) — the cmd_cycle_signal_center_test.go:232-256 shape: a `failurediag.NewWriter(time.Now, func(error) bool { return false }, WithSignals(d.Signals accessor))` built on the `--simulate` root's Center (a _test.go construction, outside the scan guard) writes into a FILE-as-workspace; asserts `FAILUREDIAG_SIDECAR_WRITE_FAILED` on the console buffer and in `.evolve/signals.ndjson` — the module tag renders at the second root. `TestWireOrchestratorDeps_SignalCenterWired` and `TestNilSignalCenterRootsArePinned` (allowlist `internal/routingtest/engine.go`, :52-54) unchanged.
27. `TestSignalsCodes_RepoDocIsInSyncWithTheLinkedRegistry` (cmd_signals_test.go:17-21) — green only after `make -C go build && evolve signals codes generate` adds the `### failurediag` block.
28. ACS by-name, unchanged and green because every named test keeps its name in `corePkg`/`bridgePkg`/`retroPkg`: acs/cycle1267/predicates_test.go:136-140, acs/regression/cycle1270/predicates_test.go:259-276 (runs the two timeout-only pins by name), acs/cycle1562:158-161 and :170-175, acs/cycle1591:159, :166, :174-176 (`go test -run` with no match prints no PASS line, so no named core test may move).

Coverage bar: 100 % statements for `./internal/core/failurediag` (`.cover-strict`), every export named (apicover: `Sidecar`, `SidecarPath`, `Writer`, `Option`, `NewWriter`, `WithSignals`, `SignalsWired`, `Write`, `DeliveryFailureCause`, `CodeSidecarWriteFailed`, the four token consts, `ExitCodeArtifactTimeout`); the core facade file at 100 % through tests 19-24. Mutants are listed in §8 and hand-applied per design §11 (:441-443): each confirmed to BUILD, the named test watched go red, reverted, the final list recorded in §10 at landing (01 doc:155-160).

## 7. Why it is better than the original

| Dimension | Before | After |
|---|---|---|
| Observability | three prose `[orchestrator] WARN failure-diag …` lines with no code and no owner (:213, :219, :223), one of them unreachable | one registered code under module `failurediag`, origin `Writer.Write`, `fields.path`, on the Signal Center stream and the console at WARN, projected into `signal-codes.md`; triage of "the failure diag is missing" is `grep '"module":"failurediag"' signals.ndjson` then one ~255-line package |
| Isolation | the writer and the classifier are package-private functions of a 937-line file, testable only from package `core` with the real sentinel; the `*exec.ExitError` branch is untested | a leaf with an injected clock and predicate, 100 % covered directly, the pure `diagnose` as the mutant seam, the subprocess exit code pinned for the first time; the unit cannot import the transient sentinel, so "timeout-only" is structural, not only a text scan |
| Changeability | the on-disk contract is an unexported struct pinned by a doc example (no Go reader); the marker's tokens are nine literals with no shared constant against the producer; the timeout gate is two inline `errors.Is` calls | `Sidecar` + `SidecarPath` are the declared contract the two operator docs project from (runtime-reference.md:81 today omits `delivery_failure`); the tokens and `81` have one declaration bridge can adopt (F2); the gate has ONE core definition (`isArtifactTimeout`) pinned at its body and both injection sites |
| Measurement | a slice of core's 85 % apicover floor (`.apicover-enforce:278`) | `.cover-strict` 100 and apicover for the package; signal counts by `FAILUREDIAG_*` code per cycle |
| Callers | four abort sites, retro.go:218, three bridge tests, five ACS-named core tests | retro, the bridge tests and every ACS predicate untouched; the four abort sites change spelling only (`cr.o.` added, `, cr.o.now` dropped — §9) |

## 8. Risks and byte-identity guarantees

Byte identity (all pinned by tests 9-12 and the kept core tests):

- `<workspace>/<phase>-failure-diag.json` bytes are identical for every input: compact `json.Marshal` of the same seven fields in the same order with the same tags and no omitempty (an empty `delivery_failure` is written as `""`), e.g. `{"phase":"scout","cycle":173,"error_message":"phase scout: bridge: launch exit=81: core: bridge artifact timeout","delivery_failure":"","exit_code":81,"attempt_count":2,"timestamp":"2026-05-31T20:27:45Z"}` (the doc example at phase-timing-and-diagnostics.md:124-135 is the pretty-printed rendering of this shape); HTML escaping of `<>&` unchanged; no trailing newline; mode `0o644`; staged at `<path>.tmp` then `os.Rename` (failure_learning.go:216-224).
- `exit_code` mapping unchanged: 81 iff `errors.Is(phaseErr, ErrArtifactTimeout)` (wrapped or bare), else `*exec.ExitError.ExitCode()` via `errors.As`, else 1 — including the preserved quirk that quota_pause.go:34 records 1 for an exit=85 pause (the error wraps `ErrAllFamiliesExhausted`, not an `*exec.ExitError`; the ledger entry at :21-26 carries 85) and that `ErrTransientBridgeFailure` records 1 (`TestWritePhaseFailureDiag_TimeoutOnlyNotWidened`, unchanged assertions).
- `DeliveryFailureCause` returns the identical string for every input: the three gates and the position-strict typed parser move verbatim; only the token literals become consts of identical value; the bridge producer↔consumer binding tests (delivery_failure_format_binding_test.go:13, :37, :60) and retro's relaunch tests (retro_test.go:461, :485) run unchanged as the cross-package proof.
- The sidecar is written at the same four moments with the same argument VALUES (workspace = `cr.cs.WorkspacePath`, phase = `string(next)`, cycle, the same error — raw `err` at cyclerun_dispatch.go:322 and `firstErr` at evaluate_batch.go:183, wrapped `ferr`/`phaseErr` at :372 and quota_pause.go:34 — and the same attempt count); a passing cycle writes none; evaluate_batch writes ONE for the batch's first failing evaluator only (:163-165); resume_execution.go's post-dispatch aborts write none — all preserved.
- `core.ErrArtifactTimeout` is the same variable at the same address: no move, no re-export, no new declaration; errors_infra_test.go and bridge/engine_artifact_timeout_test.go pass unchanged; `TestInfraTeardownUnion_SpelledExactlyOnce` (infra_teardown_single_source_test.go:150-235) is unaffected because `isArtifactTimeout` contains no `||`/`&&` naming both sentinels.
- state.json, cycle-state.json, ledger.jsonl, phase-timing.json, `<phase>-usage.json`: untouched (the unit writes only the sidecar and reads nothing).
- stderr: the two reachable lines (:219, :223) are now rendered by the root's WARN `StderrSink` in the one-line format and also land in `signals.ndjson`; the marshal line (:213) disappears because it never could print; no other stderr text changes; nothing is added on the happy path. Grep evidence (2026-09-13): no test asserts the `[orchestrator] WARN failure-diag` text — the only `failure-diag` mentions in tests are failure messages and comments (orchestrator_timing_test.go:4, :149, :179; infra_teardown_timeout_only_test.go:49; failure_delivery_evidence_test.go:86, :90, :107); and no ACS predicate names any test this unit touches beyond the five that keep their names in `corePkg` (grep `TestAdoptStructuredFailure`/`TrustBoundary` in acs/ and cmd/: none — the only `TrustBoundary` hit is `TestProtectedSurfaceManifest_CoversExplanationTrustBoundary` in internal/guards, unrelated).

Risks:

- **The one call-site deviation** (ADR:37-39 read literally): the old seam is a package-level function that takes the clock as its last parameter and has no path to the orchestrator's Center; four production sites (cyclerun_dispatch.go:322, :372; quota_pause.go:34; evaluate_batch.go:183) and three test sites gain `cr.o.` / `o.` and lose the trailing `cr.o.now` (§9). Mitigation: argument values identical, the compiler catches every site, orchestrator_timing/transient integrations exercise the seam, and `grep -F 'writePhaseFailureDiag('` shows exactly four non-test callers.
- **Center-conditional vs unconditional** (01 review HIGH-1): the two replaced lines printed on every root; the signal prints only where a Center is wired. Every abort site runs under a cycleRun whose Orchestrator comes from `NewOrchestrator` at both production roots (`wireOrchestratorDeps`, `wireSimulateOrchestrator` — cmd_cycle_signal_center_test.go:23-30, :232-256); the only nil-Center root is the test-only `internal/routingtest/engine.go` pinned by allowlist (:52-54). Test 26 proves the new module tag renders at the `--simulate` root; test 22 proves a late Center is seen. A future root that builds an `Orchestrator` literal would lose the line silently — the nil-root pin is the defence.
- **Marshal-branch removal**: a reviewer may read the deleted `if merr != nil` as lost robustness; the argument (no error path for strings+ints; ADR:34-36) is in the code comment and here. If a float or map field is ever added to `Sidecar`, the branch must return with a code (unit 01's non-finite-float precedent, 01 doc:87). The byte-identity test catches any shape change.
- **AST pin relocation**: `TestTimeoutOnlySites_NotWidenedToUnion` parses `failure_learning.go` for `writePhaseFailureDiag` today; after the move it errors `function writePhaseFailureDiag not found` unless the rows are replaced in the SAME commit; acs/cycle1267/predicates_test.go:136-140 runs it by name in `corePkg`, so a missed relocation false-REDs that lane.
- **ACS name coupling**: the five core tests named by acs/cycle1267, acs/regression/cycle1270 and acs/cycle1562 keep their exact names in `./internal/core` and exercise the facade; moving them to the unit would false-RED those predicates.
- **Closed-set literals**: `TestModule_ClosedSet` asserts `len(got) != 23` (event_test.go:49) — forgetting the bump or the `knownModules` entry fails the tree, or worse leaves every unit event stamped `SIGNALCENTER_UNKNOWN_MODULE` (raised, never dropped); the kind must be added to both the const block and `knownKinds`. Forgetting the `init()` registration stamps `SIGNALCENTER_UNREGISTERED_CODE` at runtime — test 17 is the guard; registering with a different doc twice is a Conflict (`TestRegistryConflicts_RealRegistryIsClean`).
- **signal-codes.md is generated from the LINKED binary** (cmd_signals.go:1-6): run `make -C go build` before `evolve signals codes generate` from the project root, or `evolve signals codes check` (cmd_signals_test.go:17-21) reds CI on drift.
- **Gates**: `go/.cover-strict` += `./internal/core/failurediag 100` (after the `gatesignal` line); `go/.apicover-enforce` += `./internal/core/failurediag` under a dated `# ADR-0103 unit 02 (2026-09-13)` comment (after the `gatesignal` block); every export must be NAMED by a unit test or the enforce step reds.
- **Line count**: `failure_learning.go` goes 937 → ~826 (> the documented 800 DoD, .apicover-enforce:3-7, which the tooling does not enforce today — `grep 800 internal/apicover/*.go` is empty and `./internal/core` is enforced at 937); ADR:11 still quotes 1068 and design §12 row 1 says 1070 — this doc restates the current size; units 03 and the engine unit bring the file under 800.
- **The `sh -c 'exit 7'` subprocess** in test 10 is the one non-hermetic unit test (CI's POSIX runner has `sh`; an `*exec.ExitError` with a chosen code cannot be built portably by hand). It must not `t.Skip` — a skip drops coverage below 100 and fails `cover-strict`.
- **Doc drift fixed in the same PR**: runtime-reference.md:81 lists six fields (omits `delivery_failure`); phase-timing-and-diagnostics.md:120 says transient errors map to `80/85/86` — true only when the error IS an `*exec.ExitError`, which the bridge's wrapped launch errors are not (they map to 1); both point at this doc as the schema's single source. Do not copy 01 doc:100's phantom `TestModule_ClosedSetHasOutcome` (the real proof is the enumeration in `TestModule_ClosedSet`). Checked and unchanged: docs/architecture/artifact-backfill.md:34 ("falls through to write `<phase>-failure-diag.json` and abort as before" — still true) and agents/evolve-auditor.md:53 (a slug name only).
- **Preserved quirks are NOT fixed here** (each is a behaviour change with its own commit; a reviewer asking to "fix while here" is answered by the byte-identity guarantee): F3 empty workspace still writes `<phase>-failure-diag.json` CWD-relative (no guard ever existed here; unit 01's `OUTCOME_SIDECAR_SKIPPED` guard pre-existed there) → a `FAILUREDIAG_SIDECAR_SKIPPED` guard; F4 a failed temp write leaves `<path>.tmp` → `internal/atomicwrite`; F5 quota_pause.go:34's `exit_code` 1 vs the ledger's 85; F6 resume_execution.go writing no sidecar on its post-dispatch aborts and evaluate_batch's first-failure-only sidecar.
- **Follow-ups, in order**: F1 `FAILUREDIAG_DELIVERY_FAILURE` (spec in §5; add `TestWriter_Write_DiagnosisIsSignalledBeforeThePersistFailure` — event order on a failing write); F2 re-point the bridge TOKEN sites (stopreview.go:101 `artifactTimeoutMarker`, driver_tmux_wait_diagnostic.go:16 `artifactTimeoutSubmitWedged` — a typed const from the untyped leaf const compiles — attempt_telemetry.go:264) at the unit's consts with a drift pin `TestArtifactTimeoutContract_ProjectsTheClassifiersConstants`, rewriting the Sprintf at driver_tmux_wait_diagnostic.go:105-110 to concatenate the consts (it spells `cause=`/`reason=` as format text today, so the collapse is otherwise partial), and re-point core's two ledger `81`s (failure_hook.go:121, cyclerun_dispatch.go:242) at `failurediag.ExitCodeArtifactTimeout`; NEVER project bridge/exitcodes.go:31 (acs/cycle1580/predicates_test.go:98-122 requires the literal `= 81`). Then F3-F6.

- **Concurrency**: `failureDiag()`'s lazy `if o.diag == nil` cache is not goroutine-safe on a literal `Orchestrator`. It is safe in practice for the same reason `recorder()` is: `NewOrchestrator` builds the writer eagerly after the options loop, and the one concurrent dispatch path (`evaluate_batch.go`) writes the sidecar in the serial merge after `wg.Wait()`; a literal orchestrator is a single-goroutine test shape.

## 9. Migration

`failure_learning.go` loses lines 115-225 (the struct, the classifier, the two parsers, the writer) and the imports only they used (`os/exec`; `encoding/json`, `path/filepath`, `strconv` if nothing else in the file still needs them — the build decides). Core gains `errors.go`'s 3-line `isArtifactTimeout` beside `IsInfraTeardownError` (:89-91) and the new `failure_diag.go` (the accessor pair `failureDiag()`/`wiredFailureDiag()` mirroring `recorder()`/`wiredRecorder()` at failure_learning.go:918-937, the method facade, the exported `DeliveryFailureCause` facade); `Orchestrator` gains `diag` beside `outcome` (orchestrator.go:277) and `NewOrchestrator` constructs it eagerly after the options loop (orchestrator.go:862 sibling). The two facades keep the two seams: `core.DeliveryFailureCause(err)` is unchanged for retro.go:218 and the bridge binding tests (:32, :55, :93), and stays named by failure_delivery_evidence_test.go:65, :72, :100 so `./internal/core`'s apicover gate (.apicover-enforce:278) is satisfied.

THE ONE DEVIATION, stated per the "no call site changes" rule: the old seam `writePhaseFailureDiag(ws, phase, cycle, err, attempt, cr.o.now)` is package-level, takes the clock as a parameter and has no route to the orchestrator's Signal Center. Keeping that spelling byte-identical would force either a package-level `Writer` (a global — forbidden by explicit DI) or a per-call Writer with no Center (the two deleted stderr lines were unconditional; their signals would be lost — the HIGH-1 class of 01 doc:129-132). So the facade becomes an `Orchestrator` method, like `recordPhaseOutcome` on the previous line of every site, and the four production sites change mechanically: cyclerun_dispatch.go:322 and :372 `writePhaseFailureDiag(cr.cs.WorkspacePath, string(next), cr.cycle, err, attempt, cr.o.now)` → `cr.o.writePhaseFailureDiag(cr.cs.WorkspacePath, string(next), cr.cycle, err, attempt)`; quota_pause.go:34 and evaluate_batch.go:183 likewise. Every argument value is unchanged; each site already dereferences `cr.o` on the previous line (cr.o.recordPhaseOutcome) and reads the `cr.o.now` FIELD today, so a nil orchestrator already panics at these sites and no new nil path is introduced — which is also why `failureDiag()` has no nil-orchestrator branch (ADR:34-36; `recorder()` keeps one only because a composition test flushes a `cycleRun` with no orchestrator, 01 doc:143-147, and no such test reaches the diag). Three core test sites change the same way (infra_teardown_timeout_only_test.go:82, failure_delivery_evidence_test.go:82, failure_advisor_coverage_test.go:240) using `(&Orchestrator{now: now}).writePhaseFailureDiag(...)` — the literal orchestrator gets the Null-Object writer, exactly as phase_timing_composition_test.go:107-136 does for the recorder; failure_delivery_evidence_test.go:17's comment names `failurediag.Sidecar` instead of `phaseFailureDiag`.

Pin relocation in the same commit: the `{failure_learning.go, writePhaseFailureDiag}` row of `TestTimeoutOnlySites_NotWidenedToUnion` (:130) becomes the three rows of test 20; the ACS by-name predicates (acs/cycle1267:136-140, cycle1562:158-175, cycle1591:159-176) need no edit. The single-writer guard for the new exported writer is test 24 (the construction token, since `.Write(` is too common). The facade is removed only when the last caller migrates (ADR:52-53) — when the four abort sites move into the unit-05 `RunCycle` engine they may call `o.failureDiag().Write` directly; no plan to do so in units 03-04, so the seam is stable.

**Shipping.** The diff touches four files on the `ProtectedSurfaceManifest` (`orchestrator.go`, `cyclerun_dispatch.go`, `evaluate_batch.go`, `failure_learning.go` — `go/internal/guards/integrity_surface.go`), so `verifyNoControlPlaneEdits` refuses it as a `--class cycle` commit: it ships only through `evolve ship --class manual` from an operator session outside any cycle, like unit 01. The moved code's new homes — `go/internal/core/failure_diag.go` and `go/internal/core/failurediag/` — join the manifest with the same rationale as `failure_learning.go` (and unit 01's `go/internal/core/outcome/` joins beside them: the C1 chokepoint is integrity surface wherever it lives), so a cycle's builder cannot edit the sidecar writer through the leaf.

Signal Center, gates and docs land in the same PR: event.go const + `knownModules`/`knownKinds`, event_test.go enumerations + count 23 → 24, design §5.1 prose (:164-166) and §5.2 row (:171-185), §12 row 1 note (:462) marking unit 02 landed, ADR-0103 units table row 02 (:60) linking this doc, `.cover-strict` and `.apicover-enforce` entries, `signal-codes.md` regenerated from the rebuilt binary (`make -C go build && evolve signals codes generate` from the project root), phase-timing-and-diagnostics.md:108-138 and runtime-reference.md:81 pointing here. A §10 Verification (landed) section is appended at landing, as unit 01 did (01 doc:151-169), recording the build-confirmed mutants by name and the review folds.


## 10. Verification (landed)

Red first on every surface (the unit's tests compiled against the package before it existed;
the core tests failed on the missing facade and accessor; the closed-set tests on the missing
module and kind). `internal/core/failurediag` 100 % statements (`cover-strict`) and 15/15
exports (`apicover`); `internal/core` 348/348; `signal-codes.md` regenerated (85 codes).
The kept core tests (`TestWritePhaseFailureDiag_*`, `TestDeliveryFailureCause_*`,
`TestOrchestratorForensicsHelpersCoverage`, the RunCycle integrations) pass through the facade
with a receiver edit only; the timeout-only pin now reads `isArtifactTimeout` at its body and at
both injection sites; the `--simulate` root renders the new module tag on the console and in
`signals.ndjson`.

Twenty-six build-confirmed mutants killed by name: the exit-code timeout gate dropped; the
`*exec.ExitError` lift dropped; 81 → 1; the delivery cause not projected; `.UTC()` dropped;
`RFC3339Nano`; `exit_code`/`delivery_failure` swapped; the rename skipped; the temp-write warn
dropped; the rename warn dropped; mode 0600; WARN → INFO; the predicate gate dropped; the typed
gate inverted; the reason returned for the bare cause; the legacy path deleted; the escape walk
removed; the Center snapshotted at construction; the union gate at the writer's construction;
the union gate at the classifier facade; `==` instead of `errors.Is`; the facade emptied; the
accessor rebuilding on every call; the eager construction dropped; the kind left out of the
closed set; a second construction site in another package. One equivalent mutant is recorded
rather than padded: `HasPrefix(rest, ReasonField)` → `Contains` is made equivalent by the
`TrimPrefix` on the next line, which enforces the same position — the test that would
distinguish them cannot exist without changing the parser.

`failure_learning.go`: 937 → 826 lines (units 03 and the failure-learning engine bring it
under the 800 DoD).

Review folds (fleet: code-simplifier → architecture-reviewer ∥ go-reviewer; simplifier no edits,
architecture **Approve / MERGE** with two MEDIUMs, Go review PASS with two minors — every finding
folded). MEDIUM — the wire-token single source had no projection yet and its guard pinned only
itself: core's bare `81` ledger exit codes (`failure_hook.go`, `cyclerun_dispatch.go`, `retry_opts.go`) now project
from `failurediag.ExitCodeArtifactTimeout`, and the token test is named for what it pins
(`TestWireTokens_AreTheDecodersSpellings`; the bridge's producer copies stay follow-up F2).
MEDIUM — the timeout-only gate's body moved from a protected file (`failure_learning.go`) to an
unprotected one: `errors.go` (the Bridge port's sentinels and the integrity predicates on them)
joins the `ProtectedSurfaceManifest`, and the facade entry's rationale names the injection sites,
not the gate's body. Go review — the documented nil-gate panic is pinned by
`TestNewWriter_NilGatePanicsAtFirstUse`; the deleted marshal branch's comment stays the guard for
a future non-string field. The unit-suite floor also surfaced the pre-existing
`internal/phases/audit` deadline-kill timing flake (3/3 green in isolation; unit 02 does not touch
that package) — filed as a P2 regression item in the runtime inbox.
