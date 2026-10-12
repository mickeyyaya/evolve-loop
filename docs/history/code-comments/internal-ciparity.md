# Comment history: `internal/ciparity`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero h1

### `go/internal/ciparity/ciparity.go:73` — above `func PackageDirHasProductionGoFiles(moduleDir, pkg string) bool {`

```text
// PackageDirHasProductionGoFiles reports whether the package directory behind
// an enforce-list-form pattern ("./internal/foo", resolved against moduleDir)
// holds at least one non-test .go file — an exported API surface the repo-wide
// apicover gate could actually inspect. A TEST-ONLY package (the tdd phase's
// RED-first mint) has zero exported production symbols, so the graduation
// obligation is VACUOUS on it: CI's enforce step passes such packages with
// "0 exported, 0 covered", while flagging them killed cycles 1223/1224/1228
// with identical fingerprints and halted the 2026-08-02 batch. BOTH graduation
// seams (build-entry abort, audit offender list) consult this ONE predicate so
// a package cannot pass the build seam and then die at audit for the same
// vacuous reason. A recursive pattern ("...") names no single directory, so it
// reports true (stays flagged, never guessed about). Fail-open on an
// unreadable dir — a package that cannot be proven obligation-bearing must not
// fail a phase; the repo-wide CI gate stays the backstop.
```

### `go/internal/ciparity/ciparity.go:146` — above `const CoverageTags = "integration acs"`

```text
// CoverageTags is the SSOT build-tag set covering BOTH tiers the repo gates real
// coverage behind: //go:build integration (real-FS/git/tmux) AND //go:build acs
// (in-package acs suites in internal/core, internal/acssuite,
// internal/phases/audit, internal/evalgate). Every scoped coverage-measuring
// call site MUST build its `go test` args through CoverageTestArgs so it reads
// the SAME (tagged) coverage number CI does. An untagged run under-reports a
// tag-gated package's real coverage by up to 43 points (R1:
// knowledge-base/research/test-coverage-audit-2026-07.md — internal/phases/ship
// measured 47.0% plain vs 90.6% tagged), which would let a gate ship on a wrong
// number; measuring the integration tier alone still under-counts the acs-gated
// in-package suites. This is the same defect class ADR-0069 fixed once for
// vet/apicover, now for the coverage dimension.
```

### `go/internal/ciparity/ciparity_test.go:65` — above `func TestPackageDirHasProductionGoFiles(t *testing.T) {`

```text
// TestPackageDirHasProductionGoFiles — the shared graduation predicate both
// seams consult (build-entry abort + audit offender list). The test-only row
// is the cycles-1223/1224/1228 halt class; the recursive-pattern and
// unreadable-dir rows pin the two documented non-guessing directions.
```

### `go/internal/ciparity/composedgates.go:1` — above `package ciparity`

```text
// composedgates.go — the composed-tree gate contract for the trivial-rebase
// audit carry-forward (merge ladder RUNG 0, cycle-786). Gates bind to the
// TREE: even when the audit verdict follows the change (patch-id) across a
// clean rebase, the full native gate set must re-run green on the composed
// tree — via the same CI-parity runners the cycle audit uses (ADR-0069), not
// a new gate implementation.
```

### `go/internal/ciparity/composedgates_test.go:8` — above `func TestMissingComposedGates_FullNativeGateSet(t *testing.T) {`

```text
// TestMissingComposedGates_FullNativeGateSet names MissingComposedGates and
// RequiredComposedGates (cycle-786): the composed-tree gate contract for the
// trivial-rebase carry-forward — absent keys and non-"pass" values both keep
// the fast path closed; only the complete green set returns nil.
```

### `go/internal/ciparity/composedgates_test.go:39` — above `func TestGateStatuses_ProjectsStatusOnly(t *testing.T) {`

```text
// TestGateStatuses_ProjectsStatusOnly names GateOutcome and GateStatuses
// (composed-gate-decline-coded-signal, cycle 1772): a gate runner widened to
// carry each gate's captured output tail must still be able to derive the
// status-only map MissingComposedGates and the ledger's GateResults expect.
```

### `go/internal/ciparity/coverage_tagparity_test.go:3` — above `import (`

