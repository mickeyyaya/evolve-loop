# internal/ciparity

## Purpose

`internal/ciparity` holds the pure helpers of the cycle audit's CI-parity gate. The gate runs the exact whole-repo CI command set (`go vet ./...`, `-tags acs acs-durable`, `apicover -enforce`) against a cycle's worktree before ship. A cycle therefore cannot ship green locally and red in CI. The package is a leaf: no subprocess and no I/O. The audit phase (`internal/phases/audit`) supplies the shell-out hooks and calls these functions to decide scope and to extract offenders.

## Design

- `IntersectEnforced` narrows the apicover run to the enforced packages the cycle touched. Running the whole enforce list each cycle will cost O(repo), not O(change).
- `NewUngraduatedPackages` finds the changed packages under `./internal/` that are not yet in `go/.apicover-enforce`. `IntersectEnforced` silently drops exactly those, so the detector keeps them visible.
- `GraduationPrescription` renders the two edits per ungraduated package: the enforce line and the `apicover_named_test.go` file. The build-entry seam and the audit seam both use it, so both print the same fix.
- `PackageDirHasProductionGoFiles` is the one predicate both graduation seams consult. A test-only package has no exported production symbol, so its graduation obligation is vacuous.
- `CoverageTags` is the one build-tag set. Every scoped coverage-measuring `go test` call builds through `CoverageTestArgs`.
- `RequiredComposedGates` and `MissingComposedGates` define the full native gate set. A composed tree must record each gate as passing before ship's trivial-rebase fast path accepts it. The gates are the CI-parity runners the cycle audit uses (ADR-0069).
- `DarwinPackages` derives the darwin set, the packages that the macOS CI job tests (plan C2). A package is in the set when one of four rules holds:
- Rule one: its file list under `GOOS=darwin` differs from its file list under `GOOS=linux`. Rule two: a file names `darwin` in a `//go:build` line outside a `!`. Rule three: a file uses `runtime.GOOS`, except in a comparison with a literal other than `darwin` or `linux`. Rule four: a non-test file holds the string `sandbox-exec`. Test files count for the first three rules.
- `DarwinSetText` writes `go/.darwin-set`, and `cmd/darwinset` is the thin command over `DarwinMain`. `TestDarwinSetFile_MatchesTheSetDerivedFromTheTree` fails when the committed file differs from the derived set.
- `E2ETestNames` and `E2ERunPattern` build the `-run` pattern of `make test-e2e`. A test belongs to the e2e tier when only the `e2e` tag adds its file to the package. `cmd/e2elist` is the thin command over `E2EMain`.
- **The darwin set has one writer.** `SyncDarwinSet(moduleDir, write)` derives the set and compares it with `go/.darwin-set`. With `write` it writes the file. Without `write`, a different file returns `ErrDarwinSetStale`. `evolve ci darwin-set <generate|check>` and the `darwin-set` entry of `internal/derived` call it, so `evolve pr refresh` can write the file after a rebase (plan [pr-merge-latency-2026-10](../../plans/pr-merge-latency-2026-10.md) P3). Pinned by `TestSyncDarwinSet_ChecksThenWritesTheSetFile`.

## Invariants

- Scope is `./internal/` only, because apicover scopes only that tree. `./cmd/...` entrypoints are never flagged.
- A recursive pattern (one that contains `...`) names no single directory. It is reported as ungraduated and never guessed about. Its prescription names the obligation for each package the pattern covers, not a path under the pattern.
- A package whose directory cannot be read fails open. A package that cannot be proven to bear the obligation must not fail a phase. The repo-wide CI gate stays the backstop.
- `CoverageTestArgs` keeps the package list verbatim and in order. Dropping the list will silently fall back to the whole module.
- A coverage run must use `CoverageTags`. An untagged run under-reports a tag-gated package by a large margin. In `internal/phases/ship`, the measured figure was 47.0% untagged and 90.6% tagged. Gating on the untagged figure lets a wrong number ship. ADR-0069 fixed the same defect class for vet and apicover.
- A composed-tree gate counts as missing when its key is absent or its status is not `pass`. A nil result means the fast path may stay open.
- The `IntersectEnforced` input forms are fixed. Changed entries carry a trailing `/...`. The enforce file holds one bare `./internal/foo` per line, with `#` comments and blank lines. Output is deduplicated and sorted, so it is deterministic. Empty changed input returns nil, and the caller then skips the scoped run.
- Each job of `go.yml` declares `timeout-minutes` from 1 to 90. A hung job then cannot hold the concurrency group of main for the runner default of 360 min ([plan ci-wall-time-2026-10](../../plans/ci-wall-time-2026-10.md) D27). Pinned by `TestGoWorkflow_EveryJobHasABoundedTimeout`.

## Findings

- A graduation seam must consult `PackageDirHasProductionGoFiles` before it flags a package. Without that check, a package can pass the build seam and fail at audit for the same vacuous reason.
