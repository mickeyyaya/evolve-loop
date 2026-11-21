# Build Explanation — Cycle 2596

## Build Binding
- Cycle: 2596
- Base SHA: a1d94de45f161e7196b00f806015f048d8447aba

## Summary
The event publisher now accepts the five signal-root roles that CLI call sites pass: `integration`, `auth`, `queue`, `latency`, and `retire`. The event channels specification lists all 20 roles in `publisher.Roles()` order.

## Rationale
CLI signal roots passed those five roles, and `Config.check` refused them, so `rootwire.Attach` dropped their events. Extending the closed set in `publisher.Roles()` and its spec bullet fixes the loss without weakening role validation.

## Changed Areas
- `go/internal/events/publisher/publisher.go` — adds `integration`, `auth`, `queue`, `latency`, and `retire` to `Roles()` so `Config.check` admits all 20 signal-root roles.
- `docs/architecture/event-channels.md` — updates the §1 `Roles` bullet to list all 20 roles in `publisher.Roles()` order.

## Design Decisions
`publisher.Roles()` remains the single closed role list, and unknown roles still fail validation. The call-site guard derives roles from `cmd/evolve` AST call sites rather than a static subset, so a future unregistered role fails tests before ship.

## Verification
The `internal/events/...` tests, `TestRootSignalCenter_*` in `cmd/evolve`, `evolve signals codes check`, `evolve docs ste-lint --strict`, and all six `cycle2596` ACS predicates pass on this merged tree.

## Compatibility
All 15 prior role names keep their names and positions, and the five added roles follow them in `publisher.Roles()`.

## Limitations
Any new `cmd/evolve` root that passes a non-constant, non-parameter role expression must update the explicit dynamic-site allowlist in `TestRootSignalCenter_EveryCallSiteRoleIsAPublisherRole`.
