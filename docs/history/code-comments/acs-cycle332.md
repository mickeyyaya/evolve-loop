# Comment history: `acs/cycle332`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle332/predicates_test.go:3` — above `package cycle332`

```text
// Package cycle332 materializes the cycle-332 acceptance criteria for the TWO
// committed top_n tasks (triage-report.md "## top_n"), both lifting test
// coverage of isolated dark error branches:
//
//	evalgate-error-branch-coverage — push internal/evalgate from 91.06% to the
//	    scout's >= 93.0% AC by covering FOUR dark statements:
//	      - cycleNumFromWorkspace nil-match `return 0`  (floorbinding.go:95)
//	      - cycleNumFromWorkspace Atoi-error `return 0`  (floorbinding.go:99)
//	      - fencedAfterHeading no-trailing-newline else  (slugs.go:128)
//	      - NewReviewer logf closure body, run via Review (reviewer.go:40)
//
//	releasepipeline-default-verify-error-branches — lift defaultReleaseVerify
//	    from 0.0% by covering its two early-exit guards (relative-repoRoot
//	    reject, missing-binary-on-disk) and add a DIRECT defaultShip
//	    binary-not-found characterization test.
//
// These predicates are BEHAVIORAL (cycle-85 lesson) — there is NO load-bearing
// source-grep:
//
//   - The coverage gates RUN the real package suite under -coverprofile and
//     assert on the measured `go tool cover -func` percentages. A magic string
//     cannot move a coverage number — only Builder's new tests can — so they are
//     anti-no-op by construction; an EMPTY repo yields 0% and fails every floor.
//   - coverFuncOutput Fatals (RED) if the suite does not compile or any test
//     FAILs, folding the no-regression axis into every coverage gate.
//   - The named-test PASS gates RUN the suite filtered to the committed test
//     functions and require each `--- PASS:` line; a no-op -run match prints
//     "no tests to run" with ZERO PASS lines (RED today), so they fail until
//     Builder's real tests exist and pass.
//   - The per-function anchors (cycleNumFromWorkspace / fencedAfterHeading /
//     NewReviewer / defaultReleaseVerify) pin the % movement to the TARGET dark
//     branches, not to incidental coverage elsewhere.
//
// ⚠ SCOUT-CORRECTION NOTE (Core Rule 3 — surfaced conflict; cycle-280/331 lesson
// — never pin an UNREACHABLE floor, but never lower a REACHABLE bar either):
// the scout asserted the cycleNumFromWorkspace Atoi-error branch is
// "unreachable" (regex guarantees digits). That is WRONG: strconv.Atoi of an
// all-digit string still fails on integer OVERFLOW (e.g. a 25-nine cycle
// number > int64 max → *NumError ErrRange). The correction matters because the
// scout's own >= 93.0% AC is NOT reachable WITHOUT covering that branch —
// statement math: 163/179 baseline + the 3 "reachable" branches (nil-match,
// no-newline, logf) = 166/179 = 92.74% < 93.0%. Covering the Atoi-overflow
// branch too gives 167/179 = 93.30%, the FIRST value that clears 93.0%. So the
// 93.0% AC is preserved (not adjusted) and the cycleNumFromWorkspace floor is
// pinned at 100% to FORCE both return-0 branches, including the overflow test
// the scout missed. test-report.md records this disposition.
//
// AC map (1:1 with the committed-task acceptance criteria):
//
//	evalgate-error-branch-coverage
//	  AC-E1 cycleNumFromWorkspace nil-match + Atoi-overflow covered → C332_001 (== 100%)
//	  AC-E2 fencedAfterHeading no-newline-else covered              → C332_002 (== 100%)
//	  AC-E3 NewReviewer logf closure body exercised via Review      → C332_003 (== 100%)
//	  AC-E4 evalgate package coverage lifts to the scout floor       → C332_004 (>= 93.0%)
//	releasepipeline-default-verify-error-branches
//	  AC-R1 the two defaultReleaseVerify guard tests exist & PASS   → C332_005 (named-test gate)
//	  AC-R2 defaultReleaseVerify coverage lifts off 0.0%            → C332_006 (>= 10.0%)
//	  AC-R3 defaultShip binary-not-found direct test exists & PASS  → C332_007 (named-test gate)
//
// Floor binding (R9.3): internal/evalgate and internal/releasepipeline are the
// SOLE packages the two committed top_n tasks target, so every floor/gate binds
// committed work. No triage-DEFERRED item gets a predicate here — a floor on a
// deferred task would starve the committed ones (cycle-280 lesson).
```

### `go/acs/cycle332/predicates_test.go:193` — above `func TestC332_001_CycleNumFromWorkspaceBothReturnZero(t *testing.T) {`

```text
// --- C332_001 (AC-E1): cycleNumFromWorkspace == 100% -------------------------
//
// Behavioral + branch anchor. cycleNumFromWorkspace is 71.4% today (5/7
// statements): BOTH `return 0` arms are dark — the nil-match arm
// (floorbinding.go:95, a non-`cycle-<N>` basename) and the strconv.Atoi-error
// arm (floorbinding.go:99). Reaching 100% REQUIRES a nil-match test (e.g.
// "workspace" or "cycle-300/artifacts") AND an Atoi-overflow test (a basename
// like "cycle-99999999999999999999999999" whose digits overflow int64 → Atoi
// ErrRange → return 0). The scout called the Atoi arm "unreachable"; it is
// reachable via overflow and MUST be covered or the package gate cannot reach
// 93.0% (see the scout-correction note). 7/7 = 100% is the exact ceiling.
```

### `go/acs/cycle332/predicates_test.go:316` — above `func TestC332_007_DefaultShipBinaryNotFoundDirectTestPass(t *testing.T) {`

```text
// --- C332_007 (AC-R3): defaultShip binary-not-found direct test exists & PASS -
//
// Behavioral named-test gate. NOTE (Core Rule 12 — fail loudly): defaultShip's
// binary-not-found arm (releasepipeline.go:618-621) is ALREADY incidentally
// covered by TestRun_NilShipOverriddenByDefault (which clears EVOLVE_GO_BIN+PATH
// and drives Run → defaultShip). The committed deliverable is therefore a
// DIRECT characterization test that calls defaultShip itself with no resolvable
// binary and asserts the "evolve binary not found" error — explicit, not
// incidental. RED today: TestDefaultShip_BinaryNotFound does not exist. GREEN
// when Builder adds it (clear EVOLVE_GO_BIN + PATH, t.TempDir() repoRoot with no
// go/{bin/,}evolve, assert a non-nil error mentioning "binary not found").
```
