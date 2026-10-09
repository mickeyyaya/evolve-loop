# Build Explanation — Cycle 1854

## Build Binding
- Cycle: 1854
- Base SHA: 8f9130fffee65c6e6eb98a57ac26c2604e6c84a2

## Summary
Memoizes CLI binary version inventory collection per Run in loop preflight, transitions version cache persistence to atomicwrite.JSON, stubs external process seams in unit tests, retires a stale cycle270 ACS predicate, and aligns preflight documentation with current check architecture.

## Rationale
Executing CLI binary probes multiple times per Run causes unnecessary latency and creates potential discrepancies between drift detection and reported versions. Writing version cache files via fixed temp file extensions collides under concurrent execution. Stubbing process seams ensures preflight unit tests execute hermetically without subprocess side effects.

## Changed Areas
- `go/internal/looppreflight/versioninventory.go` — replaces static temp file write and rename in saveVersionCache with atomicwrite.JSON for collision-safe atomic persistence.
- `go/internal/looppreflight/looppreflight.go` — memoizes versionInventory with sync.Once inside resolve() so binary probes execute exactly once per Run.
- `go/internal/looppreflight/looppreflight_test.go` — stubs CLIHealthActive, PhaseRoutingWarnings, OrphanKill, and VersionInventory in goodPipelineOptions while preserving default inventory probe testing.
- `go/acs/cycle270/predicates_test.go` — retires TestC270_004 with t.Skip because defaultTmuxSessions was superseded by sessionreaper.
- `docs/operations/runtime-reference.md` — updates Readiness gate documentation to list all 13 active checks and removes outdated session warning claims.
- `docs/architecture/sandbox-confinement-ssot.md` — updates item 4 and summary pattern to document fail-closed halt behavior for required sandboxes.

## Design Decisions
Encapsulating memoization inside looppreflight.go:resolve preserves the signature and callers of Run and checkCLIVersionDrift without modifying checks.go. Using atomicwrite.JSON leverages existing tested atomic file primitives without introducing new dependencies.

## Verification
Verified via go/internal/looppreflight unit tests, versioncache_once_test.go counting seam and concurrency assertions, and go/acs/cycle1854 and go/acs/cycle270 predicate suites.

## Compatibility
Options structure and Run return values remain backward-compatible. Cached version format on disk remains unchanged.

## Limitations
Memoization is scoped to a single preflight Run execution and does not persist across separate process invocations beyond the saved cache file.
