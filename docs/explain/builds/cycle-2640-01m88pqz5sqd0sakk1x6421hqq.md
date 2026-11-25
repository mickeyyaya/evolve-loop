# Build Explanation — Cycle 2640

## Build Binding
- Cycle: 2640
- Base SHA: 00b778e3211028f64771e1ed45f7d4062aec0cb5

## Summary
The dev worktree sweep removes clean worktrees with a closed unmerged pull request and keeps each branch. It also lists changed paths on active dirty trees and honors protected policy globs.

## Rationale
The prior sweep treated a closed unmerged pull request as unlanded and hid changed paths on active dirty trees. It also lacked a policy glob list. Updating the existing plan gates and `DevOptions` avoids a second sweep path or code-embedded worktree names.

## Changed Areas
- `docs/architecture/packages/internal-gc.md` — documents the closed pull request check, active dirty path reports, open pull request keep reasons, and `gc.dev_worktrees.protected`.
- `go/cmd/evolve/cmd_gc_devwt.go` — extracts `ghPullRequestTipsByState` and adds `ghClosedPullRequestTips` alongside `ghMergedPullRequestTips`.
- `go/cmd/evolve/cmd_loop_outcome.go` — wires `ClosedPRs` to `ghClosedPullRequestTips` inside `devWorktreeGCOptions` for both `evolve gc` and the loop hook.
- `go/internal/gc/devlanded.go` — checks `DirtyPaths` first on active trees, accepts `landedClosedPR` in `landedEarly`, reports open pull request causes, and shares `prTips` across merged and closed lookups.
- `go/internal/gc/devworktrees.go` — adds `ClosedPRs` to `DevOptions`, matches `Policy.Protected` globs in `gateProtected`, appends causes in `gateIdle`, and checks `closedTips` in `landed`.
- `go/internal/gcpolicy/gcpolicy.go` — adds `Protected` (`protected`) to `DevWorktreesPolicy`.

## Design Decisions
- `protectedGlob` uses `path.Match` against paths relative to `<hub>/dev/` rather than a code allowlist, so operators add or remove protected trees in `.evolve/policy.json`.
- `ghClosedPullRequestTips` reuses `GhRemoteAPI.PullRequests` through `ghPullRequestTipsByState` instead of a second `gh` parser, keeping one JSON decode path.
- `gateIdle` keeps `active within the <N>-day limit` at the start of the reason and appends dirty paths or open pull request causes, preserving existing test contracts.

## Verification
Tests in `gcpolicy`, `policy`, `gc`, `cmd/evolve`, and `acs/cycle2640` verify clean removal, dirty path reports, and protected globs. They also verify process and open pull request holds, 98-tree dry-run reasons, end-to-end execution, and 100% changed-function coverage.

## Compatibility
Existing `gc.dev_worktrees` keys remain unchanged; `Protected` defaults to empty when absent, and existing active-tree keep reasons retain their prefix.

## Limitations
The `ghClosedPullRequestTips` lookup uses `GhRemoteAPI.PullRequests`, which queries up to 1000 recent pull requests.
