# Build Explanation — Cycle 1804

## Build Binding
- Cycle: 1804
- Base SHA: 8185a58904a435488532095114c971d59b53e993

## Summary
Three inboxbatch defects are fixed: rune-splitting truncation, last-wins duplicate ids, and untokenized files[] entries. Discarded inbox load warnings are not fixed.

## Rationale
Each fix is the smallest local change: back up to a rune boundary, guard the index insert, reduce an entry to its path token.

## Changed Areas
- `go/internal/inboxbatch/item.go` — truncation backs up to a UTF-8 rune start so fields stay valid.
- `go/internal/inboxbatch/rules.go` — indexByID keeps the first holder of an id; fileArea and the new pathToken strip wrapping punctuation and :line suffixes.
- `go/internal/inboxbatch/archetype.go` — IsOperatorState tokenizes each entry before the .evolve/ prefix check.
- `go/internal/loopwave/dispatch.go` — the plan-time routing gate now prints a WARN naming each inbox file that failed to load, once per resolver build.

## Design Decisions
The inbox load warnings are printed by the gate in dispatch.go using the existing LoadDir warnings, so no new seam was added; a healthy inbox stays silent.

## Verification
ACS predicates TestC1804_001-007 and 009 and the inboxbatch and loopwave suites pass; TestC1804_008 stays red.

## Compatibility
Public signatures are unchanged; no export is added.

## Limitations
Entries with a path containing spaces are tokenized at the first space.
