# Comment history: `acs/cycle1409`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1409/predicates_test.go:3` — above `package cycle1409`

```text
// Package cycle1409 materialises the cycle-1409 acceptance criteria for the two
// fleet-scoped tasks pinned to this lane (inbox item
// `repocontract-gate-false-red-swallowed-diag`):
//
//   - repocontract-gate-infra-ambiguity-classification → distinguish a genuine
//     repo-contract RED from an infra flake; retry once; new distinct ship code
//   - repocontract-scan-log-persistence                → tee scanner output to a
//     run artifact unconditionally + name the failing tests in the ship error
//
// The live defect. `defaultRepoContractTest` (go/internal/phases/ship/
// repocontract.go:48) returns `cmd.Run()`'s error verbatim, and
// `runRepoContractGate` wraps ANY non-nil error as CodeRepoContractGate. A
// build-cache contention / OOM-kill / module-fetch flake is therefore
// indistinguishable from a genuine red guard suite, and neither `cmd.Stdout`
// nor `cmd.Stderr` is teed to a run artifact — so the false RED that blocked
// cycles 1402/1403/1405 (preserved worktree e0638346 re-ran 4/4 GREEN against
// the identical tree; baseline cba017c5 also 4/4 GREEN) left no recoverable
// evidence.
//
// Predicate strategy — behavioral, never source-grep (the cycle-85
// degenerate-predicate ban). Each predicate DRIVES the system under test by
// running the named Go unit test in ONE named package with `-run` narrowing
// (per the flaky-predicate-shape rules: no `./...` sweeps, no wall-clock
// bounds, no literal PIDs, cmd.Dir always set explicitly) and asserting the
// `--- PASS: <TestName>` line is present. The PASS-line assertion is the
// anti-vacuous guard: `go test -run '^TestDoesNotExist$'` exits 0 with
// "no tests to run", so exit-code-only checking would pass on an EMPTY repo.
//
// Predicate map:
//
//	001 — the new infra ship code exists in the vocabulary, non-aliasing (T1)
//	002 — a genuine test failure stays CodeRepoContractGate, runs ONCE (T1)
//	003 — a transient first failure + green retry ships, runs EXACTLY twice (T1)
//	004 — persistent ambiguity → the infra code, EXACTLY twice, no retry storm (T1)
//	005 — the four pre-existing gate tests are still GREEN (anti-weakening)
//	006 — the scan log artifact is written on BOTH the green and red paths (T2)
//	007 — the RED ship error message NAMES the failing tests (T2)
//	008 — WIRING PROOF: the production caller (runNative) reaches the gate with
//	      the run workspace, so the log lands in <runs>/cycle-N (T2)
```

### `go/acs/cycle1409/predicates_test.go:146` — above `func TestC1409_006_ScanLogPersistedOnGreenAndRed(t *testing.T) {`

```text
// TestC1409_006_ScanLogPersistedOnGreenAndRed — AC6. The scanner output must be
// teed to <runs>/cycle-N/ship-repocontract-scan.log UNCONDITIONALLY. Green-only
// or red-only persistence is insufficient: diagnosing a false RED requires the
// green baseline too, and a red run whose log is only written on failure is
// exactly what was missing on cycle-1403.
```

### `go/acs/cycle1409/predicates_test.go:155` — above `func TestC1409_007_RedErrorNamesFailingTests(t *testing.T) {`

```text
// TestC1409_007_RedErrorNamesFailingTests — AC7. The failing test names parsed
// from the `go test -json` events must appear in the CodeRepoContractGate
// message so ship-error.json carries them directly, instead of the generic
// "scanner pack RED (exit status 1)" string that made cycle-1402/1403
// undiagnosable.
```

### `go/acs/cycle1409/predicates_test.go:164` — above `func TestC1409_008_ProductionCallerThreadsWorkspace(t *testing.T) {`

```text
// TestC1409_008_ProductionCallerThreadsWorkspace — AC8, the WIRING PROOF. A
// log-writing seam whose only caller is a test is dead code: the log path is
// derived from the run workspace, so the PRODUCTION caller
// (ship.Phase.runNative, go/internal/phases/ship/ship.go:147) must reach the
// gate carrying req.Workspace. The unit test must drive runNative itself — not
// runRepoContractGate directly — and assert the workspace arrived at the seam.
// This is the cycle-1064 anti-trap applied to the new parameter.
```
