# Comment history: `acs/cycle321`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle321/predicates_test.go:3` — above `package cycle321`

```text
// Package cycle321 materializes the cycle-321 acceptance criteria for the one
// committed top_n task (triage-report.md / scout-report.md "## Selected Tasks"):
//
//	clihealth-store-io-coverage — raise go/internal/clihealth statement coverage
//	    from the 90.1% baseline to the committed >= 94.0% floor by exercising the
//	    dark I/O error paths of the persistent bench Store: Store.write (63.6% —
//	    the mkdir-fail, WriteFile-fail and Rename-fail exits are dark; only the
//	    happy round-trip is hit), NewStore (66.7% — the nil-clock default arm
//	    `now = time.Now` is never taken because every test injects a fixed clock),
//	    and Store.Load (76.9% — the read-error WARN degradation and the
//	    valid-JSON-but-nil-`benches` arm are dark), while every existing
//	    clihealth test stays green.
//
// These predicates are BEHAVIORAL (cycle-85 lesson). C321_001..005 RUN the real
// clihealth test suite in a subprocess under -coverprofile and assert on the
// measured `go tool cover -func` percentages; C321_006 CALLS the real Store via
// its exported Bench entry point and asserts on the returned error. There is no
// load-bearing source-grep:
//
//   - A magic string in a source file cannot move a coverage number. Only
//     Builder's new tests, which actually drive write through its error exits,
//     call NewStore(root, nil), and feed Load a corrupt / typed / unreadable
//     file, can.
//   - An EMPTY repo (no clihealth tests) yields 0% and fails every floor, so the
//     coverage gates are anti-no-op by construction.
//   - coverFuncOutput Fatals (RED) if the suite does not compile or any test
//     FAILs, so every coverage gate folds in the no-regression axis (AC5).
//   - C321_006 is the explicit ADVERSARIAL negative axis (adversarial-testing
//     SKILL §6): it drives write's first error exit through the public Bench API
//     and requires a non-nil error — a happy-path-only suite cannot satisfy it.
//
// AC map (1:1 with the scout-report.md "Acceptance Criteria Summary" — 6 ACs):
//
//	clihealth-store-io-coverage
//	  AC1 package coverage >= 94.0%                      → C321_001
//	  AC2 write coverage >= 90%                          → C321_002
//	  AC3 NewStore coverage >= 90%                       → C321_003
//	  AC4 Load coverage >= 90%                           → C321_004
//	  AC5 suite green (no regression)                    → C321_005
//	  AC6 negative: write on read-only root → non-nil err → C321_006
//
// Floor binding (R9.3): internal/clihealth is the committed top_n task this
// cycle, so the coverage floors bind a committed package. The scout-DEFERRED
// items (ledger seal, modelcatalog Write, cmd/evolve) get ZERO predicates here —
// authoring a floor on a deferred task would starve the committed one
// (cycle-280 lesson).
```
