# Comment history: `internal/core/failurelearning`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero D4

### `go/internal/core/failurelearning/engine_test.go:138` — above `func TestAdoptStructuredFailure_TrustBoundary(t *testing.T) {`

```text
// Test 16 — the ADR-0039 §7 trust boundary (moved verbatim from core).
```

### `go/internal/core/failurelearning/failurelearning.go:1` — above `package failurelearning`

```text
// Package failurelearning is unit 03b of the component breakdown (ADR-0103):
// the deterministic half of learning from a failed phase. One Engine owns the
// failed-approach recorder (the FailedRecord and the P0 todo minted into the
// State through the unit-03 Lifecycle), the deterministic floor (the
// retrospective report, the lesson YAML and the inbox remediation items
// faillearn renders when no LLM retrospective can), and the recurrence closure.
// The orchestration around it — the retro runner, the observer, the cycle-state
// writes, the ledger, the checkpoint, the disposition gate and the persist —
// stays in core (unit 05's RunCycle engine). The Engine holds a clock, the
// Lifecycle and the Signal Center accessor; it never persists, never writes
// stderr, and reports its four failure modes as failurelearning.warning under
// module failurelearning. Design: docs/architecture/decomposition/03b-failure-learning-engine.md.
```

### `go/internal/core/failurelearning/floor.go:13` — above `func (e *Engine) WriteFloor(f Failure, l Learned) {`

```text
// WriteFloor is the failure floor (inbox retro-always-invariant, gap 1 /
// cycle-243): when the LLM retro cannot run or returns a non-canonical
// verdict, render the learning artifacts deterministically —
// retrospective-report.md in the cycle workspace, the failure-lesson YAML and
// the inbox remediation items — so the lesson survives instead of degrading to
// a stderr line. Best-effort: a write failure is one WARN and never masks the
// original phase failure; the recurrence closure runs LAST, unconditionally.
```

### `go/internal/core/failurelearning/floor.go:48` — above `func floorEvent(f Failure, l Learned, now time.Time) faillearn.FailureEvent {`

```text
// floorEvent projects the failure and what was learned onto the lesson event:
// the supervisor's defaults, then — ADR-0039 §7 — the failed phase's own
// structured block (validated and capped once by the recorder) for the class,
// the defects and the evidence, with the workspace appended LAST.
```

### `go/internal/core/failurelearning/floor_test.go:297` — above `func TestRemediationItems_IDsAreInjectiveOverTheFullTitle(t *testing.T) {`

```text
// Test 25 — the cycle-1285 premise: ids are injective over the FULL title.
```

### `go/internal/core/failurelearning/importgraph_test.go:3` — above `import (`

```text
// importgraph_test.go — the package is a leaf under core (ADR-0103 unit 03b
// §2): stdlib plus the nine named internal packages, never internal/core
// itself (the compiler is the cycle guard; this is the leaf-ness declaration —
// signalcenter/importgraph_test.go idiom).
```

### `go/internal/core/failurelearning/limits_test.go:3` — above `import (`

```text
// limits_test.go — the clean-code limits the design promises (ADR-0103 unit
// 03b §4), enforced by a test rather than by review: every function < 50
// lines, nesting depth ≤ 4, every file < 800 lines (signalcenter/limits_test.go
// idiom; comments inside a function count, its doc comment does not).
```

### `go/internal/core/failurelearning/record.go:40` — above `structured := adoptStructuredFailure(f.Workspace, string(f.Phase))`

```text
// ADR-0039 §7: a phase healthy enough to self-report owns its failure
// description — its structured block beats the supervisor's synthesis.
// Read ONCE here and threaded to the floor, so state.json and the lesson
// artifacts can never diverge on the same failure event.
```

### `go/internal/core/failurelearning/record.go:63` — above `func adoptStructuredFailure(workspace, phase string) *phasecontract.FailureBlock {`

```text
// adoptStructuredFailure is the trust boundary for agent-written failure
// blocks (ADR-0039 §7): adopt the failed phase's self-report ONLY when its
// class normalizes into the canonical taxonomy (never blind trust — an
// out-of-taxonomy class would round-trip to UnknownClassification on the next
// state read), and cap list/entry sizes so a misbehaving agent cannot bloat
// state.json or the lesson corpus.
```

### `go/internal/core/failurelearning/recurrence.go:11` — above `func (e *Engine) recurrenceClosure(f Failure, pattern string) {`

```text
// recurrenceClosure is gap-G1 production wiring (cycle-662): the deterministic
// retro-closeout seam upserts the failing lesson pattern into the recurrence
// ledger keyed by the failing cycle, so Count() reflects real history instead
// of staying 0 forever. Escalator/Autofiler are nil here — escalation APPLY
// stays boundary-only (the live consult site reads the ledger; it must not
// race inboxmover.Claim from the mid-cycle closeout). Best-effort: a ledger
// failure is one WARN naming the failing call (op = load | record | save) and
// never masks the phase failure; op=record is dormant today (RecordClosure
// errs only through the nil-guarded escalator/autofiler) and reported the day
// it is not.
```

### `go/internal/core/failurelearning/remediation.go:15` — above `const (`

```text
// remediationSlugMaxRunes bounds the id's derived tail so a long defect line
// cannot produce an unwieldy filename. remediationMaxItems and
// remediationTitleMaxRunes bound the queue itself (cycle-1282 DEF-6).
```

### `go/internal/core/failurelearning/remediation.go:24` — above `func (e *Engine) remediationItems(f Failure, defects []string, weight float64) []faillearn.InboxItem {`

```text
// remediationItems turns a failed phase's self-reported defects into inbox
// remediation todos (batch-integrity-review-2026-08-04.md F1(ii)): the 1255
// defect was a retrospective that "filed" two items which never reached the
// queue. Ids are stable per (cycle, defect) so a re-run of the floor is
// idempotent (faillearn's writeIfAbsent keeps the first write). The weight is
// the caller's ONE policy read, never a literal here.
```

### `go/internal/core/failurelearning/remediation.go:33` — above `if len(items) >= remediationMaxItems {`

```text
// cycle-1282 DEF-6: defects[] and each line are agent-authored and
// previously unbounded — a queue nobody can triage hides the real item.
// Both caps are RECORDED, never silent.
```

### `go/internal/core/failurelearning/remediation.go:62` — above `func remediationFingerprint(title string) string {`

```text
// remediationFingerprint is the id's injective tail: a short digest of the
// FULL defect title, appended unconditionally (cycle-1285 F1: the slug is
// lossy twice over — it stops at remediationSlugMaxRunes and collapses every
// run of non-alphanumerics — so two defects diverging only after rune 60 or
// only in punctuation minted ONE id, and the DEF-4 collision refusal then
// suppressed the retrospective and the lesson with the second item).
```
