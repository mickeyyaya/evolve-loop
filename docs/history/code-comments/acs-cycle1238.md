# Comment history: `acs/cycle1238`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1238/predicates_test.go:3` — above `package cycle1238`

```text
// Package cycle1238 materialises the acceptance criteria for this lane's single
// fleet-scoped task, `wire-reachability-gate-into-tdd-verify` (inbox item
// tdd-structural-test-reachability-probe, weight 0.92, root cause cycle-644).
//
// What already landed (scout-report.md): the `reachabilityprobe` library
// (CheckCallSite/BuildImportGraph), the `evolve reachability check-pin`
// subcommand, and the obligation text in agents/evolve-tdd-engineer.md:132.
// What has NOT landed — and is the whole point of this cycle — is a
// DETERMINISTIC caller: nothing in the phase-gate pipeline runs the probe, so
// the cycle-644 shape is caught only if the TDD agent remembers to probe by
// hand. That is the exact LLM judgment lapse cycle-644 already demonstrated.
//
// The cycle-644 shape, restated so the predicates below are readable: a frozen
// (`doNotModifyTests: true`) structural test pinned `storage.UpdateStateMap(`
// as a required call site inside a file belonging to package `core`, while
// `storage` already imported `core`. Satisfying that pin would require
// core -> storage -> core: a compiler-proven import cycle, so the acceptance
// criterion was permanently unsatisfiable and the cycle burned.
//
// Predicate strategy — every predicate exercises a REAL production path, never
// a source-grep of production code (the cycle-85 degenerate-predicate ban):
//
//   - 001/002/003 drive the REAL CLI entry point: a freshly built `evolve`
//     binary running `phase verify tdd --workspace ... --worktree ...` over a
//     real fixture Go module, asserting the exit code and stderr an operator
//     actually sees. This is the wiring proof — a gate reachable only from a
//     unit test is dead code (House Rule 2).
//   - 001 is the crux REJECTION case (cycle-644 shape must be flagged).
//   - 002 is the false-positive regression guard (a reachable pin passes
//     unchanged) — the inbox item's explicit acceptance criterion #3.
//   - 003 is the edge/fail-open axis: an unfrozen handoff and a worktree with
//     no Go module must both leave the verdict untouched.
//   - 004 exercises the new library seam directly (extraction + resolution +
//     handoff parsing), so a CLI regression and a library regression are
//     distinguishable.
//   - 005 is House Rule 1's second half: `internal/reachabilityprobe` is
//     already enrolled in go/.apicover-enforce (line 495), so every NEW
//     exported symbol must be named and executed in its apicover_named_test.go.
//
// RED at authoring time is a COMPILE failure for 004 (the three new exported
// symbols do not exist yet) and a behavioural failure for 001 (the CLI exits 0
// today: `grep -n reachability go/internal/cli/phasecmd/phase_verify.go`
// returns nothing).
```

### `go/acs/cycle1238/predicates_test.go:128` — above `func fixtureWorktree(t *testing.T) string {`

```text
// fixtureWorktree builds a throwaway worktree whose `go/` subdirectory is a
// real, resolvable Go module carrying BOTH shapes the gate must tell apart:
//
//	internal/storage  imports internal/core  → pinning storage.UpdateStateMap(
//	                                           inside a core file is the
//	                                           cycle-644 shape (a cycle).
//	internal/leafutil imports nothing        → pinning leafutil.Helper(
//	                                           inside a core file is fine.
//
// The frozen test files pin their call sites the way this repo's structural
// tests actually do — a source path plus a package-qualified needle on one line
// (the acsassert.FileContains idiom) — which is exactly the cycle-644 artefact:
// the pin is a REQUIREMENT on production code, not a call the test itself
// makes, so the fixture module stays buildable and `go list` stays clean.
```

### `go/acs/cycle1238/predicates_test.go:214` — above `func TestC1238_001_CLIFlagsCycle644FrozenPin(t *testing.T) {`

```text
// TestC1238_001_CLIFlagsCycle644FrozenPin is the CRUX and the negative
// (rejection) predicate: the cycle-644 shape must be a CONFIRMED violation on
// the live CLI path, before the build phase ever starts.
//
// RED today: `evolve phase verify tdd` has no reachability step at all
// (phase_verify.go has zero `reachability` references), so it exits 0 on this
// fixture and the unsatisfiable criterion sails through.
```

### `go/acs/cycle1238/predicates_test.go:262` — above `func TestC1238_003_GateScopeAndFailOpen(t *testing.T) {`

```text
// TestC1238_003_GateScopeAndFailOpen pins the two edge cases that keep the gate
// from becoming a new source of false HALTs:
//
//	(a) doNotModifyTests:false — the tests are NOT frozen, so no pin is a
//	    permanent commitment and the gate must not fire, even on the cycle-644
//	    fixture. This proves the gate keys off the freeze flag rather than
//	    scanning every test it can find.
//	(b) a worktree with no Go module — the import graph is underivable, and an
//	    infra gap must fail OPEN (phase_verify.go's standing philosophy:
//	    ambiguity never becomes a confirmed violation).
```

### `go/acs/cycle1238/predicates_test.go:377` — above `func TestC1238_006_PermanentRegressionGuard(t *testing.T) {`

```text
// TestC1238_006_PermanentRegressionGuard requires the DURABLE protection: an
// ordinary (non-acs) test in package phasecmd, named with the
// `TestPhaseVerifyTDD_FrozenPin` prefix, that drives runPhaseVerify over the
// cycle-644 shape. The acs predicates above are cycle-scoped and vanish with
// this cycle; without this guard the gate can silently rot in a later refactor
// and nothing in `go test ./...` notices. The docsfloor gate (cycle-1150,
// phase_verify_docsfloor_test.go) is the precedent to copy.
```
