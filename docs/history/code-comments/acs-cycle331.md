# Comment history: `acs/cycle331`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle331/predicates_test.go:3` — above `package cycle331`

```text
// Package cycle331 materializes the cycle-331 acceptance criteria for the TWO
// committed top_n tasks (triage-report.md "## top_n"), both lifting coverage of
// the OS write-error branches in internal/adapters/ledger (84.29% baseline):
//
//	ledger-seal-write-error-branches — exercise the dark write-error branches in
//	    seal.go: writeSegment's os.CreateTemp failure (read-only parent dir;
//	    62.5% baseline) and rewriteLive's CreateTemp failure (read-only ledger
//	    dir; 65.2% baseline). The named tests are TestWriteSegment_MkdirError,
//	    TestWriteSegment_CreateTempError, TestRewriteLive_CreateTempError.
//
//	ledger-anchor-write-error-branch — exercise the dark write-error branches in
//	    anchor.go: Anchor's gatherAllLines propagation, CreateTemp failure, and
//	    final-rename failure (65.6% baseline), plus loadAnchorSHA's corrupt-JSON
//	    degradation to "" (85.7% baseline). Named tests TestAnchor_CreateTempError,
//	    TestAnchor_RenameError, TestAnchor_GatherError, TestLoadAnchorSHA_CorruptJSON.
//
// These predicates are BEHAVIORAL (cycle-85 lesson) — there is NO load-bearing
// source-grep:
//
//   - The named-test gates RUN the real ledger suite in a subprocess filtered to
//     the committed test functions and require both a clean exit AND each
//     `--- PASS:` line. A magic string in a source file cannot make a named test
//     appear and PASS; an EMPTY repo (no such tests) produces "no tests to run"
//     and zero PASS lines, so the gates are RED today and only Builder's real
//     error-branch tests turn them GREEN. They encode the scout verifiableBy
//     commands verbatim.
//   - The coverage gates RUN the real package suite under -coverprofile and
//     assert on the measured `go tool cover -func` percentages. A magic string
//     cannot move a coverage number — only Builder's new tests can — so they are
//     anti-no-op by construction; an EMPTY repo yields 0% and fails every floor.
//   - coverFuncOutput Fatals (RED) if the suite does not compile or any test
//     FAILs, folding the no-regression axis into every coverage gate; a dedicated
//     -race gate adds the data-race axis with a distinct command verb.
//   - The per-function anchors (writeSegment / Anchor / loadAnchorSHA) pin the %
//     movement to the TARGET dark branches, not to incidental coverage elsewhere.
//
// ⚠ FLOOR-CALIBRATION NOTE (Core Rule 3 — surfaced conflict; cycle-280 lesson —
// never pin an UNREACHABLE floor or it starves the committed work):
// the scout's "Both: package coverage >= 87%" AC is NOT reachable with the
// chmod fault-injection technique the scout itself specified. Line-level
// analysis of the cover profile shows the only NEW statements the planned tests
// can cover are 7 (writeSegment CreateTemp +1, gatherAllLines readSegment +1,
// loadAnchorSHA unmarshal +1, Anchor gather/CreateTemp/rename +4) → 354+7 =
// 361/420 = 85.95%. The gzip-writer / tmp.Write / tmp.Sync / tmp.Close / final
// os.Rename error arms in writeSegment, rewriteLive, and Anchor are UNREACHABLE
// by filesystem tricks (a freshly-created regular file does not fail
// write/fsync/close portably — the SAME finding the cycle-322 modelcatalog eval
// documents). Reaching 87% would need +12 covered statements, i.e. covering
// those fd-level arms, which requires a production write-seam the scout did not
// scope. So the package gate (C331_006) is pinned at the ACHIEVABLE >= 85.2%
// (+4 over the 84.29% baseline, 3-statement margin under the ~86.0% realistic
// max), NOT 87%. test-report.md dispositions the 87% AC as falsified→adjusted.
//
// AC map (1:1 with the committed-task acceptance criteria):
//
//	ledger-seal-write-error-branches
//	  AC-S1 the three seal write-error tests exist & PASS  → C331_001 (named-test gate)
//	  AC-S2 writeSegment CreateTemp branch covered         → C331_002 (writeSegment >= 66%)
//	ledger-anchor-write-error-branch
//	  AC-A1 the four anchor write-error tests exist & PASS → C331_003 (named-test gate)
//	  AC-A2 Anchor gather/CreateTemp/rename branches covered→ C331_004 (Anchor >= 75%)
//	  AC-A3 loadAnchorSHA corrupt-JSON branch covered      → C331_005 (loadAnchorSHA >= 95%)
//	both
//	  AC-B1 package coverage lifts to the achievable floor → C331_006 (package >= 85.2%, adjusted from 87%)
//	  AC-B2 ledger suite stays green under -race           → C331_007
//
// Floor binding (R9.3): internal/adapters/ledger is the SOLE package both
// committed top_n tasks target, so every floor/gate binds committed work. No
// triage-DEFERRED item (looppreflight saveVersionCache, interaction PromoteRule)
// gets a predicate here — a floor on a deferred task would starve the committed
// ones (cycle-280 lesson).
```

### `go/acs/cycle331/predicates_test.go:219` — above `func TestC331_002_WriteSegmentCreateTempCovered(t *testing.T) {`

```text
// --- C331_002 (AC-S2): writeSegment coverage >= 66% --------------------------
//
// Behavioral + branch anchor. writeSegment is 62.5% today (15/24 statements):
// the os.CreateTemp error arm (`return fmt.Errorf("ledger seal: tmp: %w", ...)`)
// is dark (the MkdirAll-error and final-rename-error arms are ALREADY covered).
// Reaching 66% REQUIRES a test that makes CreateTemp fail (read-only parent
// dir) — +1 statement → 16/24 = 66.67%. The gzip-write/close, sync, and
// tmp.Close error arms are UNREACHABLE by filesystem tricks (cycle-322 finding),
// so ~66.7% is writeSegment's practical ceiling — the floor is 66, not 75
// (the scout hypothesis of 75%+ is falsified; see floor-calibration note).
```
