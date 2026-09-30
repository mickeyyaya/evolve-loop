# Comment history: `acs/cycle1149`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1149/predicates_test.go:3` — above `package cycle1149`

```text
// Package cycle1149 materialises the cycle-1149 acceptance criteria for the one
// fleet-scoped task triage committed to THIS cycle:
//
//   - artifact-name-ssot-remaining-callsites → route the 7 remaining
//     "build-report.md" / "audit-report.md" string literals through
//     phasecontract.ArtifactFilename, the declared artifact-name SSOT.
//
// The deferred id (artifact-name-ssot-lint-guard) and the dropped one
// (manifest.go's "test-report.md") carry ZERO gating predicates for their own
// behavior (R9.3: predicates bind only to triage-committed work; a predicate
// gating deferred work starves the committed task — the cycle-280 failure
// mode). 005 is the exception that proves the rule: it pins the DROPPED half
// as a scope BOUNDARY the committed refactor must not cross, not as work.
//
// Predicate strategy. This is a behavior-preserving duplication-removal
// refactor: every literal resolves to exactly the string the registry already
// returns, so no runtime observation can distinguish "seven copies" from "one
// shared source" today. The predicates therefore split along the only axes
// that can actually observe the defect and its regressions:
//
//   - 001 is the CRUX and the only one that red-fails today: an absence check
//     over the production tree (the sanctioned form per go/acs/README.md
//     "Absence checks"), asserting the quoted literals survive in exactly ONE
//     declaration site — the registry.
//   - 002 is 001's ANTI-GAMING twin. The cheapest way to green an absence
//     check is to delete or rename the thing being counted, so 002 exercises
//     ArtifactFilename/ArtifactName directly and pins the runtime-truth values
//     plus both fallback edges (unregistered phase, NoArtifact ship).
//   - 003 and 004 are BEHAVIORAL regression guards over two of the seven
//     rewritten sites (core.RemovalClaimFailures, coherence.ReadCycleVerdicts),
//     each driven through its exported entry point with a positive case and a
//     wrong-filename NEGATIVE control that proves the positive is
//     filename-sensitive rather than trivially true. Pre-existing GREEN: they
//     stay green only if the substitution preserves the resolved value.
//   - 005 bounds the fix from the other side — the "test-report.md" half of
//     ship's manifest has no registry phase and must survive untouched.
//   - 006 pins the whole-tree compile, which is what proves the new
//     consensusdispatch → phasecontract import introduced no cycle.
//
// Roots: production source is read under acsassert.RepoRoot (the worktree —
// where Builder's change lands and is committed), never main.
```

### `go/acs/cycle1149/predicates_test.go:242` — above `func TestC1149_005_ShipManifestCoversTheTDDReport(t *testing.T) {`

```text
// TestC1149_005_ShipManifestCoversTheTDDReport bounds the fix from the other
// side: the SSOT sweep must not SHRINK ship's manifestReportFiles by dropping
// an entry it could not resolve.
//
// SUPERSEDED PREMISE (corrected in cycle-1152). This predicate originally
// asserted that the "test-report.md" entry must survive as a LITERAL, on the
// stated ground that "'test' has no phasecontract entry". That premise rested
// on a wrong registry key: the phase is named "tdd", not "test", and
// contract_registry.go:132 registers it with ArtifactName "test-report.md".
// ArtifactName("test") returns "" because "test" is not a phase at all — not
// because the name lacks an SSOT. The entry is therefore fully SSOT-able and
// cycle-1152 migrates it.
//
// What actually needs bounding is COVERAGE, not the literal: whatever ship's
// manifest names for the TDD phase must equal the registry's name for it.
```

### `go/acs/cycle1149/predicates_test.go:268` — above `if !strings.Contains(string(src), '"'+tddReport+'"') &&`

```text
// Coverage survives either as the literal (pre-cycle-1152) or, once
// migrated, as a phasecontract call — but it must not simply vanish.
```
