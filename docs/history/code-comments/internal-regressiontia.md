# Comment history: `internal/regressiontia`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero h1

### `go/internal/regressiontia/apicover_named_test.go:3` — above `import (`

```text
// apicover_named_test.go — the ADR-0069 repo-wide apicover gate's second half.
// Enrolling ./internal/regressiontia in go/.apicover-enforce is not enough:
// every exported symbol must be NAMED here in an assertion that EXECUTES it,
// or the enrolled-but-unnamed shape fails the gate.
//
// These are not restatements of regressiontia_test.go's contract tests. Each
// one exercises a path that contract file leaves cold — the enforce stage, real
// dependency resolution against this repository's own regression corpus, the
// scope-driven partition, and the marshalling round trip through Emit's real
// filesystem write.
```

### `go/internal/regressiontia/regressiontia.go:90` — above `func ChangedScope(repoRoot string, changed []string) []string {`

```text
// ChangedScope widens a cycle's changed-package set with its REVERSE
// dependencies, so a change confined to internal/router selects
// internal/routingtest — which imports it and holds the keystone parity
// invariant. Forward-only scope marks the routing regression package skippable
// and hides exactly the class that kept main red for 5 commits (cycle-1250).
//
// Best-effort, inheriting changedpkgs.ImporterClosure's contract: an empty
// repoRoot or any `go list` failure degrades to the input set unchanged.
// Closure only ever widens; narrowing below the forward-only baseline would be
// strictly worse than having no selection at all. Since 2026-09-14 the closure
// also follows the DIRECT imports of each package's tests (an untouched test
// asserting a changed package's contract is the ship-gate incident), so a
// change selects more predicates — and skips fewer — than before.
```

### `go/internal/regressiontia/regressiontia_test.go:3` — above `import (`

```text
// regressiontia_test.go — RED contract for cycle-1260 Task 1
// (`egps-regression-tia-shadow-wiring`, inbox item
// .evolve/inbox/2026-07-30T09-00-00Z-egps-regression-tia-selection.json,
// P1 weight 0.91, 3rd live instance).
//
// WHY THIS PACKAGE EXISTS (deviation from scout-report's target files, recorded
// loudly per Core Rule 3). Scout targeted go/internal/acssuite/acssuite.go. That
// path is PROTECTED CONTROL PLANE — guards.ProtectedSurfaceManifest carries
// {Fragment: "/go/internal/acssuite/", Rationale: "the gate runner"} — so no
// phase of any cycle may write it (the role guard denies and ALARMS; verified
// live this phase). A cycle may not edit the gate that grades it.
//
// The boundary is not an obstacle to route around, it is a design constraint,
// and honoring it yields the better architecture: the SHADOW stage changes
// nothing about what the gate runs (that is the definition of shadow), so it
// needs no code in the gate runner at all. It is pure observability, computed
// beside the suite by the suite's own production CALLER — the audit phase's
// generateACSVerdict (internal/phases/audit/audit.go:651). Only the future
// `enforce` stage, which actually skips packages, must live inside acssuite,
// and that change is human-gated `evolve ship --class manual` OUTSIDE a cycle
// by construction. Scout's Task-1 acceptance intent is preserved verbatim:
// off/absent is byte-identical, shadow logs would-skip counts, nothing is
// skipped yet.
//
// The contract these tests freeze:
//
//	const ArtifactName = "acs-tia-shadow.json"
//	type Decision struct{ Stage; ChangedPackages; Selected; WouldSkip; WouldSkipCount }
//	func Select(patterns, scope []string, deps map[string][]string) (selected, wouldSkip []string)
//	func ChangedScope(repoRoot string, changed []string) []string
//	func Compute(stage, repoRoot, moduleDir string, changed []string) Decision
//	func Emit(workspace string, d Decision) (string, error)
//
// Safety invariant carried from the inbox item — selection must never be able
// to hide a regression class — so every fail-safe resolves toward RUNNING a
// predicate: unknown scope ⇒ skip nothing, unknown deps ⇒ skip nothing,
// unknown stage ⇒ off.
//
// RED today: package regressiontia does not exist, so this file fails to
// COMPILE — a hard non-zero exit, never a silent pass.
```

### `go/internal/regressiontia/regressiontia_test.go:128` — above `func TestChangedScope_WidensByReverseDependency(t *testing.T) {`

```text
// TestChangedScope_WidensByReverseDependency is the ImporterClosure wiring
// proof and the cycle-1250 reproducer. A change confined to internal/router
// must widen to internal/routingtest, which imports it (agent.go/bricks.go/
// engine.go) and holds the keystone parity invariant. Forward-only scope marks
// the routing regression package skippable and hides exactly the class that
// kept main red for 5 commits. Run against the REAL repository import graph,
// so it fails if ChangedScope forwards its input instead of calling
// changedpkgs.ImporterClosure.
```

### `go/internal/regressiontia/regressiontia_test.go:273` — above `func TestChangedScope_SelectsTestOnlyImporters(t *testing.T) {`

```text
// TestChangedScope_SelectsTestOnlyImporters pins the 2026-09-14 widening at
// this consumer: internal/routingeval's tests import internal/core (its build
// deps do not), so a change confined to core selects routingeval's scope —
// a build-deps-only closure left that regression class skippable.
```
