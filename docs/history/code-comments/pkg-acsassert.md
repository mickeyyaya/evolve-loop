# Comment history: `pkg/acsassert`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d5

### `go/pkg/acsassert/assertions.go:1` — above `package acsassert`

```text
// Package acsassert is the assertion DSL ACS predicates write against.
//
// Each Go predicate under go/acs/{cycle<N>,regression,redteam}/ uses these
// helpers in the same way the retired bash predicates used [ -f ], grep -q,
// jq -e, etc. (ADR-0042). The helpers
// take a TB instead of *testing.T so they're testable in isolation
// (the test for FileExists doesn't have to fail itself when checking
// missing-file behaviour).
```

### `go/pkg/acsassert/assertions.go:84` — above `func FileNotContains(tb TB, path, substring string) bool {`

```text
// FileNotContains is the correct primitive for an ABSENCE assertion (e.g. "a
// removed flag no longer appears in this source file"). It returns true and
// logs nothing when the substring is absent, and logs an Errorf + returns false
// when the substring is PRESENT (or the file cannot be read).
//
// Use this instead of inverting FileContains. FileContains is a POSITIVE
// assertion that Errorf's "missing" when the substring is absent, so the idiom
// `if FileContains(tb, f, flag) { tb.Errorf("present") }` red-fails on the
// CORRECT (absent) state: the FileContains call still fires its internal Errorf
// even though the if body is skipped (cycle-352 broken-predicate incident).
```

### `go/pkg/acsassert/assertions_test.go:86` — above `func TestFileNotContains_Absent(t *testing.T) {`

```text
// FileNotContains — the correct ABSENCE primitive (cycle-352: authors inverted
// FileContains, which Errorf's "missing" on the absent/correct state) ----------
```

### `go/pkg/acsassert/assertions_test.go:426` — above `func TestCountInGoFunc(t *testing.T) {`

```text
// TestCountInGoFunc — function-scoped occurrence counting. Kills the ACS
// count-rot class: file-wide counts (CountOccurrencesAny == N) break when a
// LATER cycle legitimately adds an occurrence elsewhere in the file
// (cycle-296's IsAbs predicate rotted exactly this way within one day,
// refreshed 1→2 in 0c210b52). Scoping the assertion to the function whose
// body carries the invariant ("addWorktree contains NO IsAbs guard") makes
// it immune to unrelated growth — and a missing/renamed function is a LOUD
// error, never a silent 0.
```

### `go/pkg/acsassert/assertions_test.go:475` — above `func TestCountInGoFunc_MethodReceiver(t *testing.T) {`

```text
// TestCountInGoFunc_MethodReceiver — the only real-world call site
// (cycle-296's addWorktree predicate) targets a METHOD; FuncDecl.Name is the
// bare method name, receiver ignored.
```
