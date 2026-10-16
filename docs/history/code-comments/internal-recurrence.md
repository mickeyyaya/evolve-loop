# Comment history: `internal/recurrence`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero D1

### `go/internal/recurrence/apply.go:103` — above `if prev, ok := stamp[key]; ok && prev == opts.Cycle {`

```text
// Presence-checked: a missing key reads as the map zero value, and
// fleet boundaries pass the 0-based pool iteration as Cycle — a bare
// `stamp[key] == opts.Cycle` silently skipped every never-applied
// intent on iteration 0 (2026-08-10 enforce-flip swallow).
```

### `go/internal/recurrence/apply_cycle_zero_test.go:11` — above `func TestApplyBoundary_CycleZeroAppliesStagedIntents(t *testing.T) {`

```text
// TestApplyBoundary_CycleZeroAppliesStagedIntents pins the 2026-08-10
// iteration-0 swallow: the per-cycle idempotency stamp compared
// `stamp[key] == opts.Cycle`, and a NEVER-applied key reads as the map zero
// value 0 — so the fleet boundary passes (cmd_loop.go dispatches with the
// 0-based pool iteration index as Cycle) silently skipped EVERY staged intent
// on iteration 0. Observed live the day failure_disposition.stage flipped to
// enforce: apply report {bumped:null,filed:null,skipped:null,shadow:false,
// cycle:0}, stamp {} — 5 intents pending since cycles 1225-1378, zero applied.
```

### `go/internal/recurrence/apply_cycle_zero_test.go:34` — above `second, err := recurrence.ApplyBoundary(opts(root, 0, false))`

```text
// Idempotency must still hold AT cycle 0: a second pass in the same
// iteration changes nothing. Skipped must be empty too — a stamp skip is a
// silent continue, while a stamp-BYPASSING pass records the id in Skipped
// via the never-lower branch, which would mask a dead stamp guard here.
```

### `go/internal/recurrence/backfill.go:3` — above `import (`

```text
// backfill.go — cycle-662 gap G2 (HISTORICAL BACKFILL). Cycle 661 landed the
// ledger core but it starts EMPTY, so Count()==0 for every historical pattern and
// the recurrence signal is dormant. BackfillFromLessons seeds a fresh ledger from
// the on-disk lesson corpus (.evolve/instincts/lessons/*.yaml), tolerating both
// the deterministic-floor and richer LLM lesson shapes, so the historical
// recurrence chains the advisory channel merely noticed become first-class counts.
```

### `go/internal/recurrence/backfill_c662_test.go:3` — above `import (`

```text
// backfill_c662_test.go — cycle-662 RED contract for chronicle-s1-recurrence-index
// gap G2 (HISTORICAL BACKFILL). Cycle 661 (395d8b7e) landed the ledger core but
// it starts EMPTY, so Count()==0 for every historical pattern and the recurrence
// signal is dormant. This file pins the backfill scan over
// .evolve/instincts/lessons/*.yaml that seeds the ledger with the 267-lesson
// history.
//
// Builder contract (do NOT modify this file — implement production code):
//
//	func BackfillFromLessons(lessonsDir string, pol EscalationPolicy) (*Ledger, []string, error)
//	    - scans lessonsDir/*.yaml, tolerating BOTH lesson shapes:
//        deterministic-floor (one-element list, pattern==errorCategory) AND the
//        richer LLM shape (description/governingVariable/preventiveAction blocks);
//	    - records each (pattern, cycle) closure into a fresh ledger, cycle parsed
//	      from the `cycle-<N>-` id prefix; recurrence count == distinct cycles;
//	    - a malformed/unparseable file is SKIPPED (its basename returned in the
//	      second result) and never aborts the scan — err is non-nil only for a
//	      fatal I/O error (dir unreadable), not per-file parse failures.
//
// RED today: BackfillFromLessons does not exist, so package recurrence fails to
// compile and none of these tests run. GREEN once Builder adds backfill.go.
```

### `go/internal/recurrence/backfill_c662_test.go:65` — above `func TestC662_BackfillCountsRecurrenceAcrossLessonShapes(t *testing.T) {`

