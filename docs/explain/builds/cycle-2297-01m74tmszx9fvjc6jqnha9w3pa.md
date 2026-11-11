# Build Explanation — Cycle 2297

## Build Binding
- Cycle: 2297
- Base SHA: 54097188354078bcf0dcb5caea6eac3ad3a95246

## Summary
Each usage record of `evolve code` now names the backend that answered the call. `evolve code usage` groups the records by backend. This builds component G5 of the code index backends plan. This cycle continues cycle 2282, which built the same field and failed its audit on an unrelated suite abort.

## Rationale
The decision to keep or replace a backend rests on this log. A call that the router sends to `gotypes` and a call that it sends to `trail` must show as separate rows. The name comes from the backend that served the query, so it follows the route and never a constant.

## Changed Areas
- `go/internal/codeindex/backends.go` — `SearchServed` returns the hits and the name of the serving backend. `Search` now calls it.
- `go/internal/codeindex/usagelog/usagelog.go` — `Record` and `Row` have a `Backend` field. `Summarize` groups and sorts by it. The record field is `omitempty`, so an old line still reads.
- `go/cmd/evolve/cmd_code.go` — the code verb uses `SearchServed` and stores the name in the call.
- `go/cmd/evolve/cmd_code_usage.go` — the log record carries the backend. The usage table has a `BACKEND` column and shows an empty backend as `unknown`.
- `go/internal/codeindex/backends_served_test.go` — tests for `SearchServed`.
- `go/internal/codeindex/usagelog/usagelog_test.go` — tests for the field, the legacy line and the grouping.
- `go/cmd/evolve/cmd_code_usage_backend_test.go` — tests for the logged backend and the usage table.
- `go/cmd/evolve/cmd_code_usage_test.go` — the expected table has the new column.
- `go/acs/cycle2297/predicates_test.go` — the predicates of the cycle.
- `.evolve/evals/code-index-backends-g5-usage-log-backend.md` — the eval graders of the task.
- `docs/architecture/packages/internal-codeindex-usagelog.md` — the package page records the backend field.

## Design Decisions
- A call that no backend serves, and a usage error, record an empty backend. The table shows it as `unknown`.
- A backend that fails or is absent is still named, because it answered.
- Rejected: a separate log file for backends. The record already holds the call, so one more field is the smallest change.

## Verification
The predicates `TestC2297_001` to `TestC2297_006` and the package tests pass. The tests run the real route with fake backends.

## Compatibility
An old record has no `backend` key. It reads with an empty backend. The JSON rows of `evolve code usage` gain a `backend` key.

## Limitations
A call that the stale gate refuses records no backend, because no backend ran.
