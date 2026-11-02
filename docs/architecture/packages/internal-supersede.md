# internal/supersede

## Purpose

`internal/supersede` finds that the console took the item of a live cycle ([ADR-0158](../adr/0158-a-repair-round-runs-only-for-a-finding-the-lane-can-fix-and-the-auditor-applies-a-small-prescribed-fix.md) decision 13). Source incident: cycle 2035 spent about 41 LLM minutes after the console took its item.

## Design

- **`Check(read, id, cycle)` is pure.** It calls the `Reader` one time per ask and holds no state between asks. It never polls.
- **Superseded means one of two facts.** Another cycle holds the claim, or the route of the item is a console route. The `Result` carries the `Reading` that proves it.
- **A failed read is an error.** `Check` wraps the error and returns the zero `Result`, so a broken inbox cannot stop a cycle.
- **`InboxReader` is the production reader.** It locates the item with `inboxmover.Locate` and reads the `route` field of the item file.

## Invariants

- The stage is `workflow.supersede.stage` ([policy-config](../policy-config.md)). The package does not read it. The caller does.
- The call site in `internal/core` is not wired yet. The console item `wire-cost-aware-repair-call-sites` owns it.
- Pinned by `go/acs/cycle2105/predicates_test.go` and `apicover_named_test.go`.
