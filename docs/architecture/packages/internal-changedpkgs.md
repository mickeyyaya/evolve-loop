# internal/changedpkgs

> Reverse-dependency selection: [ADR-0082](../adr/0082-regression-test-impact-selection-shadow.md) and [the regression-TIA chronicle](../../chronicle/2026-08-regression-tia.md). The working-tree seed, the test-import hop and `Closure.Testable`: [the lane ship-gate incident](../../incidents/2026-09-14-lane-ship-gate-package-scoped-tests.md). The covering-test corpus (`CoveringTests`, `DirectImporters`): [micro-phase catalog, test-amplification](../micro-phase-catalog.md). This page keeps the package-level detail those do not.

## Purpose

`internal/changedpkgs` answers "which Go packages does this change touch, and which packages can that change break?". Predicates, the audit's CI-parity gates, the router digest, the build floor, the ship gate and test-impact selection all read their change set from here, so `go test` runs scoped to the change (O(change)) instead of `./...` (O(repo)).

## Design

- **Two seeds.** `ChangedFilesChecked` is the one git derivation (working tree versus a base ref); `FromGitChecked` and `PackagesOf` are projections of it. `ChangedPackages` reads the builder's legacy `handoff-build.json` (`thrusts[].files_modified` and `files_new`). The builder has not emitted that file since about cycle 215, but `acssuite.changedPackagesForCycle` and the audit's `changedPackagesForAudit` still try it first.
- **Checked and best-effort pairs.** `FromGit`/`FromGitChecked` and `ImporterClosure`/`ImporterClosureChecked` differ only in the second result. The best-effort form swallows a derivation failure; the checked form reports it, and a gate that must fail loud on an underivable set uses it.
- **Shared vocabulary.** `repoRoot` is always the repository root, the directory that holds the `go/` module directory. A package pattern is go-module-relative: `FileToPackage` emits the recursive `./dir/...`, and core's `changedGoTestPackages` emits the bare `./dir`.
- **Forward, then reverse.** `FileToPackage` maps a file to the package it lives in. The reverse direction has two derivers with different costs and depths:
  - `ImporterClosure` runs one `go list -e` over the module. It keeps every package whose transitive build deps (`transitiveDeps`) reach an input, or whose tests' direct imports (`testImports`) do. `-e` keeps one unloadable package from failing the whole listing. A test import counts one hop only. Following its deps pulls every user of a shared fixture into every closure.
  - The listing runs under `importerClosureTimeout` (120 seconds), for a cold module cache. Past it, the input set comes back unchanged and `ok` is false, so a predicate never hangs on the closure. No test pins the timeout, so that property is unpinned.
  - `DirectImporters` parses import blocks only (`go/parser`, `ImportsOnly`) and keeps packages that import an input directly, counting imports from `_test.go` files. It skips the directories that `holdsNoCoveringTest` names (`acs`, `testdata`, `vendor`, `.git` and `node_modules`). `acs` holds the cycle predicates, not the product, and none of them holds a covering test. An unparseable file or an unreadable subtree contributes no importer.
- **Covering-test corpus.** `CoveringTests` walks each pattern's directory recursively for `_test.go` files. `patternDir` maps the recursive and bare pattern forms to the same directory; the walk is a superset for the bare form, and over-listing a sub-package's tests is harmless in an advisory corpus.
- **`Closure` carries two views of one walk.** `Patterns` is the selection. `Testable` is the subset under which `go list` finds Go files in the default build context, which is what `go test` can run without tags.

## Invariants

