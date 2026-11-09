# Build Explanation — Cycle 2241

## Build Binding
- Cycle: 2241
- Base SHA: 7452aeec4c530e85ff2f9461976a5f638de752a9

## Summary
The package provides safe Git revision and SHA helpers. Git receives `--end-of-options` before ref arguments.

## Rationale
Direct argument pass-through can misinterpret a dash-prefixed ref as an option flag. Validation rejects malformed SHAs before process spawn.

## Changed Areas
- `docs/architecture/packages/pkg-acsassert.md` — documents the design and invariants of the new helpers.
- `go/internal/integration/ghci_test.go` — adjusts helper iteration to avoid test helper duplication.
- `go/pkg/acsassert/assertions.go` — implements ValidateSHA, GitRefOutput, and GitSHAOutput with option protection.
- `go/pkg/acsassert/assertions_test.go` — tests all branches of the new functions to reach complete coverage.

## Design Decisions
The implementation places `--end-of-options` before the ref argument. A regular expression verifies that SHAs contain only 7 to 64 hexadecimal characters.

## Verification
Unit tests verify both valid and invalid arguments. ACS suite cycle predicates pass with full statement coverage.

## Compatibility
All prior assertion helpers remain unchanged. New functions export dedicated revision entry points.

## Limitations
The helpers do not sanitize arbitrary custom Git subcommands beyond ref argument protection.
