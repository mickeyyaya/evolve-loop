# Build Explanation — Cycle 2638

## Build Binding
- Cycle: 2638
- Base SHA: 19a3c3e6680780d00891286b92d26de8d4e97a90

## Summary
`internal/gittest` exports its helper cost table with `StartCosts`. `internal/gitstarts` reads that table instead of a switch, and `MissingCosts` guards against uncosted exported git starters.

## Rationale
Keeping helper process counts in `gittest` next to the helpers puts cost data in one home. It fixes the bug where `Repo.CloneBare` and `BranchExists` cost zero.

## Changed Areas
- `go/internal/gittest/cost.go` — exports `StartCosts` as a cloned map of `gittest` helper process costs (`Fixture`, `Bare`, `At` = 5; `Clone`, `Repo.CloneBare`, `BranchExists`, `Repo.Git` = 1; `FromTemplate`, `BareFromTemplate` = 0).
- `go/internal/gitstarts/gitstarts.go` — replaces the `callCost` switch with `gittest.StartCosts` lookups and adds `MissingCosts`.
- `docs/architecture/packages/internal-gittest.md` — documents `StartCosts` and the `TestEveryGitMethodHasCost` guard invariant.
- `docs/architecture/packages/internal-gitstarts.md` — updates the `Count` design and invariants for `gittest.StartCosts` and `MissingCosts`.

## Design Decisions
`StartCosts` returns a fresh `maps.Clone` copy so callers cannot mutate package state. `TestEveryGitMethodHasCost` lives in package `gittest_test` because `gitstarts` imports `gittest`. An in-package test causes an import cycle. `MissingCosts` detects direct `.Git` calls, `exec`/`sysexec` `Command`/`CommandContext` calls with `"git"`, and intra-package helper calls. That rule avoids classifying `exec.LookPath("git")` in `InstallFault` as a git process start.

## Verification
Targeted tests in `go/internal/gitstarts`, `go/internal/gittest`, and `go/internal/core` pass. The cycle 2638 ACS predicates also pass and confirm measured counts, table lookups, guard checks, apicover, and unchanged ceilings.

## Compatibility
`gitstarts.Count`, `gitstarts.Verify`, and all `gittest` helper signatures stay unchanged. `go/internal/core/testdata/gitstarts.json` stays at `recorded: 730` (727 counted starts).

## Limitations
`callCost` matches non-`gittest` selector calls by `Repo.<Method>` name without type resolution. An unrelated receiver method with the same name as a `Repo.` entry can receive that cost.
