# Comment history: `acs/cycle322`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle322/predicates_test.go:3` — above `package cycle322`

```text
// Package cycle322 materializes the cycle-322 acceptance criteria for the one
// committed top_n task (scout-report.md "## Selected Tasks"):
//
//	modelcatalog-write-error-paths — raise go/internal/modelcatalog statement
//	    coverage from the 87.3% baseline to the committed >= 92.0% floor by
//	    exercising the dark temp-file error exits of store.Write (66.7% — the
//	    mkdir / CreateTemp / Rename exits are ALREADY covered by
//	    store_errors_test.go; the remaining dark branches are tmp.Write,
//	    tmp.Sync, and tmp.Close, which need an injectable seam over
//	    os.CreateTemp). The json.MarshalIndent error arm is unreachable for a
//	    plain Catalog struct, so 85% — not 100% — is the Write ceiling.
//
// These predicates are BEHAVIORAL (cycle-85 lesson). C322_001/002 RUN the real
// modelcatalog suite in a subprocess under -coverprofile and assert on the
// measured `go tool cover -func` percentages; C322_003/004 RUN the suite and
// assert on its exit code; C322_005 CALLS the real modelcatalog.Write through
// its public API on a rename-failing path and asserts on the returned error and
// the absence of a leaked temp file. There is no load-bearing source-grep:
//
//   - A magic string in a source file cannot move a coverage number. Only
//     Builder's createTemp seam + the new tmp.Write/Sync/Close failure tests can.
//   - An EMPTY repo (no modelcatalog tests) yields 0% and fails every floor, so
//     the coverage gates are anti-no-op by construction.
//   - coverFuncOutput Fatals (RED) if the suite does not compile or any test
//     FAILs, so every coverage gate folds in the no-regression axis (AC4). The
//     new store_writefail_test.go references the not-yet-existing createTemp seam,
//     so today the package does not even COMPILE — every coverage/suite gate is
//     RED until Builder adds the seam.
//   - C322_005 is the explicit behavioral negative axis (adversarial-testing
//     SKILL §6): it drives Write's rename error exit and requires both a non-nil
//     error AND a clean directory — a happy-path-only Write cannot satisfy it.
//
// AC map (1:1 with the scout-report.md "Acceptance Criteria Summary" — 5 ACs):
//
//	modelcatalog-write-error-paths
//	  AC1 package coverage >= 92.0%                         → C322_001
//	  AC2 Write coverage >= 85%                             → C322_002
//	  AC3 read-only-dir write error test passes             → C322_003
//	  AC4 suite green (no regression)                       → C322_004
//	  AC5 no temp files left on failure paths               → C322_005
//
// AC3 note: scout-report.md AC3 names a test "TestWriteReadOnlyDirError"; the
// pre-existing test that pins exactly that behavior (a read-only evolveDir making
// CreateTemp fail) is TestWriteCreateTempFailsInReadOnlyDir in store_errors_test.go.
// AC3 binds to that real test name rather than churning the file with a rename.
//
// Floor binding (R9.3): internal/modelcatalog is the committed top_n task this
// cycle, so the coverage floors bind a committed package. The scout-DEFERRED
// items (ledger seal, cmd/evolve) get ZERO predicates here — authoring a floor on
// a deferred task would starve the committed one (cycle-280 lesson).
```
