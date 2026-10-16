# Build Explanation — Cycle 1881

## Build Binding
- Cycle: 1881
- Base SHA: 26838170cd0a90a9de528b45d9ae6bfe10a4b71b

## Summary
A new Go binary `vulnfilter` consumes `govulncheck -json` output and filters findings against a tracked allowlist `go/vulncheck-allowlist.json`. The `make vulncheck` target now writes the json stream to `go/bin/vulncheck.json` and feeds that file to `vulnfilter`. The target fails when `govulncheck` exits non-zero, and `vulnfilter` fails on an unlisted finding, an expired entry or a reasonless entry. A documented, unexpired waiver lets a finding through.

## Rationale
Previously, `make vulncheck` ran `govulncheck` in text mode with no allowlist mechanism. When a vulnerability was disclosed without an available upstream fix, the build can block without an emergency escape hatch. Adding `vulnfilter` with an explicit allowlist schema (`id`, `expires`, `reason`) enables documented, time-bounded waivers without disabling CI gates or muting security checks.

## Changed Areas
- `docs/conventions/third-party-dependencies.md` — documents the emergency allowlist escape mechanism and requirements for allowlist entries.
- `go/Makefile` — updates the `vulncheck` target. The target writes `govulncheck -json` to `bin/vulncheck.json`, exits with the scanner's status on a scanner failure, and then feeds the file to `vulnfilter`.
- `go/cmd/vulnfilter/filter.go` — implements JSON stream decoding, allowlist loading, expiry validation, and summary formatting without comments.
- `go/cmd/vulnfilter/filter_test.go` — provides unit tests for date parsing, allowlist loading, stream filtering, and summary appending without comments.
- `go/cmd/vulnfilter/main.go` — provides the CLI entry point for `vulnfilter` with the `-allowlist` flag and exit code handling without comments.
- `go/internal/ciparity/workflows_test.go` — updates the workflow parity test to verify that the `vulncheck` Makefile target runs `govulncheck -json`.
- `go/vulncheck-allowlist.json` — establishes the git-tracked JSON array of allowed vulnerability exceptions, initialized empty.

## Design Decisions
The recipe does not use a pipe. In a POSIX `sh` pipe, the exit status is the status of the last command only. Thus a `govulncheck` failure with empty or partial stdout passed through a green `vulnfilter` (round-1 audit finding H1). `set -o pipefail` was rejected because the CI runner's `/bin/sh` (dash) cannot be relied on to support it. `SHELL := bash` was rejected because it changes every recipe of the Makefile. A temporary file under `$(BIN_DIR)` follows the existing `apicover-check` recipe, which keeps `apicover-dirs.txt` there.

The filter runs as a standalone `main` command in `go/cmd/vulnfilter` rather than introducing a new `internal` library package. Expiry dates support RFC3339 and `2006-01-02` (DateOnly) formats to help manual authoring. Reached vulnerabilities require an active function in their call trace to avoid false positives on uncalled module/package dependencies. If `GITHUB_STEP_SUMMARY` is defined, summaries are appended rather than overwritten.

## Verification
Verified with unit tests in `go/cmd/vulnfilter/filter_test.go`, the parity test in `go/internal/ciparity/workflows_test.go`, `make -C go vulncheck`, and 19/19 ACS predicates in `go/acs/cycle1881`. `TestC1881_019` runs `make vulncheck` with a fake `go` that stubs `govulncheck`. On the piped recipe it failed for a scanner that exits non-zero with an empty stream or with a clean stream. On the current recipe it passes.

## Compatibility
`go/vulncheck-allowlist.json` starts as an empty array, preserving strict zero-finding enforcement when no waivers are configured. The `make vulncheck` target continues to exit non-zero on any unlisted reachable vulnerability. The first build of this cycle piped `govulncheck` into `vulnfilter`, so a scanner error passed the gate. The current recipe removes that regression: a scanner error now fails the target with the scanner's exit status. In JSON mode `govulncheck` exits 0 when it reports findings, so a finding still reaches `vulnfilter`.

## Limitations
Only reachable vulnerabilities with call stack functions are evaluated against the allowlist.
