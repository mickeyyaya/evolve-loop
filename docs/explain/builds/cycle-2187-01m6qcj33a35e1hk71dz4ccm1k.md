# Build Explanation — Cycle 2187

## Build Binding
- Cycle: 2187
- Base SHA: a5f971c2d806906c3af71e3deb3c0ba2393f8a67

## Summary
A new lookup, FindConsumedRecord, returns the one consumed record for an inbox item id. It refuses a second record for the same id.

## Rationale
Two consumed records with one id made any first-match lookup pick one record silently, by name order, because the directory read sorts by name. The lookup now fails with an error that names every matching path. This follows the inbox item acceptance.

## Changed Areas
- `go/internal/inboxmover/consumed_lookup.go` — adds FindConsumedRecord. It walks the consumed directory and its cycle directories, and compares the id of each file with itemIDAt.
- `go/internal/inboxmover/consumed_lookup_test.go` — covers the duplicate, single, none, noise and missing-directory cases.
- `go/internal/inboxmover/apicover_named_test.go` — names the new export for the apicover gate.

## Design Decisions
The function reuses itemIDAt and ErrNotFound. It adds no new error type. A caller can still use errors.Is with ErrNotFound for the none case.

## Verification
The inboxmover package tests and the four cycle2187 predicates pass. The ACS suite reports red=0.

## Compatibility
No existing function changes. The not-found error is the same value as before.

## Limitations
The lookup does not count an unreadable or malformed record as a match. An unreadable duplicate stays hidden. The lookup can then return a single match when two records exist. It can also return ErrNotFound when the only record is unreadable. No production caller uses the function yet. The Glob lookup in the core task contract is a separate change.
