# Comment history: `acs/cycle784`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle784/predicates_test.go:3` — above `package cycle784`

```text
// Package cycle784 materializes the cycle-784 acceptance criteria for this
// fleet lane's sole committed inbox item, chronicle-s3-digest-wiring
// (triage-report.md ## top_n: seed-digest-at-cycle-start +
// inject-recent-outcomes-prompts; per R9.3 no predicates bind to any other
// lane's items).
//
// AC map (1:1, from scout-report.md Selected Tasks verifiableBy + Acceptance
// Criteria Summary):
//
//	AC1 seed-digest-at-cycle-start behavior (shadow seeds the artifact, off
//	    writes nothing, enforce injects Context["recent_outcomes"], digest
//	    failure WARNs and never aborts)
//	    → C784_001 runs the four TestNewCycleRun_* unit tests as a -race
//	      subprocess and requires each named "--- PASS:" marker (exit-0
//	      alone could hide a renamed/skipped test).
//	AC2 inject-recent-outcomes-prompts behavior (scout + triage render the
//	    injected digest; absent key ⇒ byte-identical prompts)
//	    → C784_002 same subprocess pattern over the scout + triage prompt
//	      tests, counting the byte-identical pin once per package.
//	AC3 permanent regression entry (.evolve/evals/chronicle-s3-digest-wiring.md)
//	    asserts real command output, not existence checks
//	    → C784_003 runs the SSOT checker (internal/evalqualitycheck — the
//	      exact code behind `evolve eval quality-check`) and requires
//	      Overall==PASS over a NON-EMPTY (≥2) command set, closing the
//	      vacuous-empty-eval hole.
//
// Adversarial axes: negative (off-stage write-nothing + byte-identical pins
// inside the C784_001/002 suites; C784_003 rejects the vacuous PASS), edge
// (digest-failure directory collision exercised by the unit suite), semantic
// (seeding vs injection vs prompt rendering vs eval rigor are distinct
// behaviors). No source-grep predicates (cycle-85 rule): every predicate
// executes the system under test as a subprocess or runs the SSOT checker.
```
