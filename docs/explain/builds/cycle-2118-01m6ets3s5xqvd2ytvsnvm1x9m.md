# Build Explanation — Cycle 2118

## Build Binding
- Cycle: 2118
- Base SHA: 2003532014179809b458f854db8597d209c04684

## Summary
This cycle secures the inbox consume contract. It prevents quoted text from closing inbox tasks, single-sources the `ConsumedRecord` Go type, and updates the consumer to use this type.

## Rationale
Quoted text in markdown reports previously matched the closure marker parser. An upstream finding in a code fence or bullet list can close an item that never shipped. The consumed record format also lacked a single Go type.

## Changed Areas
- `go/internal/inboxmover/closesmarker.go` — tracks markdown fences and enforces column 0 for closure claims.
- `go/internal/inboxmover/consumed_record.go` — introduces `ConsumedRecord` and `ParseConsumedRecord` to parse item consumed keys.
- `go/internal/inboxmover/continuation_release.go` — updates `retiredAtCycle` to call `ParseConsumedRecord`.

## Design Decisions
The parser tracks fences across lines and requires column 0 for markers. An alternative was regex matching across the entire body, but line iteration is simpler and handles unclosed fences. `ConsumedRecord` normalizes cycle numbers into decimal text while keeping string labels.

## Verification
ACS predicates for cycle 2118 and cycle 1452 verify fence rejection, column 0 enforcement, and corpus round trips. Unit tests cover all shape permutations and production call sites.

## Compatibility
The `Closes-Inbox:` convention remains identical for genuine column-0 claims. Existing consumed items in `.evolve/inbox/consumed/` parse without schema drift.

## Limitations
The closure marker parser handles markdown backtick and tilde fences. It does not parse full CommonMark ASTs.
