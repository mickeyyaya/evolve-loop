---
score_cap:
  - criterion: "internal/verifyeval.shellWords is <=50 lines (sizeratchet.MaxLines), measured by the ratchet's own AST-based scanner"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1749_001_ThreeTargetFunctionsFitTheRatchetLimit ./acs/cycle1749"
  - criterion: "go/internal/sizeratchet/offenders.json is byte-unchanged since b5d63a40 and still lists internal/verifyeval.shellWords at 51 — an allowance is a ceiling, so a lane never edits the file"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1749_002_OffendersJSONLeftUnchanged ./acs/cycle1749"
  - criterion: "The module-wide sizeratchet.Check reports zero problems — no extracted helper lands past 50 lines unlisted"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1749_003_ModuleWideRatchetCheckPasses ./acs/cycle1749"
  - criterion: "Every baseline *_test.go under go/internal/verifyeval is unmodified and undeleted; characterization tests go in new _test.go files"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1749_004_BaselineTestFilesUnchanged ./acs/cycle1749"
  - criterion: "go test -count=1 ./internal/verifyeval passes"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1749_005_TargetPackageTestsPass ./acs/cycle1749"
  - criterion: "go vet and gofmt are clean on go/internal/verifyeval"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1749_006_TargetPackagesVetAndGofmtClean ./acs/cycle1749"
  - criterion: "No comment line is added under go/internal/verifyeval — `commentaudit comments -base b5d63a40` exits 0 (production and test files alike)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1749_007_NoCommentLinesAdded ./acs/cycle1749"
  - criterion: "A non-test source under go/internal/verifyeval changed, and no baseline comment line was deleted from it"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1749_008_ShrunkSourcesChangedWithoutLosingAComment ./acs/cycle1749"
  - criterion: "The verifyeval suite passes with the baseline shellWords substituted back and kills all 7 behavior mutants of it (tab/CR separators, no empty words, literal backslash in single quotes, escaping backslash in double quotes, quotes stripped, quoted whitespace kept, escaped character kept)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1749_011_ShellWordsCharacterizationKillsBaselineMutants ./acs/cycle1749"
  - criterion: "shellWords returns the same words as its baseline for representative commands and every string of up to 5 characters over {a, space, tab, CR, LF, ', \", backslash}"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1749_012_ShellWordsMatchesBaselineOverAnExhaustiveCorpus ./acs/cycle1749"
  - criterion: "No file under a protected surface changed since b5d63a40"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1749_013_NoProtectedSurfaceTouched ./acs/cycle1749"
---

# Eval: shrink-verifyeval-shellwords

> Pins the behavior-preserving extraction of the unexported tokenizer
> `internal/verifyeval.shellWords` (51 lines at base `b5d63a40`, one over the
> ratchet) to <=50 lines. Characterization tests come first, covering quoted
> args, escaped spaces and the empty string. `offenders.json` stays
> byte-unchanged, per the goal's wave-25 ceiling rule, which overrides the
> scout's "drop the entry". The baseline suite reaches `shellWords` only
> through `hasNarrowedGoTest`, and a probe of 9 mutants found 8 that it does
> not kill. One of them (dropping `inWord = true` on an opening quote) is
> near-equivalent, differing only on a lone unterminated quote, so it was
> removed as a quirk. The other 7 are each killed by a throwaway table test
> that passes on the baseline; in an isolated scratch clone with a trial
> extraction, all 13 cycle-1749 predicates went GREEN. Predicate 012 is a
> differential check: an overlay-injected test compares the current
> `shellWords` with its baseline text over about 37k inputs. The mutation
> probe substitutes mutants of the *baseline* text into the current package
> through `go test -overlay`. Only `shellWords` must keep its name and
> signature, and its characterization tests must call `shellWords` itself.
> Source: cycle 1749 triage top_n `shrink-verifyeval-shellwords`. There is no
> inbox record, and the Task Contract names this eval as the authority.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| line-limit | shellWords <=50 lines | 8/10 | `go test -run TestC1749_001...` |
| ceiling-untouched | offenders.json byte-unchanged | 8/10 | `go test -run TestC1749_002...` |
| ratchet-gate | module-wide ratchet Check passes | 9/10 | `go test -run TestC1749_003...` |
| test-files-frozen | baseline verifyeval tests unmodified | 7/10 | `go test -run TestC1749_004...` |
| behavior-preserved | `go test ./internal/verifyeval` passes | 8/10 | `go test -run TestC1749_005...` |
| vet-clean | vet + gofmt clean | 6/10 | `go test -run TestC1749_006...` |
| no-comments-added | commentaudit lists zero added lines | 8/10 | `go test -run TestC1749_007...` |
| no-comments-deleted | source changed, no baseline comment lost | 7/10 | `go test -run TestC1749_008...` |
| characterization | suite kills 7 baseline shellWords mutants | 8/10 | `go test -run TestC1749_011...` |
| differential | matches baseline over ~37k inputs | 9/10 | `go test -run TestC1749_012...` |
| scope | no protected surface touched | 6/10 | `go test -run TestC1749_013...` |
