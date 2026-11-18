# Build Explanation — Cycle 2481

## Build Binding
- Cycle: 2481
- Base SHA: 67f531fcac35a41002f12ac780ac295066144790

## Summary
The inbox curator now proposes a lane reroute only after it checks the fix surface and the freshness of the item. Before this change it read the declared files only.

## Rationale
At the wave 178 boundary the curator proposed 16 reroutes, and lane triage bounced 7 of them. Four items declared a protected fix surface in `protected_files`. Three items were stale. The smallest fix is to add the two checks to the existing reroute decision, not to add a second decision path.

## Changed Areas
- `go/internal/inboxcurator/reroute.go` — a reroute is refused when the text of the item or the fix-surface hook names a protected path. A stale item or an item whose probes pass gets no reroute. A new function proposes a judgment quarantine for each stale item.
- `go/internal/inboxcurator/curate.go` — the Curate input carries the fix-surface hook and the stale hook, and passes the tree probe result to the reroute step.
- `go/internal/inboxbatch/item.go` — a new accessor returns the paths that the item text mentions. The path scan drops the sentence full stop, so that `ship.go.` matches `ship.go`.

- `go/internal/inboxcurator/freshness.go` — a new resolver reads the lane triage verdict in `routed_reason`. A verdict `already-shipped: <sha>` is stale when the commit is on main and touches the declared surface. A verdict `stale: <paths> absent at HEAD` is stale while the tree still lacks every named path.
- `go/cmd/evolve/cmd_inbox_curator.go` — the production curate command sets the stale hook with the resolver, so the freshness check acts without a test hook.

## Design Decisions
The hooks are injected functions on the existing option structs. The curator keeps no new flag. A stale claim is a judgment proposal and never deterministic, so the operator decides.

## Verification
The three ACS predicates of the cycle pass. A production-path test runs the 16 items of wave 178 through the curate command with no injected hook. The package tests of inboxcurator, inboxbatch and the inbox curate command pass.

## Compatibility
A call that sets none of the new hooks behaves as before. One exception: an item whose text names a protected file is no longer rerouted.

## Limitations
The production command sets the stale hook only. The fix-surface hook has no production caller. A verdict of `protected-surface` is the claim that the 9 kept items of wave 178 showed to be wrong. The four items with a protected fix surface stay off the lane through the `protected_files` fence of the builder profile. The existing `inboxbatch.ConsoleRouted` applies it, and `reroute.go` calls it. The text check of the item is an added guard for an item that names a protected file only in its prose.
