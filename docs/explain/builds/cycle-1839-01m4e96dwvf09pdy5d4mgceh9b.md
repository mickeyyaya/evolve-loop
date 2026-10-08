# Build Explanation — Cycle 1839

## Build Binding
- Cycle: 1839
- Base SHA: 017d3e817ef149b8c847df32c4edb2b4de5e28c3

## Summary
Rollback operations now fail closed on remote tag lookup and release view failures, execute the default gh step within RepoRoot, and avoid vacuous test assertions.

## Rationale
Ignoring lookup errors and exit codes treated network, authentication, and execution failures as missing resources, permitting partial failures to report success and leave dangling remote resources. Enforcing fail-closed behavior ensures operations abort reliably on error.

## Changed Areas
- `go/internal/rollback/rollback.go` — checks git ls-remote exit code and error to return failed on lookup errors, checks gh release view output to report not-present only for 404/not found while reporting failed for all other errors, and passes RepoRoot to gh execution.

## Design Decisions
The defaultGhDeleteRelease function delegates to an unexported repoRoot-aware helper defaultGhDeleteReleaseIn while keeping the exported signature callable for existing callers. The Steps.GhDeleteRelease fallback closes over opts.RepoRoot during step resolution.

## Verification
ACS predicates TestC1839_001 through TestC1839_004, failclosed test suite, unit tests, and integration tests verify lookup error classification, directory isolation, and preservation.

## Compatibility
Existing function signatures and options fields remain backward compatible.

## Limitations
This change does not modify concurrent ledger write semantics.
