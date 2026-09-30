# Comment history: `acs/cycle1730`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1730/predicates_test.go:122` — above `func TestC1730_003_ModuleWideRatchetCheckPasses(t *testing.T) {`

```text
// TestC1730_003_ModuleWideRatchetCheckPasses — the repo-wide ratchet gate
// (sizeratchet.Check over every function in the module against the loaded
// offenders map) must report zero problems, covering both directions of the
// cheapest gaming fake: deleting the entries without shrinking the code trips
// this because Check flags an unlisted function over the limit. Since
// 2026-09-28 an allowance is a ceiling, so shrinking without deleting is slack
// here and only the _002 key-absence check catches it. Currently green (the allowances match the current
// oversized code) — a guardrail predicate that must stay green throughout the
// build phase, not a RED-today check.
```

### `go/acs/cycle1730/predicates_test.go:148` — above `func TestC1730_004_OpscmdTestFilesUnchangedFromBaseline(t *testing.T) {`

```text
// TestC1730_004_OpscmdTestFilesUnchangedFromBaseline — acceptance criterion
// 2, first clause: "the package's existing tests pass unmodified". Every
// *_test.go under go/internal/cli/opscmd that exists at the baseline must be
// byte-identical in the worktree (no modification, deletion, or rename),
// which rules out weakening an existing assertion to paper over a behavior
// change. Added test files are excluded (--diff-filter=a) because the same
// criterion's second clause REQUIRES a new characterization test for any
// function whose behavior no existing test pins (audit round 1, M2).
```

### `go/acs/cycle1730/predicates_test.go:194` — above `func TestC1730_007_NoCommentLinesAddedToOpscmd(t *testing.T) {`

```text
// TestC1730_007_NoCommentLinesAddedToOpscmd — acceptance criterion 3: "No
// comments are added (docs/conventions/code-comments.md); names carry the
// intent". Runs the repo's own grader for that convention in-process —
// commentaudit.Main, i.e. `commentaudit comments -base <baseline>
// <opscmd dir>` — which lists every non-directive comment line the diff adds
// (a line moved within its file is not "added") and exits 1 when any exist.
// RED in audit round 1: 54 added doc-comment lines on the new unexported
// helpers.
```

### `go/acs/cycle1730/predicates_test.go:260` — above `func TestC1730_008_TargetFunctionDocsMatchBaseline(t *testing.T) {`

```text
// TestC1730_008_TargetFunctionDocsMatchBaseline — acceptance criterion 3 and
// audit round 1 finding L1: the extraction must not rewrite, trim, or scatter
// the doc comment of any of the eight target functions (round 1 moved the
// exit-code tables of RunRollback/RunReleasePipeline/RunMarketplacePoll into
// unexported helpers godoc never shows, leaving dangling fragments). Each
// target's go/ast doc text in the worktree must equal its baseline doc text.
//
// acs-predicate: config-check — the doc comment text IS the contract under
// test (comments have no runtime behavior); parsed with go/parser, not grepped.
```

### `go/acs/cycle1730/predicates_test.go:292` — above `var doctorMutants = []struct{ name, anchor, replacement string }{`

```text
// doctorMutants are behavior changes to the doctor live/boot result reporting
// that the pre-existing opscmd tests do not detect (audit round 1, M2). Each
// keeps the source compiling and vet-clean, so a characterization test that
// pins the reported messages, JSON field names, and pane-tail length must fail
// on every one.
```

### `go/acs/cycle1730/predicates_test.go:309` — above `func TestC1730_009_DoctorCharacterizationTestsKillReportMutants(t *testing.T) {`

```text
// TestC1730_009_DoctorCharacterizationTestsKillReportMutants — acceptance
// criterion 2, second clause: "a function without a test that pins its
// behavior gets a characterization test first, red on a mutant". The only
// pre-existing test reaching runDoctorLive/runDoctorBoot's OK/WALLED/FAILED
// reporting is an rc-in-{1,10} smoke (audit round 1, M2). The opscmd tests
// matching ^TestDoctorCharacterization must run and pass on the real code,
// and must FAIL (a test failure, not a build failure) when `go test -overlay`
// swaps in each doctorMutants source mutation.
```

### `go/acs/cycle1730/predicates_test.go:425` — above `func TestC1730_010_NoBaselineCommentDeletedFromOpscmd(t *testing.T) {`

```text
// TestC1730_010_NoBaselineCommentDeletedFromOpscmd — acceptance criterion 3
// read with docs/conventions/code-comments.md:43 (audit round 2, M1): a pure
// extraction adds no comment AND keeps every baseline comment, so a *why*
// such as console_lease.go's interspersed-parse rationale is never stripped
// with its knowledge left nowhere. Runs the repo's own comment-line grader,
// commentaudit.AddedComments, with the sides swapped: a non-directive comment
// line the baseline versions of the changed opscmd sources hold and their
// current versions lack is deleted. Moves within or across the changed files
// are not deletions. RED in audit round 2: the 5-line interspersed-parse
// comment of RunConsoleLease.
//
// acs-predicate: config-check — comment text IS the contract under test
// (comments have no runtime behavior); graded by commentaudit, not grepped.
```
