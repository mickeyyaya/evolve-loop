# Comment history: `acs/cycle662`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle662/predicates_test.go:3` — above `package cycle662`

```text
// Package cycle662 materialises the cycle-662 acceptance criteria for the single
// triage-committed (`## top_n`) task: chronicle-s1-recurrence-index (weight 0.93).
//
// TASK BINDING (R9.3 — predicates bind ONLY to triage `## top_n` work):
//
//	triage-report.md commits exactly ONE task to this cycle:
//	  chronicle-s1-recurrence-index — C662_001..007
//	The scout report proposed echo-veto-wiring-completion, but triage `## top_n`
//	(the sole task authority) committed chronicle-s1-recurrence-index instead, so
//	the predicates bind to THAT. Every non-committed item gets ZERO predicates.
//
// FEATURE CONTEXT — cycle 661 (395d8b7e) landed go/internal/recurrence
// (Ledger/RecordClosure/Escalator/Autofiler/EscalationPolicy + escalateRetroReason
// in core + `evolve lessons recurrence`) but the core shipped UNWIRED. THREE gaps
// make it dormant, closed this cycle:
//
//	(G1) PRODUCTION WIRING  — RecordClosure has zero call sites; nothing writes
//	     .evolve/recurrence-ledger.json, Count()==0 forever. Wire it at the
//	     deterministic retro-closeout seam (writeDeterministicLearning).
//	(G2) HISTORICAL BACKFILL — the ledger starts empty; a two-shape-tolerant scan
//	     over .evolve/instincts/lessons/*.yaml seeds the 267-lesson history with a
//	     SkippedFiles diagnostic for malformed files.
//	(G3) GENERIC CLASSIFICATION — operator-reset(96)+loop-fatal(62)=59% noise;
//	     a per-pattern Generic flag (denylist + pattern==errorCategory echo) keeps
//	     escalation and the CLI report on NON-generic patterns only.
//
// PREDICATE QUALITY (cycle-85): every predicate EXERCISES the SUT. Each shells
// `go test -race -v -run <name>` against the real package and asserts the named
// TDD-authored behavioral test actually RAN and PASSED (the `--- PASS: <name>`
// marker) — a package that compiles but lacks the test prints "no tests to run"
// (exit 0) with NO marker, so a bare exit check would vacuously green. C662_007
// is the apicover config-check (source-level, waived).
//
// TEST-NAME CONTRACT — these behavioral tests are authored by the TDD engineer
// (RED now; Builder must NOT modify them, only add production code to green them):
//
//	internal/recurrence : TestC662_BackfillCountsRecurrenceAcrossLessonShapes
//	                      TestC662_BackfillSkipsMalformedYAMLWithoutError
//	                      TestC662_BackfillTaskBindingChainCountsAtLeastSix
//	                      TestC662_MarksClassificationEchoPatternsGeneric
//	internal/core       : TestC662_RetroCloseoutRecordsClosureInLedger
//	                      TestC662_EscalateRetroReasonIgnoresGenericPatterns
//	cmd/evolve          : TestC662_RenderExcludesGenericPatterns
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive : C662_001 wiring writes a ledger keyed by the FAIL cycle.
//   - Positive : C662_002 recurrence counted across both lesson shapes.
//   - Negative/Edge : C662_003 malformed YAML skipped-not-fatal (SkippedFiles).
//   - Replay : C662_004 the historical task-binding chain counts >= 6.
//   - Semantic : C662_005 echo/denylist patterns marked Generic; specific ones not.
//   - Semantic/Negative : C662_006 escalation ignores generic (both directions).
//   - Semantic : C662_007 CLI report excludes generic noise.
//   - Config : C662_008 new exported symbols graduated in .apicover-enforce.
```
