# Build Explanation — Cycle 1769

## Build Binding
- Cycle: 1769
- Base SHA: fe0f8f203ba2a3fb1cfcd11b3b1ca75bd18032ec

## Summary
`installer.Validate` (58 lines) and `ciwatch.Watch` (64 lines) were the two live offenders in `go/internal/sizeratchet/offenders.json` for these packages. Both now fit the repo-wide 50-line function-size ratchet: `Validate` is 40 lines and `Watch` is 36. Each got there by moving named steps into unexported helpers. Behavior is unchanged: the existing package suites pass, and the eleven cycle-1769 characterization predicates are green. `offenders.json` was not touched.

## Rationale
Both functions were a series of independent steps written out inline, so the natural split points were already there. In `Validate`, the skill-file loop and the reference-doc loop each read only `srcDir`, add to `res.Errors`, and print to `out`. They now match the calling shape of the step helpers that already existed (`validateMarketplace`, `validateAgentFrontmatter`). In `Watch`, filling in the default seams (`Now`, `Sleep`, `Timeout`, `Poll`) and the poll-until-complete loop are two separate jobs, each with a single result.

One alternative was to extract only the input guards from `Watch`. That was rejected because it saves just 9 lines and leaves `Watch` above the limit. Another was to raise the allowance in `offenders.json`. That was rejected because the allowance is a ceiling, and this lane's scope explicitly leaves the file alone.

## Changed Areas
- `go/internal/installer/installer.go` — the skill-file and reference-doc existence loops in `Validate` moved into the new `validateSkillFiles` and `validateReferenceDocs` helpers, which `Validate` calls in the same order. The OK/FAIL transcript and the counters are unchanged. The two step comments that described the moved loops were deleted, not re-added.
- `go/internal/ciwatch/ciwatch.go` — the default resolution in `Watch` moved into the new `resolveWatchDefaults`, which returns a copy of `Options` with the defaults filled in (900s timeout, 30s poll, real clock/sleep). The fetch/deadline loop moved into the new `pollUntilComplete`. `Watch` keeps its guard order, fetch-error short-circuit, timeout error text, verdict write and escalation.
- `go/acs/cycle1769/helpers_test.go` — TDD-phase deliverable (ACS predicate helper) that already existed before this build; this build did not change it.
- `go/acs/cycle1769/predicates_test.go` — TDD-phase deliverable (ACS predicates) that already existed before this build; this build did not change it.
- `.evolve/evals/sizeratchet-shrink-installer-ciwatch.md` — eval graders for this task that already existed before this build; this build did not change them.

## Design Decisions
`resolveWatchDefaults` takes `Options` by value and returns a new value, so the caller's struct is never mutated. `Watch` then reads `opts.Now`, not a local alias. `fileEscalation` now receives the resolved `Options`, but it does not read `Now`, `Sleep`, `Timeout` or `Poll`, so its output is identical.

`pollUntilComplete` calls the clock and the fetcher in the same order as before: one `Now()` for the deadline, then fetch → completed? → `Now()+Poll` deadline check → sleep. Clock-counting seams therefore see the same sequence.

All new identifiers are unexported. There are no new exports, flags or struct fields.

## Verification
- `gofmt -l` and `go vet` report nothing for both packages.
- `go test -count=1 ./internal/ciwatch ./internal/installer ./internal/sizeratchet` passes.
- `go test -tags acs -count=1 ./acs/cycle1769` passes 11/11 predicates. These cover the size limit, `offenders.json` being byte-unchanged, module-wide `sizeratchet.Check`, frozen baseline test declarations, the package suites, the Watch fetch-error/defaults/guard-order/real-clock pins, the golden Validate transcript, and no added comment lines.

## Compatibility
No exported API, CLI flag, config or artifact format changed. `offenders.json` keeps its allowances of Validate=58 and Watch=64. The lines saved are left as slack for a later boundary-tighten cycle.

## Limitations
This build does not tighten the `offenders.json` allowances. It also does not shrink ratchet offenders outside `go/internal/installer` and `go/internal/ciwatch`.
