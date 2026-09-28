# Build Explanation — Cycle 1738

## Build Binding
- Cycle: 1738
- Base SHA: c9093d5c9cf9b92a80ea8aeac2e7bb68fc98bcbc

## Summary
Six oversized functions now fit the 50-line `sizeratchet.MaxLines` limit. In `internal/dashboard` they are `callForPhaseWindow` (67 → 29 lines), `collector.collect` (54 → 42) and `readPlan` (59 → 48). In `internal/deliverable` they are `Reviewer.Review` (68 → 43), `SalvageSummaryLine` (58 → 33) and `verdictCandidates` (64 → 33). Each change is an extract-method refactor that keeps behavior. New characterization tests pin all 29 behavior mutants that the baseline suites let survive. `offenders.json` is not edited: since fix/sizeratchet-is-a-ceiling an allowance is a ceiling, so the six entries stay as slack for a later boundary tighten.

## Rationale
Extract-method is the smallest change that gets each function under the limit. The moved lines keep their statements, variable names and comments verbatim. That keeps every mutation anchor of the cycle-1738 predicates compilable at its new location, and it moves each baseline comment with its code instead of deleting it. Leaving `offenders.json` unchanged avoids a rebase conflict with the sibling shrink lane, whose keys sit on adjacent lines.

## Changed Areas
- `go/internal/dashboard/cycle.go` — `callForPhaseWindow` hands the windowed scan (latest unused call whose terminal and start times fall in the window, with the two whole-second overlap withholding rules) to `latestCallInWindow`. The loop and its comments are moved verbatim. The only reshaped line is the return: `if !selectedAt.IsZero() { return … }` became `return selected, selectedIndex, !selectedAt.IsZero()`. On a miss this yields the same `llmCall{}, -1, false` as before, because `selected` is zero and `selectedIndex` is -1.
- `go/internal/dashboard/collect.go` — `collector.collect` hands cycle-id selection (history selection, adding every running lane that history hid, newest-first sort) to `collector.renderedCycleIDs`, which returns the same `(ids, warn)` pair. The "History limits may never hide a live lane" comment moves with its loop.
- `go/internal/dashboard/plan.go` — `readPlan` hands the walk-frontier fold to `walkFrontier` and the mandatory-phase scheduling to `scheduleMandatory`, which returns `(order, required)`. The frontier's trailing comment moves with it. The order of evaluation is unchanged: the frontier is still computed before the mandatory phases are appended.
- `go/internal/deliverable/reviewer.go` — `Reviewer.Review` hands the violation outcome (size-gate warn, below-enforce would-block, breaker increment, demote-or-block) to `Reviewer.violationResult`. That method takes the same `check`, `in`, `res` and breaker path `bp`, so its body is moved verbatim, comments included.
- `go/internal/deliverable/salvage_extract.go` — `verdictCandidates` had two identical copies of the brace-depth switch, one in brace-only mode and one in string-aware mode. Both now call `balanceBrace`. It takes and returns `depth`, `start` and `out` by value and keeps the anchor expressions verbatim. The string-aware `switch ch` keeps `case '"'` and sends every other byte to `balanceBrace` through `default`. `balanceBrace` ignores bytes other than braces, as the old switch did. The old `continue` inside `case '}'` becomes `return depth, start, out`, which has the same effect because the switch was the last statement of the loop body. `SalvageSummaryLine` hands the streamed sidecar read (skip torn records, drop a partial read) to `readSalvageApplied`, which reports `ok=false` on an open error or scan error, where the old code returned `""`. The local `appliedRec` type is now package-level so the two functions can share it. The "Streamed, not slurped", torn-record and unread-tail comments move with their code.
- `go/internal/dashboard/anchored_behavior_test.go` — new characterization tests for `callForPhaseWindow` (used call not rematched, call after the window or started before it not matched, occurrence fallback index and reuse), `Collect` (the loop adopts its own lane state, the newest running lane is the loop, the brake reaches stateless rows, a fleet loop gets its dispatch) and `readPlan` (warnings carry the `cycle N` prefix, a mandatory phase outside the walk order is unreached rather than skipped).
- `go/internal/deliverable/anchored_behavior_test.go` — new characterization tests for `Reviewer.Review` (an unset breaker path uses the evolve dir's breaker file, a block carries the breaker count), `SalvageSummaryLine` (exact breakdown, order and separator; torn record skipped; a partial read omits the line; empty at zero; a forged pattern becomes `unknown`) and `verdictCandidates` (exact spans in both modes, escapes, a key-less object ignored, unterminated objects marked open).
- `go/acs/cycle1738/predicates_test.go` — the TDD phase's acceptance predicates for this lane (not edited by Build).
- `.evolve/evals/sizeratchet-shrink-dashboard-deliverable.md` — the TDD phase's eval contract for this lane (not edited by Build).

## Design Decisions
Each helper stays in its origin file, next to its caller, so comment moves stay within one file. None of the new helpers is exported, and none has a doc comment, because the package comment contract forbids adding comment lines. The characterization tests are adapted from the TDD phase's overlay kill probes, renamed after the function each one pins, and put in new files because the baseline `_test.go` files are frozen. The files are named `anchored_behavior_test.go` so that they sort before every baseline test file. `go test` runs tests in filename order, so under `-failfast` each mutant run stops at the characterization test that kills it, and does not first run the ~31s baseline suite. That brings the deliverable mutant predicate from ~130s to ~39s, and the whole ACS package fits the build floor's `-timeout 120s`.

## Verification
- All 12 cycle-1738 ACS predicates pass (`go test -tags acs -count=1 -timeout 120s ./acs/cycle1738`, 81s). They cover the size limit, `offenders.json` unchanged, a green module-wide ratchet, frozen baseline tests, both package suites, vet, no comments added or deleted, target docs identical to baseline, and 11/11 dashboard and 18/18 deliverable mutants killed.
- `gofmt -l .` prints nothing and `go vet ./...` exits 0.

## Compatibility
No exported signature, output string, breaker file path or review result changes.

## Limitations
The six `offenders.json` allowances remain until a boundary tighten removes them. The deliverable test suite still writes a gitignored `go/internal/deliverable/.evolve/ledger.jsonl`. That test-hygiene issue predates this change and is out of scope.
