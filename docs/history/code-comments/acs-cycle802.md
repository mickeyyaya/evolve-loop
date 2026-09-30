# Comment history: `acs/cycle802`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle802/predicates_test.go:3` — above `package cycle802`

```text
// Package cycle802 materializes the cycle-802 acceptance criteria for this
// fleet lane's sole committed inbox item, retro-bridge-timeout-width10
// (scout-report.md Selected Tasks 1-4; per R9.3 no predicates bind to any
// other lane's items or to the Deferred TestConcurrentNonFloorLaunchesBounded
// criterion).
//
// AC map (1:1, from scout-report.md Selected Tasks verifiableBy + Acceptance
// Criteria Summary):
//
//	AC1 non-floor phase FAIL/WARN never overwrites an already-recorded
//	    floor-derived FinalVerdict (Task 1: floor-gated-final-verdict)
//	    → C802_001 runs TestNonFloorPhaseFailure_DoesNotOverrideFloorVerdict.
//	AC2 non-floor phase failure after audit already FAILed leaves
//	    FinalVerdict FAIL (no accidental "recovery") (Task 1)
//	    → C802_002 runs TestNonFloorPhaseFailure_FailAudit_StaysFail.
//	AC3 a FLOOR phase's own failure remains cycle-fatal — the guard must
//	    not accidentally shield floor phases too (Task 1)
//	    → C802_003 runs TestFloorPhaseFailure_RemainsCycleFatal.
//	AC4 the resume dispatch loop (resume.go:324) carries the identical
//	    guard, closing the --resume storm-recurrence gap (Task 2:
//	    floor-gated-final-verdict-resume-parity)
//	    → C802_004 runs TestResumeNonFloorPhaseFailure_DoesNotOverrideFloorVerdict.
//	AC5 contract exhaustion (unparseable verdict after retries) on a
//	    non-floor phase degrades to SKIPPED+WARN via the same floor gate,
//	    subsuming advisory-phase-contract-degrade (Task 3:
//	    contract-exhaustion-degrades-non-floor)
//	    → C802_005 runs TestContractExhaustion_NonFloorPhase_DegradesToSkippedWarn.
//	AC6 skipped/degraded non-floor phases are surfaced in the dossier via a
//	    durable CycleResult field, never silently dropped (Task 1 dossier
//	    surfacing). The field was CycleResult.SkippedPhases[] until
//	    dossier-retro-skipped-mislabel split the record by what actually
//	    happened: a phase that RAN whose verdict was declined now lands in
//	    CycleResult.VerdictsNotAdopted[] → dossier
//	    phases_run_verdict_not_adopted, while SkippedPhases[] is reserved for a
//	    phase that did NOT run. The AC — the degrade survives instead of
//	    clobbering the floor verdict — is unchanged and asserted on both halves.
//	→ C802_006 runs TestDossier_RecordsSkippedPhases (name retained: the
//	  predicate binds it by name, and renaming would mint a new ac_id identity).
//	AC7 retrospective.json and memo.json declare sandbox.allow_network
//	    true so the runtime stops silently forcing it and emitting the
//	    noisy WARN (Task 4: retro-memo-allow-network-honest)
//	    → C802_007 is a config-check waiver (declarative JSON field, not
//	      logic) reading both profiles directly.
//
// Adversarial axes: negative (AC2/AC3 pin the cases the guard must NOT
// change — a naive "always keep first PASS" fix would break AC3), edge
// (AC5 exhaustion after retries, not first failure), semantic (AC1 floor
// non-overwrite, AC3 floor-fatal, AC6 dossier surfacing, and AC7 config
// honesty are four distinct behaviors, not one behavior restated). No
// source-grep predicates over logic files (cycle-85 rule): AC1-AC6 each
// execute the system under test as a subprocess; AC7 is the declared
// config-check exception (see go/acs/README.md predicate-quality table).
```
