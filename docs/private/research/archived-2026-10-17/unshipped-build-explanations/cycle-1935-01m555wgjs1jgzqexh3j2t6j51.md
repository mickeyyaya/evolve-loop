# Build Explanation — Cycle 1935

## Build Binding
- Cycle: 1935
- Base SHA: 7cce1ae6cf82d4331ae6ef08803fbcf1693dac22

## Summary
`evolve gc` gets a cache scope, `--only cache`, that runs only the go build cache trim and the pipeline temp sweep. A new policy key, `gc.cache_trim_free_gb`, makes that scope trim only while disk free is low. The dev worktree archive now archives a landed, clean tree with no open pull request before the 14-day idle limit.

## Rationale
On 2026-10-17 the go build cache grew from 21.95 GB to 66 GB in one hour. No gc scope was safe while lanes ran (inbox item `gc-cache-scope-and-merged-dev-worktrees`). The same day the archive kept 95 merged, clean dev worktrees for the full 14 days. The cache scope reuses the existing `goCache` and `pipelineTemp` steps, so it adds no new sweep logic. The landed rule reuses the existing `landed` and `openPRs` checks, as the inbox fix asks.

A merge that fast-forwards main leaves the branch head equal to `origin/main`, the same graph as a tree created at main a moment ago. Only the HEAD reflog of the worktree tells them apart, so the rule requires a reflog entry other than the creation point. Rejected: a commit count against `origin/main`. It is zero for a fast-forward merge. Rejected: commit time against worktree creation time. Both fall in the same second in a fresh fixture.

## Changed Areas
- `go/cmd/evolve/cmd_gc.go` — adds the `--only` flag and refuses an unknown scope with exit 10 before any sweep. It extracts `loadPolicy`, so both runs load the gc block one way.
- `go/cmd/evolve/cmd_gc_scope.go` — the cache scope: the disk-free floor check, then `goCache` and `pipelineTemp` only.
- `go/cmd/evolve/cmd_gc_scope_test.go` — covers the trim under the floor, the skip at the floor, an unreadable disk free, and the unknown-scope refusal.
- `go/internal/gcpolicy/gcpolicy.go` — adds the typed key `cache_trim_free_gb` and `CacheTrimFreeBytes`, so a non-numeric value fails the strict decode.
- `go/internal/gcpolicy/gcpolicy_test.go` — covers the unset, negative and explicit values of the key.
- `go/internal/gc/devlanded.go` — the landed rule: own work in the reflog, a clean tree, `is in main`, and a lookup that ran and found no open pull request.
- `go/internal/gc/devlanded_test.go` — covers each rule that keeps a landed tree, and the reflog own-work check.
- `go/internal/gc/devworktrees.go` — `gateIdle` asks the landed rule inside the idle limit, `openPRs` records a skipped lookup, and the archive reason says the tree landed early.
- `docs/operations/runtime-reference.md` — documents the scope and the new policy key.
- `docs/architecture/packages/cmd-evolve.md` — documents `cacheScope` and the refusal.
- `docs/architecture/packages/internal-gc.md` — documents the landed rule of the dev worktree archive.
- `docs/explain/builds/cycle-1935-01m555wgjs1jgzqexh3j2t6j51.md` — this document.

## Design Decisions
The disk-free floor lives inside the cache scope, not in a new console trigger. The console can then run `evolve gc --only cache` at any time, and the command decides from the key. An unset key keeps the scope unconditional. A landed check that fails keeps the tree and names the error in its reason, because the idle limit alone kept it before this change. A skipped or unavailable pull request lookup never lets a tree skip the idle limit.

## Verification
The cycle predicates `TestC1935_001` to `TestC1935_003`, `TestC1935_005` and `TestC1935_006` pass. `TestC1935_004` passes every check of the cache scope. Its last check reads the branch `main` of an unborn fixture repository. That check fails before the scope runs (see the build report). The unit tests of `internal/gc`, `internal/gcpolicy`, `internal/policy` and the gc tests of `cmd/evolve` pass.

## Compatibility
A plain `evolve gc` runs the same steps as before. A policy file without `cache_trim_free_gb` decodes as before. A tree outside the idle limit follows the old rules.

## Limitations
No loop or wave hook runs the cache scope on its own yet; the console or an operator runs it. A worktree whose HEAD reflog was expired cannot show its own work, so it waits out the idle limit.
