# Build Explanation — Cycle 1865

## Build Binding
- Cycle: 1865
- Base SHA: 164bea7f503be47ca8dd89e0db4db9decf34db6b

## Summary
This cycle implements the `evolve doctor versions` subcommand, supporting `--latest` and `--json` flags to report installed CLI versions and detect lagging installations against upstream releases.

## Rationale
The new verb is read-only, so an operator can run it at any time to see which CLI versions are installed. Three alternatives were considered:
- Extend `evolve cli update` (`go/cmd/evolve/cmd_cli.go`). Rejected: that verb installs updates and refuses while a run holds a live lease. A version report must not install anything, and it must work inside a wave.
- Read a cached release catalog. Rejected: the repo has no cached catalog of npm releases, and adding one would bring a cache and its refresh policy into scope. The tradeoff is that `--latest` needs network access.
- Exit 0 when a CLI lags. Rejected: exit 1 lets a script or preflight gate act on lag without parsing the output. The tradeoff is that a failed lookup also exits 0, so offline hosts never fail.

## Changed Areas
- `docs/operations/runtime-reference.md` — documents the `evolve doctor versions [--latest] [--json]` operator verb.
- `go/internal/cli/opscmd/doctor.go` — routes the `versions` subcommand in `RunDoctor`.
- `go/internal/cli/opscmd/doctor_versions.go` — implements version inventory detection, network-tolerant release checks, and text/JSON formatting.

## Design Decisions
The versions subcommand delegates to `looppreflight.CLIVersion` for installed version detection. Package-level function variables `doctorInstalledVersions` and `doctorLatestRelease` provide deterministic seams for unit test verification. Release lookup failures degrade gracefully to `unavailable` and exit 0 to prevent offline environment flakiness.

## Verification
Unit tests in `go/internal/cli/opscmd/doctor_versions_test.go` and ACS predicates in `go/acs/cycle1865/predicates_test.go` verify text output, JSON payload formatting, `--latest` flagging with exit 1, and failed lookup handling with exit 0.

## Compatibility
Existing subcommands of `evolve doctor` (`probe`, `boot`, `live`, and `plane`) remain unchanged.

## Limitations
- Only `claude` and `codex` have a release source (`go/internal/cli/opscmd/doctor_versions.go:38-43`). For agy, gemini and ollama, `--latest` always prints `unavailable` and never flags lag. agy is the primary routed CLI, so the largest share of dispatches gets no lag check.
- The lookup is a live HTTPS GET to `registry.npmjs.org`, with a 3-second timeout per CLI (`go/internal/cli/opscmd/doctor_versions.go:35`). Lookups run in sequence, and no result is cached.
- The comparison reads only the leading digits of each dot-separated part (`parseVersionParts`). It ignores pre-release suffixes, so `1.2.3-beta` and `1.2.3` compare as equal.
