# Build Explanation — Cycle 2507

## Build Binding
- Cycle: 2507
- Base SHA: 5ed6d767432bb82a339e95152dc5832413118bdf

## Summary
Six Go message strings now follow ASD-STE100. The strict Go lint reports no finding in the packages campaign, codequality, fleet, gc, inboxbatch and inboxcurator.

## Rationale
The lint listed six findings, one in each package. Each fix changes the smallest part of one literal. No key=value field, signal code or parser sentinel changes.

## Changed Areas
- `go/internal/campaign/executor.go` — "attempt(s)" becomes "tries" in the wave failure error.
- `go/internal/codequality/gofmt.go` — "could not read the tree" becomes "cannot read the tree".
- `go/internal/fleet/preflight.go` — "via" becomes "with" in the control-plane preflight error.
- `go/internal/gc/devworktrees_apply.go` — "could not be pinned" becomes "cannot be pinned".
- `go/internal/gc/devdetached_cover_test.go` — the pin test follows the new gc wording in the same diff.
- `go/internal/inboxbatch/priority_class.go` — "would rank" becomes "ranks" in the ErrNoPriorityClass text.
- `go/internal/inboxcurator/intake.go` — a 27-word sentence becomes two sentences of 25 words or fewer.

## Design Decisions
Only string literals change. The sentinels keep their identity, so `errors.Is` callers see no difference.

## Verification
The strict Go lint reports 0 findings for the six packages. The package tests and the cycle 2507 predicates pass.

## Compatibility
Tokens that readers use stay: `priority_class`, `--reason`, `evolve ship --class manual` and `gofmt -l -s <dir>`.

## Limitations
This batch covers six packages only. Other packages need later batches.
