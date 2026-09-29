# Build Explanation — Cycle 1759

## Build Binding
- Cycle: 1759
- Base SHA: b401e73d542cbdfd639bf3f44940ed8e9fae3160

## Summary
The three size-ratchet offenders in the fleet-scoped lane — `internal/rollback.Run`, `internal/pruneephemeral.Run`, and `internal/marketplacepoll.Run` — are now each at or under the 50-line ceiling, reached by extracting named helper steps with no behavior change and no comments added.

## Rationale
Each offender was a single orchestrator function doing option validation, seam resolution, the step sequence, and result/ledger assembly inline. Extracting each of those responsibilities into a named helper is the smallest change that satisfies the ratchet: it needs no new abstraction, no new types, and no change to any exported signature, so the existing test suites keep proving the same behavior with no rewrites.

## Changed Areas
- `go/internal/rollback/rollback.go` — `Run` (111→35 lines) now delegates to `newRollbackLogger`, `resolveRollbackSteps`, `runRollbackSteps`, `buildRollbackLedgerEntry`, `writeRollbackLedger`, and `finalizeRollbackResult`; step order, ledger contents, and the MEDIUM-1 overall-success rule are unchanged.
- `go/internal/pruneephemeral/pruneephemeral.go` — `Run` (117→28 lines) now delegates to `resolvePruneDirs`, `defaultDuration`, `newPruneLogger`, `pruneEphemeralDirs`, `pruneDispatchLogs`, and `logPruneSummary`; the TTL cutoffs, dry-run announcements, and quiet-mode summary line are unchanged.
- `go/internal/marketplacepoll/marketplacepoll.go` — `Run` (107→27 lines) now delegates to `validateMarketplacePollOptions`, `resolveMarketplacePollSeams`, `logDryRunPoll`, `validateMarketplaceDir`, `pollForConvergence`, and `refreshInstalledPlugins`; the poll/timeout/refresh ordering and exit-code-mapped sentinel errors are unchanged.
- `go/acs/cycle1759/predicates_test.go` — the cycle's TDD-authored ACS predicates (Run size fit, offenders allowances, unmodified target-package tests, no added comments, explanation accuracy, eval materialization, module-wide ratchet); tracked in this Build's diff so the build handoff and audit grade the same file.
- `.evolve/evals/shrink-rollback-run.md` — TDD-materialized eval for the selected slug `shrink-rollback-run`; tracked in this Build's diff because the scout declared `evals-materialized` without writing it (audit round 1 C1).
- `.evolve/evals/shrink-pruneephemeral-run.md` — TDD-materialized eval for the selected slug `shrink-pruneephemeral-run`; tracked in this Build's diff for the same audit round 1 C1 reason.
- `.evolve/evals/shrink-marketplacepoll-run.md` — TDD-materialized eval for the selected slug `shrink-marketplacepoll-run`; tracked in this Build's diff for the same audit round 1 C1 reason.
- `.evolve/evals/sizeratchet-shrink-run-commands.md` — TDD-materialized eval for the lane's inbox item, grading all ten cycle predicates; tracked in this Build's diff for the same audit round 1 C1 reason.

## Design Decisions
Each extraction follows the same shape already implicit in the code: validate/resolve inputs, run the step(s), assemble/report the result. No shared cross-package helper was introduced — the three packages are independent modules with no common internal dependency, so a shared abstraction would be premature for three call sites. Helper names describe what each step does (`pollForConvergence`, `refreshInstalledPlugins`, `pruneDispatchLogs`) so no comments were needed to explain them.

## Verification
`go test -count=1 ./internal/rollback/... ./internal/pruneephemeral/... ./internal/marketplacepoll/...` passes unmodified. `gofmt -l .` is clean and `go vet ./...` reports nothing. `evolve acs suite --cycle 1759` reports `verdict=PASS green=177 red=0 skip=53 total=230`, covering all ten cycle predicates (size-ratchet fit for all three `Run` functions, unchanged `offenders.json` allowances, unmodified target-package test passes, no comments added, explanation line counts matching `sizeratchet.Walk` at 35/28/27, explanation commit claims matching HEAD, evals materialized for every selected slug, and the module-wide ratchet check). `go run ./cmd/commentaudit comments -base b401e73d go/internal/rollback go/internal/pruneephemeral go/internal/marketplacepoll` reports "no comments added in 3 changed Go file(s)".

## Compatibility
No exported signature, field, or behavior changed in any of the three packages — every helper introduced is unexported and package-private, so callers of `Run` in each package are unaffected.

## Limitations
This lane is scoped to the three named `Run` functions only; `go/internal/sizeratchet/offenders.json` is deliberately left byte-unchanged per the boundary-tighten ADR, so the allowance ceilings still reflect the pre-shrink line counts until a future boundary-tighten cycle lowers them.
