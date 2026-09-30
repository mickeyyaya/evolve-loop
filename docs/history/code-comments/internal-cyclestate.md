# Comment history: `internal/cyclestate`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/cyclestate/classification_test.go:5` — above `func TestClassificationMidExecutionFail_IsTheLegacyDefaultSpelling(t *testing.T) {`

```text
// ADR-0103 unit 03b: the supervisor's default class for a phase that failed
// mid-cycle with no self-report has ONE spelling — the failure-learning engine
// projects it into the FailedRecord and the lesson event, recurrence's generic
// denylist names it. The literal is the on-disk contract of every legacy record.
```

### `go/internal/cyclestate/diagnostic_code_test.go:3` — above `import (`

```text
// diagnostic_code_test.go — Diagnostic.Code: the C1 record's machine-readable
// reason. A phase's deterministic gate (triage refusing a protected-surface
// card) stamps a stable code beside its prose so the closeout and the
// classifier can act on the CLASS of failure without regexing the sentence
// (docs/incidents/2026-09-14-triage-refusal-poison-loop.md).
```

### `go/internal/cyclestate/diagnostic_test.go:8` — above `func TestErrorMessages_ProjectsOnlyErrorSeverityInOrder(t *testing.T) {`

```text
// ErrorMessages is the ONE projection from a phase's diagnostics to its reasons
// for a FAIL verdict: error-severity messages, in order; warnings are a trail,
// never a reason; nil in, nil out. core (the FailedRecord, the floor fail
// reasons, the chokepoint log line, the seal's backfill, judgment lessons) and
// cyclehealth all call it, so the rule cannot fork (cycles 1634/1636).
```

### `go/internal/cyclestate/result.go:31` — above `VerdictsNotAdopted []VerdictNotAdopted`

```text
// VerdictsNotAdopted records non-floor phases (retrospective, memo, the
// *-scans, router/advisor) that RAN and returned non-PASS AFTER a floor-derived
// FinalVerdict was recorded, so their verdict was PREVENTED from overwriting it
// (cycle-802, retro-bridge-timeout-width10). Without this a retro FAIL under
// quota/timeout pressure clobbered an audit PASS and zeroed the wave. The
// outcome is preserved here (never silently dropped) and surfaced in the cycle
// dossier as phases_run_verdict_not_adopted — a name that cannot be misread as
// "this phase did not run".
```

### `go/internal/cyclestate/result.go:40` — above `SystemFailure *SystemFailureSignal`

```text
// SystemFailure, when non-nil, marks that this cycle's failure was
// classified as SYSTEM-level (ADR-0072): the pipeline itself — not the
// task's code — is the cause (verdict-incoherence, infra-systemic,
// non-progress). The batch loop reads it to HALT + escalate instead of
// re-selecting the same inbox task. Nil ⇒ an ordinary task-level outcome
// (never-stop: retry/defer/quarantine as usual).
```

### `go/internal/cyclestate/result.go:47` — above `Remediations []string`

```text
// Remediations records graduated fix-forward rounds (operator directive
// 2026-07-21): each entry is "<gate>: round N -> <verdict>" for a
// deterministic gate that FAILed, received one bounded builder fix, and
// was re-run. Provenance only — the re-run verdict is what recorded; a
// remediated cycle is never a silent PASS.
```

### `go/internal/cyclestate/result.go:53` — above `SpineFailOpens []SpineFailOpen`

```text
// SpineFailOpens records every spine-gate fail-open this cycle took: the
// gate found a mandatory predecessor's handoff artifact missing and
// proceeded anyway (SpineFloor below enforce, or a non-clean absence).
// Before cycle-1166 these went to stderr and nowhere else — a width-3 batch
// emitted 76 of them with no counter, no dossier field and no threshold.
// Occurrences ACCUMULATE (never collapse repeats): the count IS the signal.
```

### `go/internal/cyclestate/result.go:60` — above `FailReasons []string`

```text
// FailReasons surfaces the floor-override explanations (the untruncated
// audit-fail-reason.json / CycleState.AuditFailReasons content) in the
// cycle summary and dossier — cycle-1022's lesson: the reason WAS recorded
// on disk while every operator-facing surface stayed silent.
```

### `go/internal/cyclestate/result.go:67` — above `type SystemFailureSignal struct {`