```text
// TestC662_BackfillCountsRecurrenceAcrossLessonShapes — AC2a. The parser must
// tolerate BOTH lesson shapes and accumulate a per-pattern recurrence count
// across distinct cycles. Two LLM-shape files sharing one pattern (cycles 100,
// 101) must count 2; a floor-shape file (cycle 102) must also parse and count 1.
```

### `go/internal/recurrence/digest_test.go:3` — above `import (`

```text
// TDD contract for chronicle-s2-digest-writer (cycle 702). These tests define
// the WriteDigest API before digest.go exists — RED = compile failure, then
// assertion failures. Builder implements digest.go to turn them GREEN without
// modifying this file.
//
// Contract under test (inbox 2026-07-10T11-02-00Z-chronicle-s2-digest-writer):
//
//	WriteDigest(workspacePath string, in DigestInput, cfg DigestConfig) error
//	  -> writes <workspacePath>/recent-outcomes.md
//	DigestInput{Dossiers []dossier.Dossier, FailedApproaches []failureadapter.Entry, Index *Ledger}
//	DigestConfig{TokenBudget int, Cycles int}
//
// Rendering rules pinned here: newest-first one line per cycle; every rendered
// value sanitized (\n \r \t collapsed — LLM-authored text is an injection
// channel); truncation at cfg.TokenBudget via the local len/4 estimator;
// generic recurrence patterns roll up to ONE aggregate line; empty history
// writes no file.
```

### `go/internal/recurrence/generic.go:3` — above `import (`

```text
// generic.go — cycle-662 gap G3 (GENERIC CLASSIFICATION de-noise). A per-pattern
// Generic flag separates classification-vocabulary noise (operator-reset x96,
// loop-fatal x62 — 59% of the corpus) from specific-defect signal so escalation
// and the CLI report act on actionable patterns only. Backfill sets Entry.Generic
// via IsGeneric; consumers read the stored flag via IsGenericPattern.
```

### `go/internal/recurrence/generic_c662_test.go:3` — above `import "testing"`

```text
// generic_c662_test.go — cycle-662 RED contract for chronicle-s1-recurrence-index
// gap G3 (GENERIC CLASSIFICATION de-noise). operator-reset (x96) + loop-fatal
// (x62) are 59% of the 267-lesson corpus; without a generic filter they dominate
// the counts and spam escalation (plan failure-mode 2). This file pins the
// per-pattern Generic classification.
//
// Builder contract (do NOT modify this file — implement production code):
//
//	Entry.Generic bool                                   // json:"generic,omitempty"
//	func IsGeneric(pattern, errorCategory string) bool   // denylist + echo rule
//	func (l *Ledger) IsGenericPattern(pattern string) bool
//
//	Generic == true when the pattern is classification-vocabulary noise: either it
//	is on the denylist (operator-reset, loop-fatal, ...) OR it verbatim echoes the
//	lesson's failureContext.errorCategory. Backfill sets Entry.Generic via
//	IsGeneric; IsGenericPattern reads the stored flag (false for unseen patterns).
//
// RED today: Entry.Generic / IsGeneric / IsGenericPattern do not exist, so
// package recurrence fails to compile. GREEN once Builder adds generic.go.
```

### `go/internal/recurrence/ledger_test.go:45` — above `led := &Ledger{Entries: map[string]*Entry{`

```text
// Seed prior history (cycles 100, 200) so this cycle's close reaches count 3.
```

### `go/internal/recurrence/ledger_test.go:54` — above `for i := 0; i < 3; i++ {`

```text
// Cycle 300's closeout runs three times (idempotent re-invocation).
```

### `go/internal/recurrence/mid_execution_fail_test.go:10` — above `func TestRecurrence_DenylistNamesTheEnginesDefaultClass(t *testing.T) {`

```text
// ADR-0103 unit 03b: the generic-pattern denylist is a CONSUMER of the
// engine's default class — the key is the projected constant, so a re-spelling
// on either side turns this red instead of silently promoting the class into
// the recurrence counts.
```
