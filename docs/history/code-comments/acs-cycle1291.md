# Comment history: `acs/cycle1291`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1291/predicates_test.go:3` — above `package cycle1291`

```text
// Package cycle1291 materializes the cycle-1291 acceptance criteria for the one
// fleet-scoped todo pinned to this lane (contract-block-cli-escalation), which
// triage resolved to a single top_n task:
//
//	fix-contract-block-identity-subset-collapse
//
// WHAT THIS CYCLE CLOSES. cycle-1289 landed the escalation identity gate
// (contractBlocksShareIdentity, scoping constraint 4) as a WHOLE-STRING compare
// of normalizeReasonForFingerprint(reason). Its audit rejected that HIGH:
//
//	"contractBlocksShareIdentity compares whole summarize() strings, so a
//	 partially-repaired violation set (subset) reads as a different defect and
//	 suppresses [escalation]"
//	(.evolve/runs/cycle-1289/audit-fail-reason.json)
//
// The compared string is deliverable.summarize() — a "; "-joined rendering of
// EVERY violation on the block ("[code] message"). So a correction that closes
// one of two violations makes block 2 render a strict SUBSET of block 1, the two
// strings differ verbatim, and the escalation is suppressed at exactly the
// moment the CLI has most clearly proven it cannot repair the deliverable. The
// fix is to key identity on the violation-CODE SET (deliverable.Violation.Code,
// the stable primitive) rather than on rendered text: same defect ⇔ the two
// blocks' code sets intersect.
//
// IMPORT-CYCLE CONSTRAINT (cycle-644 reachability obligation, compiler-verified
// this cycle): internal/deliverable imports internal/core (reviewer.go:12,
// verifier.go:18) and core imports deliverable nowhere, so core.ReviewResult can
// NOT carry []deliverable.Violation — that shape is an import cycle and would
// make the criterion permanently unsatisfiable. The code set must reach core as
// plain data. These predicates pin BEHAVIOUR through the real RunCycle ladder
// and deliberately do NOT pin either implementation shape.
//
// PREDICATE STRATEGY. Every predicate executes the system under test through a
// `go test` subprocess on ONE named package narrowed by -run, and requires an
// explicit "--- PASS: <name>" per named test — exit 0 alone never satisfies a
// predicate (rename/skip gaming is caught). No source-grep predicate exists here
// (the cycle-85 degenerate-predicate ban): a "the file contains the word
// codeSet" assertion passes on a magic string regardless of the fix. The
// underlying unit tests drive Orchestrator.RunCycle → reviewAndGuard, the real
// production caller of contractBlocksShareIdentity, so a seam reachable only
// from a test cannot satisfy them.
//
// AC map (1:1 with the disposition table in test-report.md):
//
//	AC1 subset repair still escalates      → C1291_001  (POSITIVE — THE audit defect)
//	AC2 superset regression still escalates→ C1291_002  (POSITIVE)
//	AC3 disjoint sets do NOT escalate      → C1291_003  (NEGATIVE — anti-over-escalation)
//	AC4 reordered/reworded set escalates   → C1291_004  (EDGE — rendering instability)
//	AC5 code-less reasons still escalate   → C1291_005  (EDGE — fail-safe)
//	AC6 cycle-1289 identity gate preserved → C1291_006  (regression, 3 pre-existing tests)
//	AC7 escalation feature set intact      → C1291_007  (regression, full suite)
//	AC8 core + deliverable build and vet    → C1291_008
//	AC9 eval file passes quality-check      → C1291_009
```

### `go/acs/cycle1291/predicates_test.go:74` — above `func runGoTest(t *testing.T, pkg, runExpr string, wantPass []string) {`

```text
// runGoTest runs the named tests of ONE package (verbose, fresh) and requires an
// explicit "--- PASS: <name>" for every wantPass. Deliberately -run-narrowed and
// single-package: a `./...` sweep or an unnarrowed ./internal/core run is the
// flaky-predicate shape that false-RED'd cycles 1173/1175/1178 under fleet load.
```

### `go/acs/cycle1291/predicates_test.go:93` — above `func TestC1291_001_subset_repair_still_escalates(t *testing.T) {`

```text
// AC1 — THE cycle-1289 audit defect. Block 1 reports {missing_section,
// missing_verdict}; the correction closes one, so block 2 reports the SUBSET
// {missing_verdict}. Same defect, partially repaired ⇒ the second consecutive
// block must still escalate off the failing CLI family.
```

### `go/acs/cycle1291/predicates_test.go:136` — above `func TestC1291_006_cycle1289_identity_axes_preserved(t *testing.T) {`

```text
// AC6 — the cycle-1289 identity gate's own three axes must survive the rewrite:
// differing violations suppress, normalization-equal reasons escalate, and the
// HOT-BREAKER edge (no prior reason observed ⇒ escalate) stays open. That last
// one is why the rule is "prior reason known AND differing ⇒ suppress" rather
// than "equal ⇒ escalate", and a code-set rewrite can easily drop it.
```

### `go/acs/cycle1291/predicates_test.go:170` — above `func TestC1291_008_core_and_deliverable_build_and_vet(t *testing.T) {`

```text
// AC8 — both packages on the fix's seam build and vet clean. Named explicitly
// rather than swept with ./... so a failure names the package that broke; this
// is also the compiler proof that no import cycle was introduced between core
// and deliverable while threading the code set (the cycle-644 shape).
```
