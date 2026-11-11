# Build Explanation — Cycle 2295

## Build Binding
- Cycle: 2295
- Base SHA: 3b9fe5a83a5397f6c1c392a8395b2855fb801498

## Summary
`evolve gc` now counts each flag across runs. It prints `count=N` on each flag line. When a flag reaches its second run, gc files one inbox item for it. Later runs file nothing more. This builds decision D11 and component X7 of the plan [gc closes the legacy gap](../../plans/gc-closes-the-legacy-gap-2026-11.md). This cycle continues cycle 2280, which built the same change and failed its handoff.

## Rationale
Fact F8 shows that 157 flags went to no owner, because gc only printed them. An inbox item gives a flag an owner. The tally lives in `.evolve/gc-flag-tally.json`, next to the other gc state. The filing reuses `inboxfile.Mint`, which holds the inbox lock and validates the item.

## Changed Areas
- `go/internal/gc/flagtally.go` — new. `CountFlags` adds one to the count of each flag key and drops a key that is gone. `FileRepeatedFlags` files one inbox item when a count equals `FlagRepeatThreshold`.
- `go/internal/gc/flagtally_test.go` — new. Tests cover the count, the one-item rule, a consumed item, a path-only flag and each error path.
- `go/cmd/evolve/cmd_gc.go` — the worktree step counts the flags, prints `count=N`, and files the items. A dry run does neither.
- `go/cmd/evolve/cmd_gc_flagtally_test.go` — new. It covers the report and the failure path of the filing.
- `docs/architecture/packages/internal-gc.md` — the package page records the tally design and its invariants.
- `.evolve/evals/gc-legacy-gap-x7-a-repeated-flag-gets-an-owner.md` — the eval graders of the task.
- `docs/private/research/archived-2026-11-11/superseded-predicate-packages/cycle2280/predicates_test.go` — the predicate package of cycle 2280, archived. Cycle 2295 has a fresh package.
- `docs/private/research/archived-2026-11-11/unshipped-build-explanations/cycle-2280-01m734wsk6n2tene18rpa3m40n.md` — the explanation of cycle 2280, archived because that cycle did not ship.

## Design Decisions
- The item is filed when the count equals the threshold, not when it reaches the threshold. A flag whose item the owner already consumed does not file a second item. The id of the item holds a hash of the flag key, so two flags never share an id.
- A flag key is the action, the branch and the path. A flag that is resolved leaves the tally, so a new flag of the same name starts at 1.
- A dry run neither reads nor writes the tally. This keeps `--dry-run` free of side effects.
- Rejected: a count in the inbox item only. A consumed item then resets the count.

## Verification
The cycle predicates `TestC2295_001` to `TestC2295_008` run the built `evolve gc` binary on a git hub fixture and check the coverage of each changed function. The failure of cycle 2280 was in `TestPRRefresh_TheRealGeneratorsOfACloneCommitOnlyTheStaleIndex`, an unrelated test. It passes when run alone under the `integration` tag.

## Compatibility
The flag line gains a ` count=N` suffix on a real run. Other gc output does not change. The tally file is new and gc creates it on the first real run.

## Limitations
If the filing fails on the run that reaches the threshold, gc reports the error and exits 1. It does not file the item on a later run.
