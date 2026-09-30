# Comment history: `acs/cycle787`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle787/predicates_test.go:3` — above `package cycle787`

```text
// Package cycle787 materializes the cycle-787 acceptance criteria for the
// fleet-scoped todo merge-rung0-trivial-rebase-fastpath, task
// composition-verdict-writer: ledger.WriteCompositionVerdict, the RUNG 0
// producer completing the read/verify/write triangle (cycle-786 landed the
// reader + kernel verifier; no producer existed anywhere in the tree).
//
// AC map (1:1 with .evolve/evals/composition-verdict-writer.md):
//
//	AC1 round-trip (write → read back → existing kernel verify accepts)
//	    → C787_001 runs TestWriteCompositionVerdict_RoundTrip. RED at
//	      authoring (WriteCompositionVerdict undefined: compile failure).
//	AC2 negative: forged patch_id / drifted diff / missing or failed
//	    required composed gate → error AND zero bytes appended
//	    → C787_002 runs TestWriteCompositionVerdict_RejectsPatchIDMismatch.
//	AC3 edge: empty AND whitespace-only diffs rejected, nothing persisted
//	    → C787_003 runs TestWriteCompositionVerdict_EmptyDiff.
//	AC4 `go build ./...` clean → C787_004 (whole-module build subprocess).
//	AC5 `go vet ./internal/adapters/ledger/...` clean → C787_005.
//	Step 6b: eval file passes the SSOT quality checker non-vacuously
//	    → C787_006.
//
// Adversarial axes: negative (C787_002 — four distinct rejection paths, each
// asserting NO partial write by byte-length), edge (C787_003 — nil, "", and
// two whitespace-only variants), semantic (round-trip acceptance, fail-closed
// rejection, and empty-input hygiene are three distinct behaviors). Every
// predicate executes the system under test via go test/build/vet subprocesses
// — no source-grep predicates (cycle-85 rule).
```
