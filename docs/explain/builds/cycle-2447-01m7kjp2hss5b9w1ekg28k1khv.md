# Build Explanation — Cycle 2447

## Build Binding
- Cycle: 2447
- Base SHA: cb76052bc1d198415fb96b29cf3a9f8af495007c

## Summary
The loop and each new `evolve agent run` seat now stop or refuse when the highest live usage reading reaches `cli_health.usage_hold_percent`.

## Rationale
The usage probe already records the windows. One shared check, `usageprobe.CheckHold`, gives the loop and the seat the same rule, so no console must be awake. An absent key keeps today's behavior.

## Changed Areas
- `go/internal/usageprobe/hold.go` — new `CheckHold` and `Hold` with the fail-closed rule for an unreadable reading.
- `go/internal/usageprobe/hold_test.go` — tests that name the new exports.
- `go/internal/policy/dispatch.go` — new `UsageHoldPercent` field of the `cli_health` block.
- `go/cmd/evolve/cmd_loop_window.go` — the loop boundary stops with `loop_usage_hold` and emits one `loop.halt` signal.
- `go/internal/cli/agentcmd/run.go` — a seat past the limit exits 3 before it launches.
- `docs/architecture/packages/internal-usageprobe.md` — new Usage hold section.

## Design Decisions
One helper in `usageprobe` avoids two copies of the reading rule. The check runs after the pre-wave probes, so the reading is fresh.

## Verification
The tests of `usageprobe`, `policy`, `agentcmd` and `cmd/evolve` pass, and the four cycle 2447 predicates pass.

## Compatibility
No policy key means no change. No observation at all does not hold.

## Limitations
The brake reads the family week windows only. It does not read five-hour windows.
