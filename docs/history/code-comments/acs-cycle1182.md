# Comment history: `acs/cycle1182`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1182/predicates_test.go:3` — above `package cycle1182`

```text
// Package cycle1182 materialises the cycle-1182 acceptance criteria for the sole
// triage-committed top_n task of this fleet lane: `wave-planner-pass-scope-prune`.
//
// The defect. `triagecap.pruneConsumed` drops committed ids the inbox lifecycle
// has already consumed, but it is wired into the FRESH-SEED path only
// (SelectWaveSeedMenus). The PRIMARY per-wave path — `widenNarrowDecision` in
// package main (go/cmd/evolve/cmd_loop_wave.go), which runs whenever a prior
// cycle's triage-decision.json exists — builds its committed prefix straight from
// decision.top_n and either short-circuits on `len(committed) >= count` or hands
// the list to WidenTopNToFleetWidth, which copies it through VERBATIM. Neither
// branch consults the lifecycle, so a consumed id survives in the prior decision
// file and is re-pinned into the next wave's lane-scope.json (cycle-1116
// re-pinned tdd-topn-binding-gate after cycle-1113 consumed it).
//
// Predicate strategy: behavioural-via-subprocess (the cycle-1098 precedent). The
// subject `widenNarrowDecision` lives in `package main`, which cannot be
// imported, so each predicate shells `go test -run` over the RED contract tests
// authored this cycle (go/cmd/evolve/cmd_loop_wave_prune_test.go and
// go/internal/triagecap/lane_menu_prune_export_test.go). Every one of those
// CALLS the system under test — widenNarrowDecision over a real temp-dir inbox
// lifecycle, and the planner end-to-end via fleet.PlanFromTriage — and asserts on
// the returned decision bytes / lane scopes. None is a source-grep of production
// code (the cycle-85 degenerate-predicate ban).
//
// RED at authoring time: 001 fails (consumed ids survive every branch), 002 fails
// to COMPILE (`undefined: PruneConsumed`), 003 is the pre-existing-green
// no-regression guard.
```
