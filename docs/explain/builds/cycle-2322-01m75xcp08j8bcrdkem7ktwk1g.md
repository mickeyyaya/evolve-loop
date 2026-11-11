# Build Explanation — Cycle 2322

## Build Binding
- Cycle: 2322
- Base SHA: 6e35fc4f7bcf4594d8cf8763782d76ab5f25917f

## Summary
The dev-worktree sweep of `evolve gc` now reads a branch as landed when a merged pull request has the branch tip as its head. A later commit on the branch is not landed.

## Rationale
A squash merge leaves the branch commits outside main, so git evidence alone cannot prove a landing. The merged pull request records the head oid at merge time. A branch whose tip equals that oid holds no unmerged work.

## Changed Areas
- `go/internal/gc/devworktrees.go` — adds the `MergedPRLookup` type, the `MergedPRs` option, the cache fields, and the merged-tip check in `landed`.
- `go/internal/gc/devlanded.go` — adds `mergedTips`, which reads the source once per run and notes a fallback to git evidence on failure.
- `go/cmd/evolve/cmd_gc_devwt.go` — adds `ghMergedPullRequestTips`, which keeps only merged pull requests with a non-empty head oid.
- `go/cmd/evolve/cmd_loop_outcome.go` — wires `MergedPRs` in `devWorktreeGCOptions`, the production entry of the sweep.

## Design Decisions
This change builds component X2 of `docs/plans/gc-closes-the-legacy-gap-2026-11.md`. It applies decision D4 of that plan. A branch is landed when its pull request is merged and the branch tip equals the `headRefOid`. A closed pull request that is not merged is not landed. The lookup reuses `GhRemoteAPI.PullRequests`, so no second `gh` call shape exists. The source is optional: a nil source adds no note and leaves landed to git evidence.

## Verification
Unit tests in `go/internal/gc` cover the tip, the later commit, another branch and an empty oid. They also cover one read per run and the failure fallback. Wiring tests in `go/cmd/evolve` cover the production entry. The predicates of cycle 2322 pass.

## Compatibility
Existing options and reports keep their form. A failed or absent source gives the earlier behavior plus one note.

## Limitations
A branch deleted and reused with the same tip as an old merged pull request counts as landed.
