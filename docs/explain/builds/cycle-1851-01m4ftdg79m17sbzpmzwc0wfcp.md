# Build Explanation — Cycle 1851

## Build Binding
- Cycle: 1851
- Base SHA: f8fdcdd565c87ac4972b14419eb7c27333182467

## Summary
`phasecoherence.CheckProvenance` now scans ledger lines up to 64 MiB, so a large record no longer ends the cross-check. It counts the lines that are not JSON and reports a non-zero count as one `WARN` of kind `malformed-ledger`. `canonicalRole` folds the role to lowercase before the alias lookup, so a ledger role `Build` matches the phase `build`. The package has one severity vocabulary, the constants `SeverityWarn` (`WARN`) and `SeverityError` (`ERROR`). `Check` and `CheckArtifactNames` share one persona walk. The cycle-1348 incident record no longer claims an `inbox-parked` carve-out that does not exist.

## Rationale
The inbox item found that a ledger line over 64 KiB stopped the scan, that malformed lines were skipped without a count, and that `Build` and `build` did not match. Before this cycle the oversize line already surfaced as an error, but that error aborted the whole check, so no entry after the line was compared. A larger scanner buffer is the smallest change that makes the scan complete; a line past the new limit still returns an error naming the ledger. The malformed count is a `WARN` and not an error, because one bad line must not hide the cross-check of the good lines.

## Changed Areas
- `go/internal/phasecoherence/provenance.go` — `scanLedger` sets the scanner buffer to `maxLedgerLineBytes`, skips blank lines and counts malformed lines; `checkLedgerTreeSHA` adds the `malformed-ledger` WARN; `canonicalRole` lowercases first.
- `go/internal/phasecoherence/coherence.go` — adds `SeverityWarn` and `SeverityError`; extracts `walkPersonas`, `personaName` and `personaFrontmatter`, the one persona iteration, profile lookup and frontmatter parse; `Check` uses them.
- `go/internal/phasecoherence/artifact_coherence.go` — `CheckArtifactNames` uses the shared walk and `personaFrontmatter` instead of its own copy.
- `go/internal/cli/phasecmd/phases_provenance.go` — the exit-1 test compares with `phasecoherence.SeverityError` instead of the literal `"error"`.
- `go/internal/phasecoherence/provenance_test.go` — the oversize-line error case now writes a line past `maxLedgerLineBytes`; the severity asserts use the constants; a new table test covers oversize, blank and malformed lines.
- `go/internal/phasecoherence/provenance_amplification_test.go` — the severity assert uses `SeverityError`.
- `go/internal/phasecoherence/canonical_role_amplification_test.go` — the case-variant rows now expect the alias (`Build` gives `builder`); these rows pinned the defect.
- `go/internal/phasecoherence/test_amplification_test.go` — same correction for `BUILD`.
- `docs/architecture/packages/internal-phasecoherence.md` — documents the shared walk, the case fold, the 64 MiB limit, the malformed WARN and the severity constants.
- `docs/architecture/packages/internal-cli-phasecmd.md` — the `check-provenance` exit codes name `ERROR`.
- `docs/operations/runtime-reference.md` — the `check-provenance` entry names `ERROR` and the 64 MiB limit.
- `docs/incidents/cycle-1348-evals-birth-ignored-batch-halt.md` — states that `.evolve/inbox-parked/` is allowlisted in the class pin, not carved out of the ladder.

## Design Decisions
The persona walk is a function with a two-callback visitor (`unpaired`, `paired`). `Check` needs the unpaired callback and `CheckArtifactNames` does not, so the visitor leaves it optional. A Template Method type or an iterator would add a type for two callers. `Check` keeps its rule that an empty `allowed_tools` skips the persona read, so a profile without tools still does not read its persona. The severity value for a mismatch changes from `error` to `ERROR` to match `WARN`; the only production consumer compares through the constant.

## Verification
`go test -count=1 ./internal/phasecoherence/ ./internal/cli/phasecmd/` passes, and the six cycle-1851 ACS predicates pass (`go test -tags acs -count=1 ./acs/cycle1851`).

## Compatibility
`evolve phases check-provenance` prints `ERROR:` instead of `error:` for a mismatch, and its JSON `severity` field changes the same way. Its exit codes are unchanged. A ledger with a line between 64 KiB and 64 MiB now passes the scan instead of exiting 2.

## Limitations
A ledger line over 64 MiB still returns an error. The malformed WARN gives a count, not the line numbers.
