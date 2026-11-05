# Build Explanation — Cycle 2178

## Build Binding
- Cycle: 2178
- Base SHA: ca70c89cc53c2c2319c532f3d1d377065d35348c

## Summary
The agy conversation query now compares the stored input time as an instant to the microsecond. A conversation that sits at the window end or start is now counted.

## Rationale
The store keeps `last_user_input_time` as text with a zone suffix and trimmed trailing zeros. The old query compared that text with the window bounds. The text comparison dropped rows that equal a bound. The query now builds a canonical key of seconds in UTC plus a six-digit fraction, and it compares the key.

## Changed Areas
- `go/internal/tokenusage/agy.go` — the query normalizes the stored time before it compares it with the bounds.
- `go/internal/tokenusage/agy_test.go` — the query-text assertions name the new `input_time` column.
- `go/acs/cycle2178/predicates_test.go` — the predicates of the TDD phase pin the edges.
- `.evolve/evals/agy-calls-record-no-tokens.md` — the eval of the task.

## Design Decisions
The fix stays in SQL, so the caller and the exported API do not change. A filter in Go needs a second query column and a changed fake.

## Verification
The five cycle predicates and the bridge and tokenusage packages pass.

## Compatibility
No exported symbol changes. The manifest regex is unchanged.

## Limitations
Acceptance 2 needs a live wave and is deferred by triage.
