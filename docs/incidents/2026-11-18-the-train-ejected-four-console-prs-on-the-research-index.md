# 2026-11-18: the train ejected four console PRs on the research index

Related documents: component C2 of [the concurrent-edit plan](../plans/concurrent-edit-layout-2026-11.md), and its amendment of 2026-11-18. The research is [Concurrent edits: where lane changes collide in the code](../research/concurrent-edit-layout-2026-11.md), finding F2.10.

## Summary

- **Impact:** two integration batches ejected four console PRs on a conflict in `docs/research-index.md`. Each ejected PR was rebuilt, reviewed again and recorded again. In the second batch, the rebuilds took 14 minutes and closed 2 PRs.
- **Root cause:** `docs/research-index.md` is a generated aggregate. Each console PR that adds a plan also adds a shard and the regenerated aggregate. The train applies each commit with `git merge-tree`, and it ejects a PR on each conflict (`go/internal/integration/compose.go:220`). It has no step that runs the generator. The fleet rebase of a lane has that step. In cycles 2300 to 2462, no lane landing was ejected on a derived output (research F2.10).
- **Fix in the run:** one docs PR (#1555) now holds the shards and the aggregate. The code PR #1552 was rebuilt as #1556 with no index change.
- **Fix of the cause:** C2 of the concurrent-edit plan starts again. The train regenerates a derived output when only derived outputs conflict.
- **Lesson:** the plan deferred C2 because only console PRs meet this conflict. Console PRs hold the plans, so they meet it at each boundary.

## Terms used throughout

- **Train:** `evolve integration batch`, which composes the recorded PRs onto main and lands them.
- **Aggregate:** a file that a generator writes from its sources. The research index is generated from the shard files in `docs/research-index/`.
- **Derived catalog:** the list of aggregates, their sources and their generator commands (`go/internal/derived/derived.go:57`).
- **Eject:** the train removes a PR from the batch, and the PR waits for a rebuild.

## Timeline

| Time (UTC) | Event | Evidence |
|---|---|---|
| 2026-11-17 18:40 | Batch 178 composes 9 PRs. It ejects #1546 and #1548 on `docs/research-index.md` and lands 7. | the batch output in the console report: `eject PR #1546 conflict docs/research-index.md` and `eject PR #1548 conflict docs/research-index.md` |
| 2026-11-17 18:53 | The console rebuilds #1546 as #1550. | `gh pr view 1550 --json createdAt` |
| 2026-11-17 19:06 | The console puts the shard of #1548 into #1552. | `gh pr view 1552 --json createdAt` |
| 2026-11-17 22:27 to 23:03 | A lane lands a 2026-11-18 section of the research index on main. | `git log origin/main -- docs/research-index.md` |
| 2026-11-17 23:43 | A dry run of the next batch ejects #1550 and #1552 on `docs/research-index.md`. | the dry-run output in the console report: `eject PR #1550 conflict docs/research-index.md` and `eject PR #1552 conflict docs/research-index.md` |
| 2026-11-17 23:57 | After the rebuilds (#1555 and #1556), the dry run composes all 5 PRs. | the file time of the third dry-run output |

## Root cause

**F1.1** The train applies each commit of a PR on its own. It runs `git merge-tree --write-tree --merge-base=<commit>^` onto the batch tip (`go/internal/integration/compose.go:274`). On a conflict, it ejects the PR (`go/internal/integration/compose.go:220`).
*Implication:* a later commit of the PR that removes the index change does not help. The first commit still conflicts.

**F1.2** Each new section of the aggregate goes at the end of the generated region, sorted by the shard name. Two PRs that each add a section change the same lines. A lane section on main changes them too.
*Implication:* the conflict comes back at each boundary that lands a plan from the console.

**F1.3** The fleet rebase of a lane already regenerates a derived output when only derived outputs conflict (`derivedConflictIn`, `go/internal/core/ship_recovery.go:221`). The train has no such step.
*Implication:* the plan amendment of 2026-11-17 deferred C2 for this reason: "only console PRs meet a derived conflict in the train". This incident shows the cost of that deferral.

## What changed

| Change | Pull request | Regression test |
|---|---|---|
| One docs PR holds the shards and the aggregate of two code PRs. | #1555 | none (a manual step) |
| #1552 is rebuilt with no index change. | #1556, which supersedes #1552 | none (a manual step) |
| The train regenerates a derived output when only derived outputs conflict. | open: C2 of the concurrent-edit plan | open: two branches that each add a shard compose with no conflict |

## What we learned

- A deferral that says "only the console meets it" still costs the console time at each boundary. Count the boundaries, not the lanes. (F1.2, F1.3)
- One fix in two places is a duplicate. The train needs the same regeneration step as the fleet rebase, from the same derived catalog. (F1.3)
