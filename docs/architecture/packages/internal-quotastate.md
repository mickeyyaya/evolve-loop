# internal/quotastate

## Purpose

`internal/quotastate` parses a CLI's captured usage pane into a structured snapshot: for each quota window, the used fraction and the reset time. It is the measurement layer of the quota-driven dynamic budgeting design.

## Design

- **Numbers, not a boolean.** The usage probe already sends the usage command and classifies the result as capped or not. `quotastate` keeps the numbers. The budget allocator sizes a fleet wave against real headroom and time to reset, not a static `min_lanes` assertion.
- **Native units only.** Budgets use each CLI's own units: the remaining fraction and the reset time. Dollars are not used. The earlier dollar-cost budget was removed because subscription Claude reports $0.
- **Source says how far to trust the numbers.** `SourceProbed` means real numbers were parsed. `SourceUnknown` means none were, because the CLI has no usage command or the pane did not match. A budget reads an unknown source with the floor fallback.
- **The binding window sizes the wave.** `TightestRemaining` returns the smallest remaining fraction across the matching windows. A 34%-remaining weekly cap dominates a 73%-remaining session when both apply.
- **Reset times are parsed in local time.** `ParseResetWhen` accepts the two forms Claude's `/usage` emits: `Jul 5 at 9pm` (an absolute date) and `4:10pm` (the next occurrence of that clock time).

## Invariants

- **A missing reset is not a fabricated one.** When `ParseResetWhen` cannot match, the caller leaves `ResetAt` at the zero time.
- **A date more than a day in the past rolls to the next year.** This handles a December-to-January reset; a recent date stays in the current year.
- **A clock time that has already passed today means tomorrow.** `TestParseResetWhen` pins both forms.
- **The July Claude golden keeps its buckets.** `TestReadWindows_TheJulyClaudeGoldenKeepsItsBucketsThroughStatesOf` pins the parsed session and week windows against `testdata/claude_usage.txt`.

## Findings

- None.
