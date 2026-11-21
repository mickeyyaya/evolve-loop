# Build Explanation — Cycle 2586

## Build Binding
- Cycle: 2586
- Base SHA: 23c32f42926c3dcde2c2da08401c897afd66275a

## Summary
The publisher now accepts the five roles that signal roots already use: integration, auth, queue, latency and retire.

## Rationale
The call sites pass these roles to the publisher. The publisher refused them, so their events never reached the bus. Adding them to the closed set is the smallest change that fixes this.

## Changed Areas
- `go/internal/events/publisher/publisher.go` — adds the five roles to the closed role set.
- `docs/architecture/event-channels.md` — lists the five added roles in §1 so the spec matches `publisher.Roles()`.

## Design Decisions
The role list stays one closed set in one place. A role outside the set is still refused.

## Verification
The cycle 2586 predicates and the publisher tests check the set, the call sites and the bus delivery.

## Compatibility
Existing roles keep their names and order. The new roles come after them.

## Limitations
This change does not add a role for any future root.
