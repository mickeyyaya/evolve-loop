---
score_cap:
  - criterion: "acsassert.FilePresent exists as a pure path-bool (no testing.TB / Errorf)"
    max_if_missing: 7
    evidence: "grep -q 'func FilePresent(path string) bool' go/pkg/acsassert/assertions.go"
  - criterion: "FilePresent unit tests present and green"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run TestFilePresent ./pkg/acsassert/..."
  - criterion: "Zero FileExists-as-skip-guard sites remain under go/acs/"
    max_if_missing: 7
    evidence: "test $(grep -rc '!acsassert\\.FileExists(t,' go/acs/ --include='*.go' 2>/dev/null | awk -F: '{s+=$2} END {print s+0}') -eq 0"
  - criterion: "acs/cycle106 stale v12.1.1 version pin no longer fails CI"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 ./acs/cycle106/..."
---

# Eval: Fix acsassert FileExists-as-skip-guard false-green

> Pins the cycle-192 HIGH correctness fix to the acs assertion layer. `FileExists`
> was doing double duty as both a failing assertion (logs `Errorf`) and a skip
> guard, producing a spurious `Errorf` "double-signal" even when a clean skip on a
> missing artifact was correct — a false-green class visible on clean clones
> (cycle57/66). The fix is a pure path-bool `acsassert.FilePresent` (no `testing.TB`,
> so structurally incapable of logging a failure), migrating the 111 acs skip-guard
> sites to it while leaving real `FileExists(t, …)` assertions untouched, plus
> correcting the stale `acs/cycle106` v12.1.1 version-pin predicate to skip when the
> binary is not that release. Source incident: cycle 192 intent AC1–AC4.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| pure-bool-present | FilePresent exported as `func(path string) bool` | 7/10 | `grep -q 'func FilePresent(path string) bool' …/assertions.go` |
| filepresent-green | FilePresent unit tests pass | 6/10 | `go test -run TestFilePresent ./pkg/acsassert/...` |
| migration-complete | 0 `!acsassert.FileExists(t,` skip-guard sites in go/acs/ | 7/10 | grep-count == 0 |
| cycle106-pin-fixed | cycle106 predicates green (stale pin gone) | 5/10 | `go test ./acs/cycle106/...` |
