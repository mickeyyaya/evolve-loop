# Comment history: `internal/cyclehealth`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero h1

### `go/internal/cyclehealth/cyclehealth_cycle188_test.go:13` — above `func writeStopReviewLedger(t *testing.T, ws string, cycle int, action string) {`

```text
// cyclehealth_cycle188_test.go — RED contracts for cycle-188.
//
// Task 1 (stop-review-ledger-trail) READ side: checkSelfHealEvents must treat
// a kind=stop_review ledger entry with action=pause as a WARN anomaly (a phase
// the reviewer paused for investigation), while action=extend is healthy and
// emits nothing.
//
// Task 2 (per-phase-latency-ceiling) was superseded by the policy-backed global
// ceiling. These tests now pin the explicit Options input and default fallback.
```

### `go/internal/cyclehealth/outcome.go:1` — above `package cyclehealth`

```text
// outcome.go — R6: the cycle-outcome SLO classifier (concurrency-factory
// plan), the measurement instrument for the EVOLVE_PHASE_RECOVERY soak.
//
// "Every cycle delivers a result" is only auditable if every ending is
// classified. The inputs are the ADR-0044 C1 records every terminal path
// already writes — phase-timing.json (the append-merge dispatch log) and
// interaction-summary.json (the I1 rollup) — so the classifier needs no new
// instrumentation and works on any historical run dir.
```

### `go/internal/cyclehealth/outcome.go:39` — above `OutcomeDeferred Outcome = "DEFERRED"`

```text
// OutcomeDeferred: the dispatch seam detected all-CLI-families quota
// exhaustion (every attempt exit=85, cycle-656), wrote a quota-likely
// checkpoint, and aborted resumable. Not a failure of the change under
// test — the work is preserved for `evolve loop --resume`.
```

### `go/internal/cyclehealth/outcome.go:52` — above `type outcomeRollup struct {`

```text
// The C1 record is read through phasetiming.Entry — its one schema home
// (cyclehealth must not import core, and does not need to: phasetiming is a
// leaf over cyclestate). A hand-typed mirror here silently parsed a renamed
// key to nil and reverted the detail to verdict-only with every test green
// (architecture review of cycles 1634/1636's fix).
```

### `go/internal/cyclehealth/outcome.go:84` — above `for _, e := range timing {`

```text
// DEFERRED: the abort reason carries the all-families quota-exhaustion
// prefix (cycle-656) — checked BEFORE the generic explained-failure arm
// so a quota defer is never paged as a failed cycle.
```

### `go/internal/cyclehealth/outcome.go:145` — above `for _, e := range timing {`

```text
// FAILED_EXPLAINED: a phase recorded verdict FAIL. The audit-FAIL →
// retro → end terminal is a normal COMPLETION, not an abort — the C1
// chokepoint never fires and no abort_reason exists — but the recorded
// verdict is the explanation (cycle-306, soak #3: a legitimate audit
// FAIL paged as UNEXPLAINED). Scanned in timing order so the detail
// names the first failing phase, which is the causal one.
```

### `go/internal/cyclehealth/outcome_deferred_test.go:16` — above `func TestClassifyOutcome_Deferred(t *testing.T) {`

```text
// Cycle-656: an all-families quota abort must classify DEFERRED, never as a
// failed cycle — and must not disturb the existing FAILED_EXPLAINED arm.
```

### `go/internal/cyclehealth/outcome_diagnostics_test.go:10` — above `func TestClassifyOutcome_FailVerdictDetailNamesThePhaseOwnReason(t *testing.T) {`

```text
// A phase that recorded verdict FAIL with its own error-severity diagnostics
// (cycles 1634/1636: triage's protected-surface admission rejection) is
// FAILED_EXPLAINED — it already was, by the verdict — but the detail must name
// the phase's reason so the batch report says WHY, not merely THAT. Warnings
// are not reasons; without an error-severity diagnostic the detail keeps its
// verdict-only wording.
```

### `go/internal/cyclehealth/outcome_test.go:1` — above `package cyclehealth`

```text
// outcome_test.go — R6 (concurrency-factory plan): the cycle-outcome SLO
// classifier, computed from the ADR-0044 C1 records every terminal path
// already writes (phase-timing.json + <phase>-usage.json sidecars +
// interaction-summary.json). This is the measurement instrument for the
// R8 shadow→enforce soak: SHIPPED / SALVAGED / FAILED_EXPLAINED are the
// only acceptable cycle endings; FAILED_UNEXPLAINED must be 0 and alarmed.
//
// Precedence (first match wins): SHIPPED (ship verdict PASS) → SALVAGED
// (no ship, but a correction-ladder salvage produced the artifact) →
// FAILED_EXPLAINED (an abort_reason was recorded — the C1 chokepoint
// guarantees abort paths record one) → FAILED_EXPLAINED (a phase recorded
// verdict FAIL with no abort — the audit-FAIL completion terminal,
// cycle-306) → FAILED_UNEXPLAINED (phases ran, nothing explains the
// ending — the bucket that pages someone).
```

### `go/internal/cyclehealth/outcome_test.go:60` — above `name: "failed_explained_via_abort_reason",`

```text
// cycle-283 shape: build ran, audit FAILed, abort recorded.
```

### `go/internal/cyclehealth/outcome_test.go:82` — above `name: "failed_explained_via_fail_verdict_without_abort",`

```text
// cycle-306 shape (soak #3, 2026-06-13): the audit-FAIL → retro →
// normal-completion terminal records the FAIL verdict but NO
// abort_reason — it is a completion, not an abort, so the C1
// chokepoint never fires. A recorded FAIL verdict IS the
// explanation; this must not page anyone as UNEXPLAINED
// (auto-filed defect auto-unexplained-outcome-cycle-306).
```
