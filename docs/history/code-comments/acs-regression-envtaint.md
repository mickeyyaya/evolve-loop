# Comment history: `acs/regression/envtaint`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/regression/envtaint/antirename_test.go:14` — above `func TestGetenvKeysFromSrc_AllPrefixesFoldsDynamicExcluded(t *testing.T) {`

```text
// GetenvKeysFromSrc underlies the anti-rename invariant (ADR-0064 Pillar 2, M2):
// it collects every compile-time-constant key passed to os.Getenv / os.LookupEnv
// in src, at ANY prefix (so a dial renamed out of the EVOLVE_ namespace is
// visible), folding split-consts (so "HO"+"ME" -> HOME and the rename dodge
// "FO"+"O" cannot hide), and dropping dynamic (non-constant) keys.
```

### `go/acs/regression/envtaint/gate_test.go:55` — above `func TestCycle20Dodge_OrphanedWhenRowDeleted(t *testing.T) {`

```text
// TestCycle20Dodge_OrphanedWhenRowDeleted replays the exact cycle-20 metric dodge
// and proves the gate catches it: a split-const reader of an operator dial keeps
// the dial working byte-identically while vanishing from the go/ast literal scan,
// so the registry row could be deleted unnoticed. The fold-aware read-set still
// contains the key, so deleting the row leaves a detectable orphan.
```

### `go/acs/regression/envtaint/readset.go:36` — above `func EvolveConstKeys(src string) ([]string, error) {`

```text
// EvolveConstKeys returns the sorted, de-duplicated set of EVOLVE_* operator-dial
// keys that the source reads as compile-time constants — the read-set R for one
// file. It folds split-consts (so the cycle-20 dodge "EVOLVE_"+"X" is visible),
// excludes keys whose declaration carries the IPC-allowed marker, and excludes
// dynamic (non-constant) keys.
//
// Type-checking is best-effort: imports are stubbed and errors are swallowed, so
// a whole-repo walk can fold every file without resolving the build graph (an
// unresolvable "os" does not abort the constant fold of an argument).
```

### `go/acs/regression/envtaint/taint.go:3` — above `package envtaint`

```text
// Package envtaint is the constant-folding + env-source taint harness for the
// honest flag-metric gate (Pillar 2 of ADR-0064, the pipeline-integrity
// boundary).
//
// The existing flag-reader guard (go/acs/regression/flagreaders) scans with
// go/ast and strconv.Unquote, so it only sees *ast.BasicLit string literals.
// Cycle 20 gamed that by writing os.Getenv("EVOLVE_" + "WORKTREE_BASE"): the
// dial kept working byte-for-byte while the literal "EVOLVE_WORKTREE_BASE"
// vanished from every grep/AST scan, so the registry row could be deleted with
// no guard objecting.
//
// This harness type-checks source with go/types instead. The type-checker runs
// the language spec's constant folding, so "EVOLVE_" + "WORKTREE_BASE" is
// already the single string value "EVOLVE_WORKTREE_BASE" by the time we inspect
// it — the split-const dodge is transparent. It also lets us tell a
// compile-time-constant os.Getenv argument (a real, countable operator dial)
// from a dynamic one (a non-constant key, not a fixed dial).
//
// S0 (this file) is the scaffold: load/fold + os.Getenv/LookupEnv classification.
// Later slices build the carrier read-set R and the derived registry on top.
```

### `go/acs/regression/envtaint/taint_test.go:7` — above `func TestLoad_FoldsConcatenatedStringConstant(t *testing.T) {`

```text
// These tests pin the two capabilities the flag-metric harness MUST have that
// the existing go/ast literal scanner (flagreaders) lacks, and which let
// cycle-20 game the metric:
//
//  1. See through a split-const dodge: `"EVOLVE_" + "WORKTREE_BASE"` is an
//     *ast.BinaryExpr, invisible to a strconv.Unquote literal scan. The
//     type-checker constant-folds it; the harness must report the folded value.
//  2. Distinguish a compile-time-constant os.Getenv argument (a real, countable
//     operator dial) from a non-constant one (dynamic key — not a fixed dial).
```
