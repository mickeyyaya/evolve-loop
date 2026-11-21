# Build Explanation — Cycle 2582

## Build Binding
- Cycle: 2582
- Base SHA: 23c32f42926c3dcde2c2da08401c897afd66275a

## Summary
The pool now launches a parked lead again after its first lane exits. A todo marked Resume bypasses the known-id dedup when no lane of that id runs or waits.

## Rationale
The pool recorded each id in a known set at first launch. A resupply that named the released parked scope was dropped. A Resume mark on the todo is the smallest typed signal that separates a deliberate relaunch from a duplicate.

## Changed Areas
- `go/internal/fleet/partition.go` — adds the Resume field to Todo.
- `go/internal/fleet/pool.go` — resupply accepts a known todo when it is marked Resume and no lane of that id runs or is pending.
- `go/cmd/evolve/cmd_loop_pool_paused.go` — marks the lead of a main-red parked scope as Resume when the stop is released.

## Design Decisions
Only a released main-red parked lead is marked. Other paused leads keep the dedup, so existing ordering behavior stays unchanged.

## Verification
The fleet tests, the planner test in cmd/evolve and the four cycle 2582 ACS predicates pass.

## Compatibility
The Resume field is omitted from JSON when false. Unmarked todos behave as before.

## Limitations
A marked todo that is already running or pending is still deduped, by design.
