# Comment history: `acs/cycle1587`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1587/predicates_test.go:3` — above `package cycle1587`

```text
// Package cycle1587 materializes the cycle-1587 acceptance criteria for the
// sole fleet-assigned committed task, pipeline-defect-pipeline-blocker-cycle1582
// (scout-report.md ## Selected Tasks; triage priority P0). The lane's other
// assigned id, tokenopt-session-resume-on-retry, is DEFERRED per R9.3 — no
// predicate binds to it.
//
// The defect (cycle-1585 closed half of it; this cycle closes the rest):
// go/internal/core/cyclerun_dispatch.go's all-CLI-families-quota-exhausted
// abort is a DEFERRED, resumable checkpoint (typed ErrAllFamiliesExhausted,
// loop exits rc=5, `evolve loop --resume` re-enters the drained phase) — not a
// diagnosed phase failure. cycle-1585 stopped the runner from force-dispatching
// retro on that arm, but go/internal/core/failure_learning.go:recordFailureLearning
// still called recordFailedApproachState first, unconditionally appending a
// FailedRecord to state.FailedAt and queuing a P0 "cycle-N-failed-<phase>"
// carryover todo before its ErrAllFamiliesExhausted short-circuit — the
// cycle-1582 dossier root cause: that spurious todo competed with real work on
// every quota wall. The fix moves the short-circuit BEFORE
// recordFailedApproachState and threads a ClassificationOverride so a
// resource-abort at the dispatch boundary can never adopt a stale phase
// self-report (a prior code-failure report left over from an earlier attempt).
//
// AC map (1:1 with scout-report.md ## Selected Tasks "Acceptance Criteria"):
//
//	AC1 "a final negative verdict with a runtime-minted substantive explanation
//	     remains coherent — does not trigger verdict-incoherence"
//	    → C1587_001 (pre-existing GREEN: go/internal/core/system_failure_test.go
//	      already covers the audit- and ship-phase SubstantiveError carriers,
//	      cycles-930/931/932 and cycle-1329; this predicate pins that regression
//	      coverage stays wired rather than re-deriving it)
//	AC2 "a negative verdict with green artifacts but no explanation is still
//	     detected as verdict-incoherence, except the fully-verified late-write
//	     case reconciles to PASS"
//	    → C1587_002 (pre-existing GREEN, same file — the forged-vs-reconciled
//	      branch, TestDetectVerdictIncoherence_ForgedVerdict_Halts +
//	      TestDetectVerdictIncoherence_ReconcileUsesFullVerify)
//	AC3 "malformed or missing artifacts cannot be laundered into reconciliation"
//	    → C1587_003 (pre-existing GREEN, same file —
//	      TestDetectVerdictIncoherence_ReconcileUsesFullVerify's OK=false arm)
//
// The concrete pipeline-blocker shape this cycle repairs — a DEFERRED
// quota-exhaustion abort minting its OWN false-negative "verdict" via
// failure-learning bookkeeping before the coherence floor ever runs — is
// pinned by C1587_004..007, driving the real dispatch()/RunCycle chain:
//
//	C1587_004 no FailedRecord is appended on the all-85 (DEFERRED) arm
//	C1587_005 no P0 carryover todo is queued on the all-85 (DEFERRED) arm
//	C1587_006 the retro runner is never force-run as a learning side effect
//	C1587_007 (NEGATIVE, anti-no-op) a single exit=85 attempt with a
//	          differently-shaped (non-85) sibling — NOT the all-families
//	          signature — still learns normally: FailedRecord appended AND
//	          the carryover todo queued. A guard that swallows every failure
//	          passes C1587_004..006 and dies here.
//	C1587_008 the eval file for this task passes the SSOT quality checker
//	          over a non-empty score_cap set (a vacuous eval PASSes for free)
//
// Adversarial axes: negative (C1587_007 — the fix must not become a blanket
// learning suppression), edge (the multiply-%w-wrapped ErrAllFamiliesExhausted
// sentinel matched via errors.Is, exercised inside C1587_004..006's underlying
// core test), semantic (verdict-coherence-at-finalization vs
// bookkeeping-before-classification are two distinct seams, not one behavior
// restated).
//
// No source-grep predicates (cycle-85 rule): every predicate here runs the
// real production dispatch/finalization chain as a subprocess `go test` and
// asserts on the named PASS marker (a bare exit 0 would hide a renamed or
// skipped test) or the eval-quality checker's own verdict. Every invocation
// names ONE package and is narrowed with -run (flaky-predicate-shape rule: no
// `/...` sweep, no unnarrowed ./internal/core).
```

### `go/acs/cycle1587/predicates_test.go:115` — above `func TestC1587_001_explained_negative_verdict_stays_coherent(t *testing.T) {`

```text
// C1587_001 (AC1, pre-existing GREEN regression pin): a negative verdict
// EXPLAINED by a runtime-minted SubstantiveError (audit-phase diagnosed
// gate-downgrade, cycles-930/931/932; ship-phase rejection, cycle-1329) must
// never trigger verdict-incoherence. Binds the coherence floor's own
// regression suite so a future edit cannot silently regress it without also
// failing this cycle's audit.
```

### `go/acs/cycle1587/predicates_test.go:151` — above `func TestC1587_005_deferred_arm_queues_no_carryover_todo(t *testing.T) {`

```text
// C1587_005: no P0 "cycle-<N>-failed-<phase>" carryover todo is queued on the
// DEFERRED arm — the exact cycle-1582 dossier root cause (a spurious todo
// competing with real work on every quota wall).
```
