# internal/marketplacepoll

> Related: [ADR-0062](../adr/0062-doc-impl-reconciliation.md) (task T1.7, no bash shell-out remains). This page keeps the package-level detail that the code cannot say.

## Purpose

`internal/marketplacepoll` is the post-publish verifier. It ports `legacy/scripts/release/marketplace-poll.sh`. It polls the local marketplace checkout, which Claude Code reads at session start. It stops when the `plugin.json` there matches the target version, or when the deadline passes.

## Design

- **Poll first, then refresh.** On success the package calls the `ReleaseSh` seam to refresh `installed_plugins.json`. This ordering fixed the cache-refresh bug of the bash script: the refresh runs only when the version match is already true (the ordering test).
- **Convergence is detected by the version check, not by git exit codes.** `DefaultPull` runs `git fetch origin main` and `git reset --hard origin/main`, and swallows both errors on purpose, as the bash script does. A directory that is not a git checkout is a silent no-op.
- **Testing seams.** A caller can inject `Now`, `Sleep`, `Pull` and `ReleaseSh` to bypass real time, real git and real shell calls. The defaults keep the external behaviour of the bash script. A zero value of a seam field selects the real implementation.
- **`ReadMarketplaceVersion` mirrors the bash pipeline** `sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1`. A missing or unreadable file returns an error. A file with no matching version field returns an empty string and no error, as the bash script does.
- **The exit codes map to sentinel errors.** The cmd layer maps nil to exit 0 (converged, refresh succeeded). It maps `ErrTimeout` to exit 1 (polled until `MaxWait` without a match). It maps `ErrRuntime` to exit 2: a missing directory, a malformed or missing `plugin.json`, an invalid semver target or a `ReleaseSh` failure. `Run` validates `Target`, `MarketplaceDir`, `MaxWait` and `PollInterval`, and reports a bad value as `ErrRuntime`. `Result` is also filled on error, so a caller can log diagnostics.

## Invariants

- **`DefaultReleaseSh` is a Go no-op and never runs a script.** The legacy `release.sh` was removed in the v12 script-to-Go migration. Its cache-refresh side effect is obsolete, and release consistency is covered by `internal/releaseconsistency`. Even a stray executable `legacy/scripts/utility/release.sh` that exits 1 is ignored. The `Options.ReleaseSh` seam remains for a caller that wants a real check. Pinned by `TestDefaultReleaseSh_IgnoresLegacyScript` and `TestDefaultReleaseSh_AlwaysNil`.
- **A stale version times out.** A marketplace that never reaches the target ends in `ErrTimeout`, not in a false success.
