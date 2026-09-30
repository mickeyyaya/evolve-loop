# Comment history: `acs/cycle661`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle661/predicates_test.go:3` — above `package cycle661`

```text
// Package cycle661 materialises the cycle-661 acceptance criteria for the single
// triage-committed (`## top_n`) task: recurrence-ledger-weight-escalation.
//
// TASK BINDING (R9.3 — predicates bind ONLY to triage `## top_n` work):
//
//	triage-report.md commits exactly ONE task to this cycle:
//	  recurrence-ledger-weight-escalation (weight 0.93) — C661_001..005
//	Every `## deferred` item (chronicle-s1-recurrence-index,
//	echo-veto-wiring-completion, new-package-graduation-buildentry-gate) gets
//	ZERO predicates here.
//
// FEATURE CONTEXT
//
//	The system NOTICED every recurrence (retros wrote "6th occurrence,
//	confidence 0.97" chains) but noticing lived in an advisory channel with no
//	write access to the priority queue. This cycle mints a deterministic
//	internal/recurrence ledger:
//	  (1) LEDGER   — on retro closeout, upsert {pattern_key -> cycles[], count,
//	      last_seen, fix_item_id, fix_landed_sha} into .evolve/recurrence-ledger.json
//	      (flock + atomic write).
//	  (2) ESCALATION — a pure policy formula bumps the linked OPEN inbox item's
//	      weight (min(0.99, base + 0.03*(count-1))), idempotent per cycle; when
//	      no open item exists the pattern is handed to the autofile seam once.
//	  (3) RETRO-DECISION PARITY — RetroDecision consults the ledger and must NOT
//	      emit bare "proceed" while the current lesson pattern has count>=2.
//	  (4) `evolve lessons recurrence` CLI report (patterns by count + fix status).
//	internal/recurrence is a NEW leaf package and MUST be graduated into
//	go/.apicover-enforce in the same commit (new-package-graduation class,
//	4th recurrence: 575/587/652/661).
//
// PREDICATE QUALITY (cycle-85): every load-bearing predicate EXERCISES the SUT —
// it runs `go test -race` against the real packages and asserts the NAMED
// builder-authored behavioral test actually RAN and PASSED (checks the
// `--- PASS: <name>` marker in `-v` output). `go test -run X` on a package with
// no matching test exits 0 with "no tests to run" — a bare exit-code check would
// vacuously green, so every behavioral predicate below asserts the PASS marker,
// not merely exit==0. The only source-level assertions are the config-check
// apicover-enforce entry (C661_005, waived) and the core-consult wiring pin
// (C661_003), whose LOAD-BEARING anchor is that internal/core BUILDS against the
// new recurrence dependency (a magic string cannot make core compile).
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive  : C661_001 three same-pattern retros => count=3 + one idempotent bump.
//   - Negative/Edge : C661_002 count>=2 with NO open item => autofile seam fires
//     exactly ONCE (an implementation that double-fires, or never fires, FAILS).
//   - Semantic  : C661_003 RetroDecision with an Nth-occurrence lesson forces
//     "adapt"-with-escalation, never bare "proceed".
//   - Semantic  : C661_004 `evolve lessons recurrence` sorts patterns by count
//     and shows fix-item status.
//   - Config    : C661_005 internal/recurrence graduated into .apicover-enforce.
//
// BUILDER TEST-NAME CONTRACT — the predicates target these exact test names;
// Builder must author them (do NOT rename without updating this file):
//
//	internal/recurrence : TestLedger_ThreeSamePatternCountsThreeAndBumpsOnce
//	                      TestLedger_CountGE2NoOpenItemAutofilesOnce
//	internal/core       : TestDecideAfterRetro_NthOccurrenceForcesAdapt
//	cmd/evolve          : TestLessonsRecurrence_SortedByCountWithFixStatus
```