```text
// SystemFailureSignal records a system-level failure classification (ADR-0072).
// Category is the failure_policy category (e.g. "verdict-incoherence"); Halt is
// true when the Go floor mandates a loop halt regardless of orchestrator
// judgment. Evidence is the deterministic proof (e.g. the coherence signal).
```

### `go/internal/cyclestate/result.go:86` — above `type VerdictNotAdopted struct {`

```text
// VerdictNotAdopted is one phase that RAN to completion whose non-PASS verdict was
// NOT adopted as the cycle verdict (the cycle-802 floor guard: a post-verdict
// non-floor phase may not clobber a floor-derived FinalVerdict). Verdict is what
// the phase actually returned (FAIL|WARN|SKIPPED) — the value the old
// SkippedPhase.Reason carried, under a name that no longer implies a skip.
```

### `go/internal/cyclestate/result.go:128` — above `const (`

```text
// The triage gate's refusal codes — the reasons triage.Classify itself FAILs a
// cycle (not the agent's verdict). TRIAGE_PROTECTED_SURFACE is the one that is
// deterministic AND operator-owned: the FAIL closeout routes its Subject to
// console-manual on the first hit (docs/incidents/2026-09-14-triage-refusal-poison-loop.md).
```

### `go/internal/cyclestate/result.go:146` — above `func ErrorMessages(diags []Diagnostic) []string {`

```text
// ErrorMessages is the ONE projection from diagnostics to a FAIL's reasons:
// the error-severity messages, in order (nil when there are none). core's
// FailedRecord, floor fail reasons, chokepoint log line, seal backfill and
// judgment lessons, and cyclehealth's outcome detail, all call it — the rule
// lives beside the vocabulary so no reader can re-derive it differently
// (cycles 1634/1636).
```

### `go/internal/cyclestate/result.go:178` — above `type Disposition struct {`

```text
// Disposition is whose fault a coded refusal is — the ONE place that answers
// it, beside the vocabulary (a fourth code added here decides its own fate):
// TaskLevel charges the item's failure_count toward the ADR-0072 S5 ceiling;
// RouteConsole hands the refused Subject to the operator on the first hit.
```

### `go/internal/cyclestate/result_test.go:9` — above `func TestSystemFailureSignal_Wire(t *testing.T) {`

```text
// TestSystemFailureSignal_Wire pins the ADR-0072 system-failure signal JSON
// shape (serialized into the escalation dossier) and names the type (apicover).
```

### `go/internal/cyclestate/result_test.go:70` — above `func TestVerdictNotAdopted_Wire(t *testing.T) {`

```text
// TestVerdictNotAdopted_Wire pins the ran-but-declined record (cycle-802
// retro-bridge-timeout-width10 guard): a non-floor phase's non-PASS verdict is
// preserved in the cycle dossier instead of clobbering a floor-derived
// FinalVerdict. It is DISTINCT from SkippedPhase — the field names the phase's
// VERDICT, so the dossier can never claim retro was skipped on a cycle where retro
// ran (dossier-retro-skipped-mislabel). The tags are the dossier contract, so a
// drift here silently drops the audit trail.
```

### `go/internal/cyclestate/state.go:86` — above `CyclesUnpicked int 'json:"cycles_unpicked"'`

```text
// Deprecated (ADR-0072 S5): this counter was only ever written as 0 and
// never incremented in the real cycle-failure path — a dead field. The
// single source of truth for per-task failure memory is now the inbox item
// JSON's own "failure_count", bumped and consulted by
// inboxmover.ApplyCycleOutcome's FAIL drain. Kept for wire-compat with
// existing state.json blobs; do not add new reads.
```

### `go/internal/cyclestate/state.go:109` — above `Shipped bool 'json:"shipped,omitempty"'`

```text
// Shipped is the cycle's own ship latch: set by both dispatch roots when the
// ship phase PASSes and survives the deliverable review, never inferred from
// main HEAD movement (a sibling lane moves HEAD too — cycle 1630). Persisted
// so a pause/resume after ship keeps the fact; the outcome label
// (SHIPPED_VIA_BUILD) and the post-ship observer degrade both read it.
// omitempty: pre-latch checkpoints decode/encode unchanged.
```

### `go/internal/cyclestate/state.go:133` — above `WorktreeBaseSHA string 'json:"worktree_base_sha,omitempty"'`

```text
// WorktreeBaseSHA is the per-cycle worktree HEAD at creation == the cycle
// base. Persisted so the crash-resume path (RunCycleFromPhase) can run the
// cycle-156 build-commit normalize, which RunCycle previously drove from a
// run-local variable. Empty (omitted) for pre-field checkpoints and
// worktree-less cycles → the normalize degrades to a no-op.
```

