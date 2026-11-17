# Build Explanation — Cycle 2463

## Build Binding
- Cycle: 2463
- Base SHA: 90ba72ca5cbba2c322823458fdd1da99b9f439ab

## Summary
`evolve loop` takes two new flags, `--only` and `--exclude`. Each flag lists inbox ids. A launch with `--exclude` never gives an excluded id to a lane. A launch with `--only` gives a lane only the listed ids. No inbox file changes.

## Rationale
Goal text only warned about inbox ids, so an operator had no way to keep a lane off an item without a file edit. The plan seam in `internal/loopwave` already builds every lane assignment: the seed plan, the prior-decision widen and the backlog refill. One filter at that seam covers all three. Launch validation stays in `cmd/evolve`, next to the goal-item check that it extends.

## Changed Areas
- `go/internal/loopwave/selector.go` — the `Selector` type, the `WithSelector` option and the filtered backlog read.
- `go/internal/loopwave/plan.go` — the seed plan and the prior-decision prune use the selector. The seed needs one lane, not two, when a selector is set.
- `go/internal/loopwave/decision.go` — the widen reads the filtered backlog.
- `go/internal/loopwave/launcher.go` — the refill reads the filtered backlog.
- `go/internal/loopwave/loopwave.go` — the engine holds the selector.
- `go/internal/triagecap/lane_menu.go` — `SeedMenusFromBacklog` seeds from a given backlog. `SelectWaveSeedMenus` calls it.
- `go/cmd/evolve/cmd_loop_args.go` — the two flags and the id-list parser.
- `go/cmd/evolve/cmd_loop.go` — the config fields and the launch check at both entry points.
- `go/cmd/evolve/cmd_loop_goal_ids.go` — the launch check: an id in both lists, and an `--only` id no lane can take, exit 10. The id classifier is shared with the goal check.
- `go/cmd/evolve/cmd_loop_wave.go` — the wave engine receives the selector from the config.
- `go/cmd/evolve/cmd_loop_driver.go` — a sequential batch warns that it ignores the selector.
- `go/cmd/evolve/cmd_boundary.go` — the boundary loop step carries both flags.
- `go/cmd/evolve/cmd_wave_compose.go` — `wave next` parses both flags into the boundary arguments.
- `go/cmd/evolve/cmd_wave.go` — the `wave next` usage text.
- `docs/operations/runtime-reference.md` — one table row for the launch selector.

## Design Decisions
The detached child and the chain re-exec copy the original arguments, so both flags ride on them with no new code. The boundary step builds its arguments, so it gets explicit lines. A relaunch, a resume and a chain handoff skip the `--only` takeability check, because the earlier waves can have shipped a listed item. The id-conflict check always runs.

## Verification
The cycle tests `TestSelectorFlags_*` and `TestPlanSelector_*` cover each acceptance criterion. `TestSeedMenusFromBacklog_SeedsOnlyFromTheGivenBacklog` covers the new triagecap export. `TestWarnSelectorIgnored_NamesTheSequentialPathOnlyWhenASelectorIsSet` covers the warning.

## Compatibility
With no flag set, the selector is empty and the plan is the same as before. `SeedWavePlanFromInbox` and `WidenNarrowDecision` keep their signatures.

## Limitations
A sequential batch does not use the selector, because its cycle triage picks the item. The batch prints a warning. A prior decision with committed floors is not pruned.
