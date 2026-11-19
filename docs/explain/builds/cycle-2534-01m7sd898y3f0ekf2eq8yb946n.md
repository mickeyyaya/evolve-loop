# Build Explanation — Cycle 2534

## Build Binding
- Cycle: 2534
- Base SHA: 234383500cc7a7a113a70a9889b950ed247c6a6b

## Summary
Failure-class console escalations now carry title, per-occurrence summary lines, and acceptance criteria, update open items on higher recurrence, and pass `evolve inbox lint`.

## Rationale
Prior failure-class escalations staged only `pattern`, `cycle`, `recurrence`, and `reason`. The boundary autofiler wrote items with no `title`, `summary`, or `acceptance`, and skipped repeat occurrences. Extending `Intent` and `PreventiveAction` keeps occurrence facts in `.evolve/failure-digests/` and updates the open item in place.

## Changed Areas
- `go/internal/dispositionrouter/dispositionrouter.go` — adds `Title`, `Summary`, and `Acceptance` fields to `Intent` so staged autofiles carry actionable item text.
- `go/internal/core/failure_class.go` — gathers prior class digests with `priorClassOccurrences` and fills `Title`, `Summary` lines, and `Acceptance` in `stageClassEscalation`.
- `go/internal/retrofile/retrofile.go` — writes `title`, `summary`, and `acceptance`, and updates an open inbox file when `Recurrence` rises.
- `go/internal/recurrence/apply.go` — passes `Title`, `Summary`, and `Acceptance` through `autofileIntent` and uses a pre-loop stamp snapshot so later intents apply.
- `go/internal/cli/inboxcmd/cmd_inbox.go` — adds `lintMissingFields` to `runInboxLint` to warn on absent `title`, `summary`, or `acceptance` with exit 0.

## Design Decisions
Occurrence details come from `.evolve/failure-digests/` rather than new state files. `ApplyBoundary` checks a clone of `applied-stamp.json` from before the loop. Newer intents for the same item then run on a new cycle, and same-cycle re-runs stay idempotent. `retrofile.FileActions` updates only open root items when `a.Recurrence > prev.recurrence`, which leaves processed and claimed items untouched. The in-place update rewrites the full open item from `inboxItem` and overwrites fields changed after creation, such as operator edits, `weight`, and `injected_at`.

## Verification
Targeted tests in `dispositionrouter`, `retrofile`, `recurrence`, `inboxcmd`, and `core` pass, and all 4 cycle-2534 ACS predicates pass. In `evolve acs suite --cycle 2534`, 246 checks pass and the 2 timed-out scopes (`acs/regression/protectedsurface` and `acs/redteam`) pass on direct rerun.

## Compatibility
`ContractGateDemotion.Intent()` and legacy classless intents still file through `autofileIntent` with the fallback title format. `evolve inbox lint` keeps exit code 0 when it prints missing-field warnings on `stderr`.

## Limitations
`lintMissingFields` checks presence of `title`, `summary`, and `acceptance` on pending items and does not fail the lint command.