### `go/internal/cyclestate/state.go:160` — above `AuditDispatches int 'json:"audit_dispatches,omitempty"'`

```text
// AuditDispatches counts audit DISPATCHES (not completions) this cycle. It
// is the round-supersession index for retiring the previous audit round's
// verdict artifacts (cycle-1603): CompletedPhases records only successes,
// so an audit that crashed or quota-paused mid-flight after the auditor
// pre-wrote acs-verdict.json would be invisible to a completion-derived
// index and its dead attempt's verdict would be honored on resume.
// Incremented in the audit pre-dispatch block BEFORE the pre-phase
// cycle-state write, so an interrupted round has already persisted its
// dispatch and the resumed re-dispatch retires it. Additive omitempty:
// pre-field checkpoints decode as 0 and the completion-derived count
// backstops them (supersedePreviousAuditRound).
```

### `go/internal/cyclestate/state.go:185` — above `AuditFailReasons []string 'json:"audit_fail_reasons,omitempty"'`

```text
// AuditFailReasons: the error-severity diagnostics behind an audit FAIL
// verdict recorded by the runner's OWN gates (set in-process at the
// recordFloorVerdictFailure chokepoint; cleared on every audit re-dispatch).
// The ADR-0072 coherence floor reads THIS field — orchestrator memory, never
// an agent-writable workspace file — to tell a DIAGNOSED gate-downgrade
// (coherent task-FAIL → retro + continue) from an unexplained forged verdict
// (halt). Additive omitempty; persisted so a crash between the verdict
// record and cycle finalization resumes without a false halt (the resume
// path already trusts cycle-state.json wholesale).
```

### `go/internal/cyclestate/state.go:195` — above `ShipFailReasons []string 'json:"ship_fail_reasons,omitempty"'`

```text
// ShipFailReasons: the SHIP-phase's explained-failure carrier, the
// post-audit twin of AuditFailReasons (pipeline-defect-pipeline-blocker
// Task 1, cycle-1329). Set at the ship error-record chokepoint
// (recordFailureLearning, fl.Failed==PhaseShip) from the orchestrator's
// own in-process record of the ship error — never a workspace file, to
// preserve the same trust boundary AuditFailReasons documents above —
// and cleared on ship re-dispatch (resetFloorFailReason). The ADR-0072
// coherence floor folds this in alongside AuditFailReasons so a green
// audit + green ACS cycle that legitimately fails at ship (e.g. a
// repo-contract-gate rejection) is diagnosed as a coherent task failure,
// not misclassified as a forged verdict. Additive omitempty; persisted so
// a crash between the ship error record and cycle finalization resumes
// without a false halt.
```

### `go/internal/cyclestate/state.go:209` — above `FailedAt []FailedRecord 'json:"failed_at,omitempty"'`

```text
// FailedAt: the cycle's failure history (mirrors State.FailedAt), carried on
// the per-cycle checkpoint so the ADR-0072 S4 evidence dossier can compose
// its non-progress counters (same-class recurrence, repeat count) from
// independent evidence at the retro-decision chokepoint. Additive omitempty;
// pre-S4 checkpoints decode/encode unchanged.
```

### `go/internal/cyclestate/verdict.go:3` — above `const (`

```text
// Verdict constants — the four outcomes a phase may emit. These match
// the EGPS gate vocabulary (CLAUDE.md env-var table: WARN removed at
// v10.0.0 but still accepted by Audit for the pre-EGPS soft-start
// boundary; SKIPPED used when a phase opted out, e.g. EVOLVE_TRIAGE_DISABLE).
```

### `go/internal/cyclestate/verdict.go:14` — above `const ClassificationMidExecutionFail = "cycle-mid-execution-fail"`

```text
// ClassificationMidExecutionFail is the supervisor's default class for a phase
// that failed mid-cycle with no self-report. Deliberately OUTSIDE failurelog's
// taxonomy: NormalizeLegacy maps it to UnknownClassification, so a record
// carrying it ages out on the one-day legacy bucket (failurelog.LegacyEffectiveTTL)
// — an operator decision (ADR-0103 unit 03b, F11). Projected by the
// failure-learning engine (the FailedRecord and the lesson event) and by
// recurrence's generic-pattern denylist; every other spelling is data.
```
