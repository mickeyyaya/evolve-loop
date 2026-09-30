# Comment history: `acs/cycle1152`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1152/predicates_test.go:3` — above `package cycle1152`

```text
// Package cycle1152 materialises the acceptance criteria for the single task
// triage committed to THIS cycle:
//
//   - artifact-name-ssot-remaining-callsites → route every remaining
//     hand-rolled report-filename literal in go/internal through the
//     phasecontract SSOT (ArtifactName / ArtifactFilename), completing the
//     migration cycle-1145 started and cycle-1149 left half-done.
//
// The deferred id (artifact-name-ssot-grep-guard) carries ZERO predicates —
// R9.3: predicates bind only to triage-committed work, and a predicate gating
// deferred work starves the committed task (the cycle-280 failure mode).
//
// Continuation context (ADR-0076). This worktree is a salvage continuation of
// cycle-1149, which already migrated SIX of the eight target files
// (consensusdispatch, core/phase_bindings, core/build_removal_check,
// coherence, phases/audit, phases/build). TWO literals survive and are what
// this cycle must close:
//
//	go/internal/phases/tdd/tdd.go:38      — the ArtifactFilename hook returns
//	                                        the literal, though the file already
//	                                        imports phasecontract.
//	go/internal/phases/ship/manifest.go:53 — the manifest list, whose comment
//	                                        wrongly claims "test" has no
//	                                        registry phase (it does: "tdd").
//
// Predicate strategy. This is a pure refactor: the literals currently EQUAL
// the registry's values, so no runtime observation can distinguish "reads the
// SSOT" from "carries an equal copy" — duplication is inherently a source-level
// property. The suite therefore pairs the two axes so neither half is gameable
// alone (go/acs/README.md sanctioned absence-check form, and the cycle-1147
// 001+005 precedent for this same task family):
//
//   - 001 is BEHAVIORAL over the SSOT itself. It calls ArtifactName /
//     ArtifactFilename and asserts their return values, including the phases
//     whose registry name DIVERGES from the "<phase>-report.md" convention.
//     It is the pairing anchor: a builder who greens 002-004 by deleting or
//     renaming the registry's tdd contract fails here.
//   - 002 is the duplication-ABSENCE check over the two surviving call sites.
//     RED today at both.
//   - 003 is the repo-wide invariant AC, enforced by PARSING go/internal with
//     go/parser and inspecting string literal AST nodes — not grep. Prose in
//     comments and error messages that merely mentions a report name is
//     correctly ignored; only a literal that IS the filename trips it. RED
//     today with exactly the two findings above.
//   - 004 is the anti-gaming NEGATIVE half. The obvious wrong fix — replacing
//     the literal with the hand-rolled `phase + "-report.md"` convention —
//     would green 002/003 while silently breaking tdd (whose registry name is
//     "test-report.md", not "tdd-report.md"). 004 rejects that fix, and also
//     pins the six files cycle-1149 already migrated so the salvaged work
//     cannot regress inside this cycle.
//   - 005 is BEHAVIORAL over the toolchain: `go build ./...` must succeed,
//     materialising the "no new import cycles" criterion.
```

### `go/acs/cycle1152/predicates_test.go:91` — above `var alreadyMigrated = []string{`

```text
// alreadyMigrated are the six files cycle-1149 salvaged. 004 pins them so this
// cycle cannot regress work it inherited.
```

### `go/acs/cycle1152/predicates_test.go:190` — above `func TestC1152_003_no_report_filename_literals_outside_phasecontract(t *testing.T) {`

```text
// TestC1152_003_no_report_filename_literals_outside_phasecontract is the
// repo-wide invariant AC and the regression guard against this drift class
// recurring a third time (cycle-1145 → cycle-1149 → here).
//
// It PARSES every non-test .go file under go/internal and inspects string
// literal AST nodes, so a comment or an error message that merely mentions a
// report name — of which the tree has dozens, legitimately — is not a finding.
// Only a literal whose value IS the filename counts: that is a path being
// constructed, which is exactly what must route through the registry.
//
// The phasecontract package is exempt: it is the SSOT and the one place the
// names are allowed to be typed.
```

### `go/acs/cycle1152/predicates_test.go:267` — above `func TestC1152_004_no_handrolled_convention_and_no_regression(t *testing.T) {`

```text
// TestC1152_004_no_handrolled_convention_and_no_regression is the negative,
// anti-gaming half.
//
// The tempting wrong fix for tdd.go is `string(core.PhaseTDD) + "-report.md"`,
// which greens 002 and 003 while producing "tdd-report.md" — a file the agent
// never writes, which is precisely the exit-81 timeout the tdd.go doc comment
// records as already having happened once. This predicate forbids that shape at
// the migrated sites.
//
// It also pins the six files cycle-1149 already migrated: they must still call
// the SSOT and must not have reacquired a literal. Without this, "completing the
// migration" could silently trade one set of literals for another.
```
