# Comment history: `acs/cycle1314`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1314/predicates_test.go:3` — above `package cycle1314`

```text
// Package cycle1314 materialises the cycle-1314 acceptance criteria for the
// single fleet-scoped task pinned to this lane: boundary-binary-refresh
// (inbox item auto-refresh-binary-at-boundary, P1, weight 0.94).
//
// The defect: runLoopChain (go/cmd/evolve/cmd_loop_chain.go) relaunches
// runLoopBatchFn at every boundary but never checks whether the running
// binary has fallen behind HEAD — fixes that land on main mid-chain sit inert
// until an operator manually rebuilds + re-pins + relaunches (cost: cycles
// 1302-1309 kept running an already-fixed defect).
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549…1098
// precedent). The subject lives in `package main` (go/cmd/evolve), which
// cannot be imported, so each predicate shells `go test -run` over the RED
// contract tests authored this cycle in
// go/cmd/evolve/cmd_loop_chain_boundaryrefresh_test.go. Every one of those
// drives real system-under-test behaviour — a real git fixture for the
// ahead-of-HEAD detection, and an end-to-end runLoopChain drive over spied
// rebuild/repin/re-exec seams — asserting on returned values, the on-disk
// state.json pin, the boundary-refresh audit log, and the emitted chain
// summary. None is a source-grep of production code (the cycle-85
// degenerate-predicate ban). RED now: chainBoundaryAheadFn / chainRebuildFn /
// chainReExecFn / maybeRefreshChainBoundary are all undefined, so
// go/cmd/evolve does not compile.
```

### `go/acs/cycle1314/predicates_test.go:131` — above `func TestC1314_007_ExistingChainSemanticsUnchanged(t *testing.T) {`

```text
// TestC1314_007_ExistingChainSemanticsUnchanged — AC5: anti-regression. The
// cycle-1075/1098 chain contract (drain->next batch, quota defer, exact cap,
// brake, fleet-width preservation, rc mapping, min-one-batch, inbox
// pending-validity) must still hold after wiring in the boundary refresh.
// This is the predicate that fails if Builder "wires in" the refresh by
// weakening the pre-existing decision precedence.
```