- **Working tree, never the index.** A lane's build output is unstaged and its new files untracked until the ship stages them, so an index-based seed at gate time sees nothing. Pinned by `TestChangedFilesChecked_WorkingTreeNotIndex`.
- **Not derivable is never "nothing changed".** Empty inputs, no repository, a bad ref or a concurrent `index.lock` return `ok == false`; a clean tree is `(nil, true)`. Pinned by `TestChangedFilesChecked_NotDerivable` and the `TestFromGitChecked_*` tests.
- **A rename is one Added path, at its destination.** The source left nothing behind to import. Pinned by `TestChangedFilesChecked_RenameIsAddedAtTheDestination`.
- **Under-scope, never misroute.** `FileToPackage` accepts only `.go` paths under `go/`; anything else is rejected rather than turned into a pattern, because `go test` on a nonexistent pattern is a false RED. A `.go` file at the module root maps to `./...`. Pinned by `TestFileToPackage`.
- **Patterns carry no whitespace.** `acssuite` joins the set into the space-separated `CHANGED_PACKAGES` env var. The `assert_go_test_pass_changed` helper of the builder iterates it unquoted. So a pattern with whitespace splits into bogus tokens. Pinned by the `go/internal/foo bar/baz.go` case of `TestFileToPackage`.
- **The corpus derivers fail open.** `CoveringTests` and `DirectImporters` return nil for every unusable input, and the phase falls back to its own search. The module-wide `./...` counts as unusable: naming every test file, or widening from every package, is worse than the blind search the corpus replaces. Patterns that escape the module are rejected. Pinned by the two `*_FailsOpenOnUnusableInput` tests.
- **Exact import-path match, never a prefix.** `example.com/other/internal/foo` and `.../internal/foobar` are different packages, and `dirByImportPath` is keyed by the full import path. Pinned by the `decoy` package in `importerFixture` and by `TestImportsTarget_SkipsMalformedSpecsAndMatchesExactPathsOnly`.
- **`DirectImporters` is one hop and excludes its inputs.** An unbounded closure would re-inflate the context the corpus exists to shrink, and the inputs are already in the corpus. Pinned by `TestDirectImporters_WidensToReverseImportersIncludingTestOnly`.
- **`ImporterClosure` only widens.** Every degenerate input (empty or non-module root, junk pattern, `go list` failure) returns the input set unchanged, because narrowing below the forward-only baseline would be worse than having no closure. The closure of `./...` is the identity. Pinned by `TestImporterClosure_BestEffortOnBadInput` and `TestImporterClosure_SortedDedupedAndModuleRoot`.
- **A tag-only or removed directory is in `Patterns`, never in `Testable`.** Its importers are exactly what breaks, but `go test` on it without the tag is "matched no packages", exit 1. Pinned by `TestImporterClosureChecked_TestableExcludesTagOnlyDirs` and `TestClosure_TestableIsASubsetOfPatterns`.
- **A test import is one hop, and the fixtures show it.** The importer fixtures of `importerclosure_test.go` name the packages that must stay out.
  - `gitexec` cannot reach `router`. But its tests import `test/fixtures`, whose deps do. A transitive test-import walk then includes `router`.
  - `acssuite` reaches `gitexec` only through `changedpkgs`.
  - `routingeval`'s tests import `core`, but its build deps do not.
  - `go/acs/cycle8` holds only `//go:build acs` files.
  - Pinned by `TestImporterClosure_RouterRoutingtest`, `TestImporterClosure_ExcludesNonImporters`, `TestImporterClosure_Transitive` and `TestImporterClosure_TestOnlyImporter`.
- **Output is sorted and deduped.** The corpus lands in an agent prompt, where a reordered set churns the prompt cache and blurs token measurement; gates compare and cache sets. `sortedDedup` never mutates its input. Pinned by `TestDirectImporters_DeterministicSortedAndDeduped` and `TestImporterClosure_SortedDedupedAndModuleRoot`.
- **The corpus derivers have production callers.** A deriver reached only from tests injects nothing. `TestCoveringTests_ReachableFromProduction` and `TestDirectImporters_ReachableFromProduction` parse the module for a non-test caller outside `acs`.

## Findings

- **cycle-200**: a predicate running `go test ./...` exceeded the per-predicate timeout on this repo and flaked to a false RED. Widening the timeout ([the proxy-failure report](../../research/verdict-and-gate-proxy-failure-class-2026-06-03.md)) only widened the window; scoping predicates through `CHANGED_PACKAGES` was the fix.
- **cycle-573**: the builder stopped emitting `handoff-build.json` around cycle 215, which left the apicover CI-parity gate silently fail-open on every real cycle (the third recurrence of `warnship_apicover_ci_gap`). Deterministic work must not depend on an LLM artifact, so `FromGit` derives the set from git.
- **cycles 581/582**: `FromGit` swallowed every git error, so the CI-parity gate could not tell a clean tree from a failed derivation and stayed fail-open on the cycle that most needed it (cycle-581 audit). `FromGitChecked` added the derivability signal.
- **cycle-1253** (from the cycle-1250 miss): `ImporterClosure`. See ADR-0082.
- **cycles 1255 and 1267**: `CoveringTests`, then `DirectImporters` for the half it could not see. See the micro-phase catalog.
- **2026-09-14 ship-gate incident**: the working-tree seed, the test-import hop and `Closure.Testable`. See the incident record.
