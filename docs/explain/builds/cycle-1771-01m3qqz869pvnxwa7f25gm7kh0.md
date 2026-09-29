# Build Explanation — Cycle 1771

## Build Binding
- Cycle: 1771
- Base SHA: 6aa43b70d6d0f68302ce12e2ab2fe54180394d79

## Summary
Four functions that exceeded the repo-wide 50-line function-size ratchet — `tokenusage.ScanConfigRoot` (60), `llmcalls.Aggregate` (53), `changedpkgs.DirectImporters` (79) and `addedtests.BuildTags` (56) — are shrunk to 46, 32, 33 and 37 lines by extracting named, unexported helper steps. Behavior is unchanged and `offenders.json` is untouched, so the old allowances remain as unused ceiling.

## Rationale
The ratchet allowance is a ceiling, not a target: lowering the functions below the limit lets a later cycle drop the offender entries without re-measuring. Pulling out whole existing blocks unchanged into named helpers is the smallest refactor that keeps behavior byte-for-byte, and it needs no new tests because each package already covers the function end to end.

## Changed Areas
- `go/internal/tokenusage/scanner.go` — the per-transcript assistant-usage loop inside the `WalkDir` callback moves to `recordAssistantUsage`; `ScanConfigRoot` keeps the walk, the attribution check and the sum/peak reduction.
- `go/internal/llmcalls/aggregate.go` — the seven-key `sort.Slice` comparator moves to `lessPerformance(a, b Performance) bool`; the key order is unchanged.
- `go/internal/changedpkgs/direct_importers.go` — the module walk moves to `importersOf`, and the per-file import match moves to `importsTarget`, which returns on the first exact-path hit (the same as the old `break`). `DirectImporters` keeps input validation, target building and the sorted `./rel` output.
- `go/internal/addedtests/addedtests.go` — the `//go:build` header scan (open, scan leading comment lines, parse) moves to `goBuildExpr`; `BuildTags` keeps the default-context match, the tag-set ceiling and the subset search.
- `docs/explain/builds/cycle-1771-01m3qqz869pvnxwa7f25gm7kh0.md` — this explanation document.

## Design Decisions
Each helper is an unexported function taken verbatim from the original body, not a new abstraction. Existing comments (the exact-match note and the fail-open walk note) move with their code word for word, so the comment audit counts them as moved, not added. No helper has a doc comment, because the name states what it does. Error precedence in `goBuildExpr` stays the same as before: open error, then scanner error, then parse error.

## Verification
The cycle-1771 ACS predicates 001–006 pass (size fit, offenders.json unchanged, module-wide `sizeratchet.Check`, baseline test declarations intact, package suites, no comments added). The `tokenusage`, `llmcalls`, `changedpkgs`, `addedtests` and `sizeratchet` package tests also pass with `-count=1`.

## Compatibility
No exported signature, type or behavior changes. The new helpers are package-private.

## Limitations
`offenders.json` still lists the four functions at their old sizes. Removing those now-slack entries is left to a separate ratchet-tightening change, as the task requires.