```text
// Cycle-555 RED tests for the SOLE `## top_n` task per triage-report.md:
//
//	coverage-gate-tag-parity — close the residual gap left by the already-landed
//	ciparity.CoverageTestArgs/CoverageTags SSOT (commit 78d73f08): the SSOT tag
//	set is "integration" only, but the repo puts real coverage behind BOTH
//	//go:build integration AND //go:build acs tags. Four internal/** packages
//	(core, acssuite, phases/audit, evalgate) carry in-package `//go:build acs`
//	tests, so a coverage run through the SSOT with the acs tag MISSING under-
//	counts them — the exact defect class the SSOT was created to kill (R1:
//	knowledge-base/research/test-coverage-audit-2026-07.md — 47.0% plain vs
//	90.6% tagged for internal/phases/ship). The Builder makes these GREEN by
//	adding `acs` to CoverageTags (→ "integration acs"); the Builder MUST NOT
//	edit this test file.
//
// These are in-package behavioral tests (they call the SSOT and, for the
// regression, run a real `go test -coverprofile` through it over a hermetic
// tag-gated fixture module). The cycle-555 ACS predicates in
// go/acs/cycle555/predicates_test.go gate on THESE via a `go test` subprocess.
```

### `go/internal/ciparity/graduation_prescription.go:3` — above `import (`

```text
// graduation_prescription.go — cycle-1329 (audit-warn-prescription-gate):
// GraduationPrescription is the shared home for "what does an ungraduated
// package need" text, relocated here (exported) from the build-entry seam's
// package-private graduationPrescription (phase_bindings_graduation.go:81)
// so BOTH the build seam (core) and the audit seam (phases/audit) render the
// identical copy-pasteable fix. Both callers already import ciparity for
// NewUngraduatedPackages — the shared detection predicate — so this is the
// natural single home rather than audit importing core for one helper.
//
// Behavior is a byte-identical move: same two-step-per-package text, same
// "..."-suffixed recursive-pattern branch (a pattern names no single
// directory, so step 2 falls back to "EACH package the pattern covers"
// instead of a bogus ".../apicover_named_test.go" path).
```

### `go/internal/ciparity/graduation_prescription_test.go:3` — above `import (`

```text
// graduation_prescription_test.go — cycle-1329 RED contract for task
// audit-warn-prescription-gate (scout-report.md hypothesis 1): relocate the
// build-entry seam's `graduationPrescription` helper
// (go/internal/core/phase_bindings_graduation.go:81, currently unexported to
// package core) into this shared `ciparity` package as an EXPORTED
// `GraduationPrescription`, so BOTH the build seam (core) and the audit seam
// (phases/audit) can render the identical copy-pasteable fix for an
// ungraduated package — closing the gap where audit's offender line only
// said "add it + an apicover_named_test.go" while build's abort_reason
// spelled out the exact two edits.
//
// Contract under test (Builder implements; this file must not be modified):
//
//	func GraduationPrescription(pkgs []string) string
//
// Per-package output is a copy-pasteable two-step fix:
//  1. "append this line to go/.apicover-enforce:  <pkg>"
//  2. "create go/<dir>/apicover_named_test.go naming every exported symbol
//     of the package in a real assertion that executes it..."
//
// EXCEPT a recursive "..."-suffixed pattern names no single directory, so
// step 2 instead reads "add an apicover_named_test.go in EACH package the
// pattern covers..." (AC3 — the branch preserved verbatim from
// phase_bindings_graduation.go:86-92, the exact behavior this relocation
// must not regress).
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive : TestGraduationPrescription_EmitsAppendLineAndTestPath
//   - Negative : TestGraduationPrescription_EmptyInputReturnsEmptyString
//   - Edge     : TestGraduationPrescription_PatternSuffixSkipsBogusPath (AC3:
//     the "..." recursive-pattern branch must not degrade to a bogus
//     ".../apicover_named_test.go" path)
//   - Semantic : TestGraduationPrescription_MultiplePackagesEachGetOwnBlock
//     (distinct behavior from the single-package case, not a restatement)
```

### `go/internal/ciparity/newpkg_test.go:3` — above `import (`

```text
// newpkg_test.go — RED contract for cycle-547's
// apicover-new-package-graduation-gate task.
//
// PROBLEM (scout Key Finding 3, 3rd confirmed recurrence of
// warnship_apicover_ci_gap): apicoverEnforceChangedDefault computes
// `touched := IntersectEnforced(changed, enforceBytes)` — a brand-new
// go/internal/<pkg> is in `changed` but, being new, cannot yet be listed in
// .apicover-enforce, so the intersection silently drops it and the
// unnamed-export gate never inspects it.
//
// FIX CONTRACT (new surface this cycle — undefined until Builder adds it, so
// this package's test build fails to compile today; that compile failure IS
// the RED evidence):
//
//	NewUngraduatedPackages(changed []string, enforceBytes []byte) []string
//	returns the changed package patterns that (a) live under go/internal/
//	(package go/cmd/... entrypoints are never apicover-scoped, so must NEVER
//	be flagged) and (b) are NOT present in the enforce list — i.e. exactly
//	the set IntersectEnforced silently drops. Normalizes the same way
//	IntersectEnforced does (strips a trailing "/..."), dedupes, and returns a
//	sorted slice (nil when nothing is ungraduated).
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive : TestNewUngraduatedPackages_FlagsChangedInternalPkgAbsentFromEnforceList
//   - Negative : TestNewUngraduatedPackages_AlreadyGraduatedPackagesNotFlagged
//     (the strongest anti-no-op: a naive "flag everything changed" impl fails
//     this — already-enforced packages must NOT reappear as "new")
//   - Edge     : TestNewUngraduatedPackages_CmdPackagesNeverFlagged (go/cmd/...
//     is out of apicover's scope entirely, per the AC)
//   - Edge     : TestNewUngraduatedPackages_DedupesAndSorts
```
