# Comment history: `acs/cycle951`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle951/predicates_test.go:3` — above `package cycle951`

```text
// Package cycle951 materializes the cycle-951 acceptance criteria for this
// fleet lane's sole committed task, coherence-reconcile-selfheal (triage top_n
// id / inbox id: clean-exit-late-write-verdict-race-TRUE-root-cause).
//
// Root cause (LIVE-proven in the inbox item): the bridge declares a phase's
// clean exit before Claude Code finishes its post-turn async writes. The
// runner's ~3s reconcile-settle window is far shorter than the observed 60-90s
// deliverable dribble, so the runner records FAIL while the audit-report is
// still landing. Minutes later the file is valid and green on disk, and the
// ADR-0072 verdict-coherence floor (coherence.CheckVerdictCoherence, consulted
// from core.detectVerdictIncoherence) sees recorded=FAIL vs on-disk audit=PASS
// + acs=PASS and HALTS the loop as a "forged verdict" — when it was a benign
// timing race (cycles 930/931/932/cycle-3 family).
//
// Fix (this cycle): make the coherence floor SELF-HEAL instead of halt when the
// recorded-negative is contradicted by green artifacts AND the audit-report
// passes the FULL deliverable.Verify chain (challenge-token + required sections
// + ADR-0039 failure-context) — a benign clean-exit-late-write race reconciles
// to PASS. Halt is PRESERVED as the fallback for any case where the deliverable
// does NOT fully verify (genuine forgery / a malformed report merely tagged with
// a PASS sentinel) — the anti-gaming boundary the inbox explicitly must-preserve.
//
// SUT surface the Builder must implement (see test-report.md handoff), WITHOUT
// modifying this file:
//
//	coherence.VerdictInputs gains:  DeliverableValid bool
//	    // the on-disk audit-report passed the FULL deliverable.Verify chain
//	    // (challenge-token + required sections + ADR-0039 failure-context),
//	    // NOT just the cheap ParseVerdictSentinel read.
//	coherence.Coherence gains:      Reconciled bool
//	    // the recorded negative was a benign clean-exit-late-write race:
//	    // green artifacts AND a fully-valid deliverable → self-heal to PASS,
//	    // do NOT halt. Mutually exclusive with Incoherent.
//	coherence.CheckVerdictCoherence: within the existing forgery-signature branch
//	    (rec FAIL/WARN, AuditRan, !SubstantiveError, audit==PASS, acs==PASS):
//	        DeliverableValid==true  → Coherence{Reconciled:true}   (Incoherent=false, no halt)
//	        DeliverableValid==false → Coherence{Incoherent:true, Category:"verdict-incoherence"} (halt, as today)
//	    Every non-signature case (PASS recorded, SubstantiveError, !AuditRan,
//	    audit!=PASS, acs absent) returns the zero Coherence{} REGARDLESS of
//	    DeliverableValid — DeliverableValid only ever DOWNGRADES a would-be halt
//	    to a reconcile; it never manufactures a reconcile out of a coherent case.
//	core.detectVerdictIncoherence: compute DeliverableValid by running the FULL
//	    deliverable.Verify / VerifyCatalogAware on the on-disk audit-report
//	    artifact (NOT ReadCycleVerdicts's sentinel parse) and feed it into
//	    VerdictInputs; on Reconciled, the recorded final verdict is updated to
//	    PASS and NO halt signal is returned.
//
// Predicate style (cycle-85 rule): every predicate EXERCISES the system under
// test — the direct-call predicates (001-003) invoke coherence.CheckVerdictCoherence
// and assert on its returned struct; the subprocess predicates (004-005) require
// an explicit "--- PASS: <name>" for each named unit test (rename/skip/vacuous-run
// gaming is caught — exit 0 alone never satisfies a predicate); the build/vet/-race
// predicates (006-008) run the toolchain; 009 runs the SSOT eval quality checker.
// No source-grep predicate exists in this file.
//
// Adversarial diversity (skills/adversarial-testing §6):
//
//	NEGATIVE  → C951_002 (deliverable INVALID → forged still HALTS; the strongest
//	            anti-no-op signal: a no-op that always reconciles fails here).
//	EDGE/OOD  → C951_003 (coherent cases must NOT reconcile even with a valid
//	            deliverable: PASS recorded, SubstantiveError, !AuditRan, acs absent).
//	SEMANTIC  → reconcile, halt, and no-op-coherent are three DISTINCT outcomes,
//	            each asserted separately, not one behavior restated.
//
// AC map (1:1 with the disposition table in test-report.md):
//
//	AC1 valid deliverable → reconcile-to-PASS, no halt → C951_001 (pure) · C951_004 (unit) · C951_005 (call-site)
//	AC2 invalid deliverable → forged still halts       → C951_002 (pure/NEGATIVE) · C951_004 (unit) · C951_005 (call-site)
//	AC1/AC2 no over-reconcile of coherent cases        → C951_003 (pure/EDGE)
//	AC3 -race green, no regression                     → C951_006 (scoped -race) · C951_007 (build) · C951_008 (vet)
//	AC4 ship-guard/pane/teardown untouched             → manual+checklist (Auditor diff-scope) — see test-report.md
//	Step 6b eval file passes quality-check             → C951_009
```

### `go/acs/cycle951/predicates_test.go:151` — above `func TestC951_002_InvalidDeliverableStillHalts(t *testing.T) {`

```text
// TestC951_002_InvalidDeliverableStillHalts is the anti-gaming boundary: the
// SAME green-artifact signature but with a deliverable that does NOT fully
// verify (missing challenge-token / required section / ADR-0039 failure-context,
// or a malformed report merely tagged with a PASS sentinel) must STILL be
// Incoherent → halt, exactly as before the fix. A no-op that always reconciles
// (launders every FAIL to PASS) fails here — this is the strongest anti-no-op
// signal in the suite.
```

### `go/acs/cycle951/predicates_test.go:226` — above `func TestC951_005_CallSiteUsesFullVerify(t *testing.T) {`

```text
// TestC951_005_CallSiteUsesFullVerify requires the core-package unit test proving
// detectVerdictIncoherence derives DeliverableValid from the real
// deliverable.Verify / VerifyCatalogAware (challenge-token + required sections +
// ADR-0039) — so a PASS-sentinel-tagged but malformed audit-report still halts,
// and a fully-valid one reconciles to PASS with no halt signal.
```

### `go/acs/cycle951/predicates_test.go:237` — above `func TestC951_006_ChangedLogicRaceClean(t *testing.T) {`

```text
// -----------------------------------------------------------------------------
// AC3 — the changed logic is race-clean (scoped to the touched tests to avoid
// the whole-integration-suite flake trap, cycles 858/859/862).
// -----------------------------------------------------------------------------
```
