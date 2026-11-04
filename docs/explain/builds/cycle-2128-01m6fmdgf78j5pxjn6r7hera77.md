# Build Explanation — Cycle 2128

## Build Binding
- Cycle: 2128
- Base SHA: ca2f3ed59f6455db6a07ceb4f37ba4b448dfa451

## Summary
The Claude family model filter now recognises the fable alias. This change unblocks within-tier failover for floored seats.

## Rationale
The bridge floor dropped fable from the failover chain because familyModelWords lacked the token. Adding fable aligns bridge failover with modelquery family definitions.

## Changed Areas
- `go/internal/bridge/model_failover.go` — includes fable in the Claude family model words to preserve fable in fallback chains.

## Design Decisions
The family definition in bridge matches modelquery. The floor keeps valid Claude aliases while filtering models from other families.

## Verification
Unit tests in bridge and modelcatalog verify chain construction and failover. ACS cycle 2128 predicates pass with zero errors.

## Compatibility
The change is backward-compatible. Existing Claude family models remain recognised.

## Limitations
This change only updates within-tier model failover for the Claude family.
