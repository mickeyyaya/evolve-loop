# Build Explanation — Cycle 2523

## Build Binding
- Cycle: 2523
- Base SHA: 46440c64f0cb79babc4f653dd684468b1b77ee7d

## Summary
The `evolve test audit` command and the tree ratchet now enforce per-package `internal/testaudit` baselines across the module.

## Rationale
The `internal/testaudit` analyzer previously had no CLI or tree caller. Wiring both prevents new weak tests and removes stale rows without editing protected scopes or foreign predicate packages.

## Changed Areas
- `go/cmd/evolve/cmd_test_audit.go` — implements `evolve test audit` with `--rebaseline`, `--check`, `--root`, and `--json`.
- `go/cmd/evolve/registry.go` — registers the `test` command so `evolve test audit` dispatches from the CLI.
- `go/internal/testaudit/analyze.go` — adds `AnalyzeTree` and relative-path support so tree checks produce clean module-relative package keys.
- `go/internal/testaudit/ratchet.go` — shares `diffFindings` and `readShardFile` between single-package and tree-wide shard checks.
- `go/internal/testaudit/tree.go` — adds `Rebaseline` with set-based refusal and `CheckTree` over all local shards and central unwritable baselines.
- `go/internal/testaudit/unwritable-baselines.json` — stores baseline rows for the 93 protected and foreign-predicate packages that a lane branch cannot write directly.

## Design Decisions
Writable packages store baseline rows in `<pkg>/testdata/testaudit-baseline.json`. The 93 unwritable packages store their rows in `go/internal/testaudit/unwritable-baselines.json`, so a lane branch commits without `guards.json` exemptions.

## Verification
Unit tests in `go/cmd/evolve` and `go/internal/testaudit` pass, `TestTreeRatchet` passes across all 288 baselined packages, and all 10 cycle 2523 ACS predicates pass.

## Compatibility
Existing `Analyze`, `WriteShard`, and `CheckShard` signatures and behavior remain unchanged for callers outside this change.

## Limitations
The `unwritable-baselines.json` file tracks packages listed in its `packages` array. New protected packages with findings require registering their package key there before `--rebaseline` targets the central file.
