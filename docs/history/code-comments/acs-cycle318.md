# Comment history: `acs/cycle318`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle318/predicates_test.go:3` — above `package cycle318`

```text
// Package cycle318 materializes the cycle-318 acceptance criteria for the two
// committed top_n tasks (triage-report.md "## top_n"):
//
//	cmd-loop-fail-breaker-isolation — TestRunLoop_FailVerdictBreaks inherits the
//	    operator's ambient EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS=3, so the loop never
//	    trips the stop-on-first-fail breaker and exits via max_cycles (rc=3)
//	    instead of fail (rc=2). The fix isolates the test from the ambient var
//	    (a t.Setenv call). After the fix the test — and the whole cmd/evolve
//	    suite — is green even when the var is set.
//
//	ledger-seal-io-coverage — add targeted tests for the low-coverage seal I/O
//	    helpers (writeSegment 50.0%, rewriteLive 52.2%, readSegment 71.4%,
//	    linesEqual 66.7%; package total 82.4%), lifting internal/adapters/ledger
//	    statement coverage to the committed >= 85.0% floor while the existing
//	    TestSeal* contract stays green.
//
// These predicates are BEHAVIORAL (cycle-85 lesson). They run the REAL test
// suites in a subprocess and assert on the measured exit code / coverage
// percentage — no source-grep gaming:
//
//   - Task 1 predicates run cmd/evolve under an INJECTED
//     EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS=3 (cmd.Env), so they require the
//     isolation fix REGARDLESS of the host's ambient env: pre-fix the loop
//     continues past both FAIL cycles and exits rc=3 → subprocess non-zero →
//     RED; post-fix the test's own isolation overrides the injected value →
//     rc=2 → exit 0 → GREEN. The predicate encodes INTENT (the test isolates
//     from the ambient var) not a specific implementation line, so any valid
//     isolation mechanism satisfies it. A magic string cannot make a failing
//     loop exit rc=2.
//   - Task 2 coverage gates RUN the real ledger suite under -coverprofile and
//     assert on `go tool cover -func`. A magic string in a source file cannot
//     move the number — only Builder's new tests, which actually call the
//     helpers, can. An EMPTY repo (no tests) yields 0% and fails every floor,
//     so they are anti-no-op by construction. coverFuncOutput Fatals (RED) if
//     the suite does not compile or any test FAILs, so every coverage gate also
//     folds in the no-regression axis.
//
// AC map (1:1 with the scout-report.md "Acceptance Criteria Summary" — 4 ACs —
// plus one adversarial negative predicate):
//
//	cmd-loop-fail-breaker-isolation
//	  AC1 full cmd/evolve suite exit 0          → C318_002
//	  AC2 TestRunLoop_FailVerdictBreaks exit 0  → C318_001
//	ledger-seal-io-coverage
//	  AC1 ledger coverage >= 85.0%              → C318_003
//	      (negative/edge axis of AC1)           → C318_005 (linesEqual false branch)
//	  AC2 TestSeal* suite exit 0                → C318_004
//
// Floor binding (R9.3): internal/adapters/ledger is a committed top_n task this
// cycle, so the coverage floor (C318_003/C318_005) binds a committed package.
// The triage-DEFERRED items (bridge-*, preserve-worktree-on-verdict-fail) get
// ZERO predicates here — authoring a floor on a deferred task would starve the
// committed ones (cycle-280 lesson).
```
