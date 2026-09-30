# Comment history: `acs/cycle1300`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1300/predicates_test.go:3` — above `package cycle1300`

```text
// Package cycle1300 materialises the cycle-1300 acceptance criteria for the two
// fleet-scoped tasks pinned to this lane (inbox contract-block-cli-escalation):
//
//   - breaker-neutral-salvage-retry-when-no-escalation-family-exists
//   - demotion-ledger-records-salvage-attempted-vs-no-remedy-possible
//
// Predicate strategy. The contract this cycle adds lives entirely on unexported
// seams of internal/core (contractSalvageRetryDirectiveHeading, contractDispatch,
// formatContractGateDemotionWarn) reached only through Orchestrator.RunCycle, so
// the predicates DRIVE the RED contract tests as a subprocess rather than
// grepping source: each named test builds a real .evolve/profiles/*.json on
// disk, runs a real cycle, and asserts on the directives/ledger the production
// ladder actually emitted. A no-op implementation cannot pass them — the
// negative tests (no heading when an escalation target exists, none on the first
// block, none on a Blocks==0 rejection) fail a blanket "always re-prompt" too.
//
// Flaky-shape compliance: ONE named package, always narrowed with an anchored
// -run so the 40s+ whole-core suite never runs; no wall-clock bounds, no literal
// PIDs, no un-reaped load generators; `go test -C <worktree>/go` pins the working
// directory so a fleet lane's cwd cannot change which tree is measured.
```
