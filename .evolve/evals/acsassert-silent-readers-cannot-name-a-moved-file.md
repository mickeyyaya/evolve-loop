---
score_cap:
  - criterion: "FileContainsAnyChecked, CountOccurrencesAnyChecked and LineContainsAllChecked each name the path and the moved-file possibility on a not-exist read, with an error satisfying errors.Is(err, fs.ErrNotExist)"
    max_if_missing: 7
    evidence: "cd go && go test -run 'TestFileContainsAnyChecked_MissingFileHintsAtRelocation|TestCountOccurrencesAnyChecked_MissingFileHintsAtRelocation|TestLineContainsAllChecked_MissingFileHintsAtRelocation' -count=1 ./pkg/acsassert/..."
  - criterion: "An existing path that simply lacks the content fails without the moved-file hint"
    max_if_missing: 6
    evidence: "cd go && go test -run 'TestFileContainsAnyChecked_ExistingFileNoHintOnMiss|TestCountOccurrencesAnyChecked_ExistingFileNoHintOnZero|TestLineContainsAllChecked_ExistingFileNoHintOnMiss' -count=1 ./pkg/acsassert/..."
  - criterion: "go/pkg/acsassert package tests pass in full (no regression to the existing value-only readers)"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 ./pkg/acsassert/..."
---

# Eval: acsassert silent readers cannot name a moved file

> Cycle 1702 gave every message-carrying acsassert reader (a TB parameter or an
> error result) a moved-file hint on a not-exist read. `FileContainsAny`,
> `CountOccurrencesAny` and `LineContainsAll` return only a bool/int, so on a
> missing path they silently return false/0 — indistinguishable from "file
> present, content absent" — and the calling predicate's own failure message
> says nothing about a moved or renamed file, across the 46 predicate files
> that call at least one of them. This eval pins the checked-variant fix
> (`*Checked` forms returning `(value, error)`, mirroring the existing
> `CountInGoFunc` convention) so a future regression is capped, not silently
> re-introduced. Source: inbox record
> `acsassert-silent-readers-cannot-name-a-moved-file` (split out of cycle 1702 /
> acs-cycle50-predicates-point-at-a-moved-file).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| missing-path-hint | Checked readers name path + moved hint, error wraps fs.ErrNotExist | 7/10 | `go test -run '...MissingFileHintsAtRelocation' ./pkg/acsassert/...` |
| existing-path-no-hint | Existing file lacking content fails with no moved hint | 6/10 | `go test -run '...NoHintOn(Miss\|Zero)' ./pkg/acsassert/...` |
| no-regression | Full acsassert suite still green | 8/10 | `go test ./pkg/acsassert/...` |
