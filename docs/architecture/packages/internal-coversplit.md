# internal/coversplit

## Purpose

`internal/coversplit` plans the coverage runs of a test package that a team split from the package it tests. Go counts the coverage of a package only in the test binary of that package. Thus a split package runs with `-coverpkg`, and its profile joins the main profile.

Example: [`internal/phases/ship/shiptest`](internal-phases-ship-shiptest.md) holds black-box scenarios of `internal/phases/ship`. Without the merge, the coverage of the ship phase fell from 99.0% to 94.1% when its landing tests left the package (measured on 2026-10-28). With the merge, it stays at 99.0%.

## Design

- **The list.** `go/.cover-split` has one line for each split: `./<split package> ./<package it tests>`. `Parse` refuses a malformed line, a pattern, a split of a package from itself and a duplicate. `Load` reads the list of a module. An absent list means no split.
- **The plan.** `Plan` takes the packages of one run. A split whose tested package is in the run is active. `Plan` removes each active split package from the main run and gives it a run of its own with `-coverpkg=<tested package>,<split package>`. A split package without its tested package stays in the main run. `Run.Args` gives the arguments of one `go test`.
- **The merge.** `Merge` joins profiles under one mode line, and refuses a profile with another mode. The build floor calls it, and `make test-integration` calls it through `coversplit -merge`. Thus the join has one home. The parsers merge a block that two profiles hold (`covchanged.ParseProfile`, `go tool cover`). Thus a block that either run hits counts as hit.
- **The detector.** `Detect` walks the module. A package is a split when its test files import an ancestor package that its non-test files do not import. `Unlisted` names each split that the list does not hold, and each list line that is not a split.
- **The consumers.** `make test-integration` runs the plan through [`cmd/coversplit`](cmd-coversplit.md). The build floor of `internal/core` runs it in `scopedCoverProfile` and `runSplitCoverPasses`. Thus CI, `evolve cover changed` and the floor of a lane read the same coverage.

## Invariants

- Each split package has its line in `go/.cover-split`, and each line names a real split. `TestEverySplitTestPackageHasItsCoverEntry` pins this rule on the real tree. The test reads the whole tree, so `internal/repocontract` lists it in `TreeReadingTests`.
- A split run that fails fails the whole run. `make test-integration` then writes no `coverage.txt`, and the floor reports that the tests failed. `TestMakeTestIntegration_AFailedSplitRunFailsTheRunAndLeavesNoProfile` and `TestMakeTestIntegration_ASplitPackageCountsForThePackageItTests` (`internal/ciparity`) run the real recipe on a fixture module.
- The package has 100% statement coverage (`go/.cover-strict`), and a test names each export (`go/.apicover-enforce`).

## Limits

- The detector sees only a split under the directory of its tested package. A split package in another place needs its line by hand.
- The local `make apicover-enforce` does not use the plan. It measures the exports of a package with the tests of that package only. CI reads the joined `coverage.txt` through `apicover-check`.
- The audit gate `ciparitygate` does not use the plan. It judges only the exports of a changed package. On 2026-10-28, the tests of `internal/phases/ship` alone still covered each of its exports.
