# 2026-09-14 — a lane ship redded main: the ship gate tested the wrong tree, seeded from an empty index, and never looked at importers

**Status:** fixed (this change) · **Severity:** P1 pipeline (main red from 4db205a8 until the console hotfix #590; every lane dispatched in between rebased onto a red base and the wave's ships were blocked behind CI) · **Found by:** the verification wave's CI reds (cycles 1657/1659), inbox item `lane-ship-gate-lands-red-main-package-scoped-tests`, and the architecture review of the first fix.

## What happened

A lane renamed a stop in `internal/core`. Its own package tests were green, its build floor was green, and the ship-time repo-contract gate (`internal/phases/ship/repocontract.go`) ran green. `internal/deliverable`'s e2e test, untouched by the diff, asserted the old contract; main's CI went red on the next push and stayed red until #590 landed by hand.

The first cut of this fix added an importer layer. Its architecture review found the gate had **three** defects, two of them older than the incident:

| # | Defect | Effect on every lane ship |
|---|---|---|
| 1 | **The gate ran in the project root.** `runNative` passed `req.ProjectRoot` — the plane's main checkout — while a cycle ship's changes live in the lane worktree (`req.Worktree`) until `Run` lands them | every layer tested main's pre-landing tree: a tree without the lane's changes. The gate could not see what it was guarding |
| 2 | **The seeds read the index.** The added-test backstop selected `git diff --cached --diff-filter=A`; the first importer cut selected `git diff --cached` | a lane's build output is unstaged and its new files untracked until the ship itself stages them (inside `Run`, after the gate). At gate time the index is empty: both backstops saw nothing on the cycle path. Every test staged by hand, so the suite could not tell |
| 3 | **No importer layer.** The fixed pack (four repo-wide guard suites) and the added-test backstop never looked at the packages that import what the lane changed | the incident shape: a modified package, green in place, red in an untouched importer's test |

## Fix

| Layer | Change | Test (red first) |
|---|---|---|
| `ship` `runNative` | **one landing-tree decision:** `landingTree(opts)` in `gitops.go` — the cycle's active worktree when the class is cycle, it is set, differs from the project root and exists; the project root otherwise; a typed worktree that cannot be resolved is `WORKTREE_RESOLVE`. `atomicShip` lands from it and `repoContractGateRoot` tests it (against `WorktreeBaseSHA` when landing from a worktree, else HEAD), so the tree the gate proves is the tree the ship pushes and an unresolvable worktree fails the gate closed instead of testing main in the lane's stead | `TestRunNative_GateTestsTheLaneWorktreeNotTheProjectRoot` (the root is untouched, the worktree holds the red importer: the gate goes RED naming `TestUserContract`; the scan log names the worktree module and the base), `TestRepoContractGateRoot_UnresolvableTypedWorktreeFailsClosed` |
| `changedpkgs` | **one seed:** `ChangedFilesChecked(repoRoot, baseRef)` — tracked changes vs the base (`git diff --name-status`) plus untracked files (`git ls-files --others --exclude-standard`), each marked Added or not; `FromGitChecked` is its projection (`PackagesOf`) | `TestChangedFilesChecked_WorkingTreeNotIndex` (an unstaged edit, an index-added file and an untracked file all count; only the two the base lacks are Added), rename, not-derivable cases |
| `changedpkgs` | **one closure:** `ImporterClosureChecked` walks build deps AND the direct imports of each package's tests (the incident lives in a `_test.go`), returns `Closure{Patterns, Testable}` — `Testable` is what `go test` can run under the default build context (a tag-only or removed directory is a seed, never a target: `go test ./tagonly/...` is "matched no packages", exit 1) — and says when the module could not be listed. `ImporterClosure` keeps its input-preserving contract as the projection, so `regressiontia`'s selection now sees test-only importers too | `TestImporterClosure_TestOnlyImporter` (`internal/routingeval`'s tests import `internal/core`; its build deps do not), `TestImporterClosureChecked_TestableExcludesTagOnlyDirs`, `TestImporterClosureChecked_NotDerivableOutsideAModule`; the existing router/routingtest, non-importer and transitivity pins still hold |
| `ship` gate | the added-test backstop selects Added `_test.go` files from the shared seed (index-added OR untracked); the **importer backstop** runs the closure's testable patterns minus what an earlier layer already ran in the default context (the fixed pack, the untagged added-test groups), through the same classified pack runner, **bounded**: one run inside `importerBackstopTimeout` (20 min), and the ambiguous-exit retry only up to `importerBackstopRetryMaxTargets` (25) targets — above that an ambiguous exit is classed infra at once rather than paying for a second near-module run; the layer's wall time is logged to the scan log. Discovery failure (git or `go list`, one retry after a one-second pause) is the distinct, re-dispatchable `REPO_CONTRACT_INFRA` — a skipped guard on a red-main gate must never read as a healthy green | `TestRepoContractGate_ImporterOfAChangedPackageBlocksShip` (unstaged incident shape; the test-only importer is in the closure), `TestRepoContractGate_UntrackedAddedRedTestBlocksShip`, `TestRepoContractGate_UnimportedChangeRunsOnlyItself`, `TestRepoContractGate_NoGoChangeSkipsTheImporterBackstop`, `TestRepoContractGate_TagOnlyChangeIsSaidNotRun`, `TestRepoContractGate_DiscoveryFailureIsInfra`, `TestRepoContractGate_AddedTestDiscoveryFailureIsRecorded` (re-pinned to the infra class), the added-test scope split (a red modified test is caught by the importer backstop, not the added-test detector) |

## What it costs

The closure is whatever the graph says: a leaf change runs one package; a change to `internal/core` runs most of the module — what main's CI would run, moved to before the push. A red importer now fails the lane honestly in place (`REPO_CONTRACT_GATE`, naming the test) instead of redding main for every other lane.

## Bounds (stated, not closed)

- A test's imports count one hop: the helper a test imports is in the closure through its own deps and its tests assert its contract; following a test import's deps would pull every package whose tests share a fixture (`test/fixtures` links `core`) into every closure.
- A **modified** test in a tag-only package is covered by neither layer (the added-test backstop runs added files under their tags; the importer backstop runs the default context). CI remains its backstop.
- `go.mod`/`go.sum` changes do not seed the closure; CI remains the backstop for dependency bumps.
- `regressiontia.ChangedScope` inherits the widened closure: a change now selects the predicates of its test-only importers too, so TIA skips fewer predicates per cycle (`TestChangedScope_SelectsTestOnlyImporters`).
- A lane can still ship a tree whose Go files the seed cannot see only by putting them outside `go/` — which the module does not build.

## Follow-ups

- The lane's own attempt at this item (cycle 1673, stranded on the plane's local main by a rejected push) replaced the fixed pack with `./...` on every ship; it is superseded by this change and reconciled at the plane's next sync.
