# Build Explanation — Cycle 2280

## Build Binding
- Cycle: 2280
- Base SHA: 67a8d49f6e51f8d7b1d26e44d71d883df4e848da

## Summary
`evolve gc` now counts each flag across runs. It prints `count=N` on each flag line. When a flag reaches its second run, gc files one inbox item for it. Later runs file nothing more. This builds decision D11 and component X7 of the plan [gc closes the legacy gap](../../plans/gc-closes-the-legacy-gap-2026-11.md).

## Rationale
Fact F8 shows that 157 flags went to no owner, because gc only printed them. An inbox item gives a flag an owner. The tally lives in `.evolve/gc-flag-tally.json`, next to the other gc state. The filing reuses `inboxfile.Mint`, which holds the inbox lock and validates the item.

## Changed Areas
- `go/internal/gc/flagtally.go` — new. `CountFlags` adds one to the count of each flag key and drops a key that is gone. `FileRepeatedFlags` files one inbox item when a count equals `FlagRepeatThreshold`.
- `go/internal/gc/flagtally_test.go` — new. Tests cover the count, the one-item rule, a consumed item, a path-only flag and each error path. Each changed function has 100% coverage.
- `go/cmd/evolve/cmd_gc.go` — the worktree step counts the flags, prints `count=N`, and files the items. A dry run does neither.

## Design Decisions
- The item is filed when the count equals the threshold, not when it reaches the threshold. A flag whose item the owner already consumed does not file a second item. The id of the item holds a hash of the flag key, so two flags never share an id.
- A flag key is the action, the branch and the path. A flag that is resolved leaves the tally, so a new flag of the same name starts at 1.
- A dry run neither reads nor writes the tally. This keeps `--dry-run` free of side effects.
- Rejected: a count in the inbox item only. A consumed item then resets the count.

## Verification
The cycle predicates `TestC2280_001` to `TestC2280_007` run the built `evolve gc` binary on a git hub fixture. The `go/internal/gc` suite passes with the `integration` tag.

## Compatibility
The flag line gains a ` count=N` suffix on a real run. Other gc output does not change. The tally file is new and gc creates it on the first real run.

## Limitations
If the filing fails on the run that reaches the threshold, gc reports the error and exits 1. It does not file the item on a later run.
