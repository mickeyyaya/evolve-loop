# internal/testmainexit

> Consumers: its own module-wide test, `TestModuleTestMainsNeverDeferCleanupPastOsExit`, which runs in `go test ./...` (and so in CI and the build floor).

## Purpose

`testmainexit` finds `TestMain` functions whose deferred cleanup never runs. `os.Exit` skips deferred calls, so a `TestMain` that makes a temp dir, `defer`s its removal and ends with `os.Exit(m.Run())` leaks that dir on every run. Since Go 1.15, a `TestMain` that returns uses `m.Run()`'s result as the exit code, so the fix is always to return.

- `SkippedDefers(path)` parses one Go file and returns the line of every `defer` in a `TestMain` (a function with no receiver) whose own body calls `os.Exit` directly, spelled `os.Exit` with the package unaliased. It does not follow a helper that exits, an aliased `os`, or `log.Fatal` (which also exits); none occurs in the module today, and a new occurrence is a reason to extend it. A `defer` inside a function literal belongs to that closure and runs when it returns, so it is excluded. A file that cannot be read or parsed is an error, never a clean result.

## Design

- **Named for what it checks, not as a guard.** It is a test-hygiene check like `rawgitratchet` and `sizeratchet`. A `guard`-named file joins the trust-kernel perimeter (`TestEveryGateShapedFileIsProtectedSurface`), which a hygiene check is not.

- **The module's test files come from `rawgitratchet.BoundTestFiles`.** That is the one rule for which test files the module binds (git-tracked, index-staged included, on-disk as the strict fallback), reused rather than repeated.
- **No allow-list.** Every offender is fixed; nothing is exempted.

## Invariants

- **The checker finds the skipped defers of a direct `os.Exit`.** A defer followed by `os.Exit(m.Run())` is reported. A returning `TestMain`, an `os.Exit` without a defer, cleanup run before `os.Exit`, a defer in an ordinary test, and a defer inside a closure are not. Pinned by `TestSkippedDefers_FindsACleanupThatOsExitSkips`.
- **No module test defers cleanup past `os.Exit`.** Pinned by `TestModuleTestMainsNeverDeferCleanupPastOsExit`.
- **Every export is named by a test** (`.apicover-enforce`).

## Findings

- **2026-09-28, the disk-full halt.** `acs/regression/cycle1515` and `acs/cycle1498` leaked a roughly 22 MB `evolve-under-test` binary per run, 1,427 dirs and 32 GB in all. The durable suite runs in every audit's CI-parity gate, every floor and CI. See [the incident](../../incidents/2026-09-28-the-disk-filled-and-two-lanes-failed-for-it.md).
