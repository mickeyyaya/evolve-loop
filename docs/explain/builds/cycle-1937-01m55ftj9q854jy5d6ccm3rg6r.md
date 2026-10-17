# Build Explanation — Cycle 1937

## Build Binding
- Cycle: 1937
- Base SHA: 2ab5823a7237968e8e5c193e04043f4f3c253441

## Summary
`evolve gc` gets a cache scope, `--only cache`, that runs only the go build cache trim and the pipeline temp sweep. A new policy key, `gc.cache_trim_free_gb`, makes that scope trim only while disk free is low. The dev worktree archive archives a landed, clean tree with no open pull request before the 14-day idle limit.

## Rationale
On 2026-10-17 the go build cache grew from 21.95 GB to 66 GB in one hour, and no gc scope was safe while lanes ran (inbox item `gc-cache-scope-and-merged-dev-worktrees`). The same day the archive kept 95 merged, clean dev worktrees for the full 14 days. The cache scope reuses the existing `goCache` and `pipelineTemp` steps, so it adds no sweep logic. The landed rule reuses the existing `landed` and `openPRs` checks.

A fast-forward merge leaves the branch head equal to `origin/main`, the same graph as a tree created at main a moment ago. Only the HEAD reflog tells them apart, so the rule requires a reflog entry other than the creation point. Rejected: a commit count against `origin/main` (zero after a fast-forward). Rejected: commit time against creation time (same second in a fresh fixture).

The predicate package of cycle 1935 asserted the branch `main` on an unborn fixture repository, so it failed before the scope ran. Cycle 1937 supersedes it with a fixture that commits first. The old package and its unshipped explanation move to the research archive.

## Changed Areas
- `go/cmd/evolve/cmd_gc.go` — adds the `--only` flag, refuses an unknown scope with exit 10 before any sweep, and extracts `loadPolicy`.
- `go/cmd/evolve/cmd_gc_scope.go` — the cache scope: the disk-free floor check, then `goCache` and `pipelineTemp` only.
- `go/cmd/evolve/cmd_gc_scope_test.go` — covers the trim under the floor, the skip at the floor, an unreadable disk free and the unknown-scope refusal.
- `go/cmd/evolve/parallel_serial_tests_test.go` — registers the scope tests that swap the `gcDiskFreeBytes` seam as serial.
- `go/cmd/evolve/cmd_gc_scope_amplify_test.go` — amplified tests for malformed scopes, a dry run, an unset floor and floor overflow; the two pure tests run in parallel.
- `go/acs/cycle1937/parallel_default_test.go` — the predicate that keeps the amplified tests under the parallel-default gate.
- `go/internal/gcpolicy/gcpolicy.go` — adds the typed key `cache_trim_free_gb` and `CacheTrimFreeBytes`.
- `go/internal/gcpolicy/gcpolicy_test.go` — covers the unset, negative and explicit values of the key.
- `go/internal/gc/devlanded.go` — the landed rule: own work in the reflog, a clean tree, `is in main`, and a lookup that found no open pull request.
- `go/internal/gc/devlanded_test.go` — covers each condition that keeps a landed tree.
- `go/internal/gc/devworktrees.go` — the idle gate asks the landed rule inside the idle limit, and the archive reason says the tree landed early.
- `go/acs/cycle1937/predicates_test.go` — the eight cycle predicates, with a committed fixture repository.
- `.evolve/evals/gc-cache-scope-and-merged-dev-worktrees.md` — the eval graders of the task.
- `docs/operations/runtime-reference.md` — documents the scope and the new policy key.
- `docs/architecture/packages/cmd-evolve.md` — documents `cacheScope` and the refusal.
- `docs/architecture/packages/internal-gc.md` — documents the landed rule of the dev worktree archive.
- `docs/private/research/archived-2026-10-17/superseded-predicate-packages/cycle1935/predicates_test.go` — the archived cycle 1935 predicates.
- `docs/private/research/archived-2026-10-17/unshipped-build-explanations/cycle-1935-01m555wgjs1jgzqexh3j2t6j51.md` — the archived cycle 1935 explanation.
- `docs/explain/builds/cycle-1937-01m55ftj9q854jy5d6ccm3rg6r.md` — this document.

## Design Decisions
The disk-free floor lives inside the cache scope, so the console runs `evolve gc --only cache` at any time and the command decides from the key. An unset key keeps the scope unconditional. A failed or skipped pull request lookup keeps the tree, so only the idle limit can archive it.

## Verification
`go test -tags acs ./acs/cycle1937` passes all eight predicates. The unit tests of `internal/gc`, `internal/gcpolicy`, `internal/policy`, `internal/gittest` and the gc tests of `cmd/evolve` pass. `evolve acs suite --cycle 1937` reports green=181 red=0. The cmd/evolve parallel-default gate passes under `-tags integration`.

## Compatibility
A plain `evolve gc` runs the same steps as before. A policy file without `cache_trim_free_gb` decodes as before.

## Limitations
No loop or wave hook runs the cache scope on its own; the console or an operator runs it. A worktree with an expired HEAD reflog waits out the idle limit.
