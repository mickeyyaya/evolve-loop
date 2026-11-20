# Build Explanation — Cycle 2558

## Build Binding
- Cycle: 2558
- Base SHA: 499c111b5f3a1ddb8e30fec6de050faf4d5233ce

## Summary
`bundleVerified` in `go/internal/gc/salvage.go` now checks `refs/remotes/origin/main` before creating a bundle. When `origin/main` holds the tip, both leaf writers skip `refs.bundle`; otherwise they pass `^refs/remotes/origin/main`.

## Rationale
`bundleBranch` in `go/internal/gc/branchsweep.go` and `writeSalvageLeaf` in `go/internal/gc/salvage.go` both call `bundleVerified`. Placing the `origin/main` check and negative refspec in `bundleVerified` fixes both writers and the dev archive path in one function without duplicating range logic.

## Changed Areas
- `go/internal/gc/salvage.go` — updates `bundleVerified` and adds `hasOriginMain` and `commitsAhead` to skip `refs.bundle` when `origin/main` holds the tip and bound other bundles with `^refs/remotes/origin/main`.
- `go/internal/gc/salvage_bundle_steps_test.go` — adds unit coverage in `TestSalvageBundleVerified_OriginMainBranches` for ahead-of-`origin/main`, in-`origin/main`, and missing-`origin/main` paths.
- `docs/architecture/packages/internal-gc.md` — documents the `refs/remotes/origin/main` thin-bundle and zero-count behavior of `bundleVerified`.

## Design Decisions
When `refs/remotes/origin/main` is absent or `rev-list --count` returns no integer, `bundleVerified` falls back to a full bundle (`git bundle create <file> <ref>`). Changing `bundleBranch` and `writeSalvageLeaf` separately was rejected because both share `bundleVerified`, and `bundleBranch` already writes `HEAD` whenever `bundleVerified` returns `nil`.

## Verification
All seven predicates in `go/acs/cycle2558`, the `go/internal/gc` package test suite, `gofmt`, `go vet`, and `evolve docs ste-lint --strict` verify the change.

## Compatibility
Leaves without `refs/remotes/origin/main` keep full verified bundles, and `ListSalvage` reads leaves that hold `HEAD` without `refs.bundle`.

## Limitations
`bundleVerified` bounds bundles against `refs/remotes/origin/main` only; repositories using another remote or default branch name fall back to full bundles.
