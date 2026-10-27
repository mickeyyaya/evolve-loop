# internal/regressiontia

## Purpose

`internal/regressiontia` computes test-impact selection over the EGPS Go regression corpus (`go/acs/regression/<sub>`) and writes it as evidence. It is observability in the `shadow` stage. It reports which regression predicates a change can skip, and it skips none. The audit phase's `generateACSVerdict` is its one production caller. The `acssuite` gate runner stays untouched.

## Design

- The package is separate from the gate runner because `go/internal/acssuite` is protected control plane (`guards.ProtectedSurfaceManifest`). A cycle may not edit the gate that grades it. The shadow stage changes nothing about what the gate runs, so it needs no code there.
- Only the future `enforce` stage, which skips packages, will have to live in `acssuite`. That change is human-gated: `evolve ship --class manual` runs outside a cycle.
- `Select` partitions the corpus into `selected` and `wouldSkip`. `Compute` produces one cycle's `Decision` from the resolved stage, the repo root, the module dir and the forward-only changed set. `ChangedScope` widens the changed set with reverse dependencies. `Emit` writes the decision to `<workspace>/acs-tia-shadow.json`.
- `Decision.WouldSkipCount` is a projection of `WouldSkip`. It is never a separate tally.
- Dependency data comes from two `go list` calls, bounded by `depsTimeout`. The test-package listing uses `-test`. `-e` keeps one unloadable package from failing the whole listing.

## Invariants

- Selection must never hide a regression class. Every fail-safe resolves toward running a predicate. An unknown changed scope skips nothing. Unresolved dependency data skips nothing. An unknown stage is off.
- A package is a skip candidate only when its dependency set is known and provably disjoint from the scope. `selected` plus `wouldSkip` is always exactly the input, in input order. Selection may reorder packages, but it never loses or duplicates one.
- An empty scope means unknown impact, never zero impact. An empty corpus decides nothing.
- A dormant or unrecognized stage computes nothing and returns the zero `Decision`. The caller then writes no artifact, so the audit path stays byte-identical under the live configuration.
- `Emit` with an empty workspace is a loud error. Evidence that was never written must not look like evidence of nothing to report.
- A broken evidence sink degrades quietly. It must not fail the audit.
- The reverse-dependency closure only widens. Narrowing below the forward-only baseline is worse than no selection. An empty repo root or a `go list` failure returns the input unchanged. Since 2026-09-14 the closure also follows the direct imports of each package's tests. A change therefore selects more predicates and skips fewer.
- An escape-hatch package reads repository files or spawns a subprocess. The stdlib packages in `escapeHatches` mark such a package. Its import graph does not show what it observes, so its impact set cannot be derived. `resolveDeps` drops such a package, and `Select` treats it as unresolvable and runs it.
- `go/acs/regression/apicover` is the case that makes the escape-hatch rule necessary. It reads `go/.apicover-enforce` and shells out to `go list`, and it imports none of the code it grades. A naive importer-graph selector will mark it skippable on exactly the diff it exists to catch.
- `listCorpusDeps` must use `-test`. A regression predicate package holds only `_test.go` files, so its plain dependency set is empty. Without `-test`, every corpus package will appear to depend on nothing. The fail-safe will then skip nothing, forever.
- `testPackageOwner` maps the synthetic packages that `go list -test` emits (`X.test`, `X [X.test]`, `X_test [X.test]`) back to `X`.
- The module-root pattern `./...` normalizes to the empty prefix, which intersects everything.

## Findings

- The reverse-closure rule exists because the forward-only scope hid the cycle-1250 class. A change confined to `internal/router` skipped `internal/routingtest`, which holds the keystone parity invariant. Main stayed red for five commits. `TestChangedScope_WidensByReverseDependency` reproduces that case on the real import graph.

## The shadow rollup (2026-10-21)

- `Rollup(runsDir)` reads `acs-tia-shadow.json` and `acs-verdict.json` from each `cycle-<N>` directory, in cycle order. A run directory without the shadow artifact is not counted.
- `Summary` holds the cycle count, the totals of the selected packages and of the packages the selection can skip (`would_skip`), and each `Miss`. A miss is a red result with `is_regression: true` whose base name matches a skippable package. `acssuite` writes a regression predicate as `go/acs/<base>/...:<Test>`, with no `regression/` segment, so the rollup matches on the base name of `./acs/regression/<base>`. The rollup decodes each result as `acssuite.Result`, so the two cannot drift. `Decision.DurationMS` records the time of each selection, and the summary keeps the total and the largest.
- Limit: the rollup does not tell apart two regression packages with one base name. The corpus is one flat directory, so this does not occur today.
- A cycle with a shadow decision and no verdict is listed in `NoVerdict`. A file that does not parse is listed in `Unreadable`. A runs directory that does not exist is an error.
- `evolve acs tia-report` prints the rollup, or `--json` prints the `Summary`.
- Pinned by `TestRollup_CountsTheShadowDecisionsAndNamesEachMiss`, `TestRollup_AMissingRunsDirIsAnError` and `TestRollup_ARealCycleVerdictNamesTheMissOfASkippedRegressionPackage`. The last test reads a verbatim copy of the `acs-verdict.json` of cycle 2027 (`testdata/`).
- **Module-local walk.** `listCorpusDeps` lists the direct imports of each package (`-deps`, `-test`). It walks only the packages of the module, and it records each module package and each escape-hatch import that it meets.
- The walk ignores the generated `X.test` main package, because that package imports `os` for every test binary. A stdlib package such as `testing` is not walked.
- A package that reaches `os` through a module package (for example `apicover` through `acsassert`) stays an escape hatch.
- Pinned by `TestCompute_SkipsAPredicateThatReachesOsOnlyThroughTesting` and `TestC2053_001_PackageWithNoEscapeHatchInItsModuleLocalGraphIsSkippable` and `TestC2053_002_ReachingOsThroughAModulePackageStaysAnEscapeHatch`.

