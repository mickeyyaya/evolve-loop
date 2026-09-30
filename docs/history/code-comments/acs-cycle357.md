# Comment history: `acs/cycle357`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle357/predicates_test.go:3` — above `package cycle357`

```text
// Package cycle357 materializes the cycle-357 acceptance criteria for the
// committed top_n task:
//
//   - dispatch-bridge-retirement — retire EVOLVE_DISPATCH_STOP_ON_FAIL and
//     EVOLVE_DISPATCH_VERIFY deprecated bridge flags from flagregistry,
//     bridge function, bridge test cases, and skills docs.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	dispatch-bridge-retirement:
//	  AC-1 (neg)  EVOLVE_DISPATCH_STOP_ON_FAIL absent from flagregistry.Lookup → C357_001
//	  AC-2 (neg)  EVOLVE_DISPATCH_VERIFY absent from flagregistry.Lookup        → C357_001
//	  AC-3 (neg)  bridge code absent from resolveDispatchPolicy + whole file    → C357_002 + C357_007
//	  AC-4        TestResolveDispatchPolicy: no legacy cases in test output      → C357_003
//	  AC-5 (neg)  bridge test cases absent from cmd_loop_m4_test.go             → C357_004
//	  AC-6 (neg)  EVOLVE_DISPATCH_VERIFY=0 absent from claude-runtime.md        → C357_005
//	  AC-7        evolve flags check exits 0 (no drift)                          → C357_006
//	  AC-8 (neg)  no non-test Go files reference either deprecated flag          → C357_007
//	  AC-9        EVOLVE_DISPATCH_POLICY preserved in resolveDispatchPolicy      → C357_002
//	  AC-10       go test ./internal/flagregistry/... all PASS                   → C357_008
//
// Floor binding (R9.3): predicates only for committed top_n task.
// Deferred tasks (EVOLVE_CHANNEL, EVOLVE_BYPASS_SHIP_VERIFY, etc.) get zero predicates.
```

### `go/acs/cycle357/predicates_test.go:141` — above `func TestC357_006_FlagsCheckExitsZero(t *testing.T) {`

```text
// TestC357_006_FlagsCheckExitsZero verifies that `evolve flags check` exits 0,
// confirming that docs/architecture/control-flags.md is in sync with the
// flagregistry after Builder removes the 2 deprecated rows and re-runs
// `evolve flags generate`.
//
// BEHAVIORAL: runs the real evolve binary against the worktree.
//
// RED confirmed: control-flags.md in the worktree is already stale vs
// flagregistry (cycle-356 drift). Builder must run `evolve flags generate`
// after removing the 2 deprecated registry rows to reach GREEN.
```
