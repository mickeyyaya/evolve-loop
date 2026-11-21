# Build Explanation — Cycle 2588

## Build Binding
- Cycle: 2588
- Base SHA: 23c32f42926c3dcde2c2da08401c897afd66275a

## Summary
The two loop producers of inbox items now declare a fix surface. Inbox lint accepts both items.

## Rationale
Both items had no `files` list and no console route. Each boundary inbox PR failed lint. A declared `files` list is the smallest fix and keeps lint strict.

## Changed Areas
- `go/cmd/evolve/cmd_wave_latency_check.go` — the latency breach item now lists the lane time check and the command file as its files.
- `go/internal/practicewatch/item.go` — the practice item lists the model notes file of its family, or `agents/` when the family has no notes.

## Design Decisions
No lint rule changes. A new field on each item struct carries the surface.

## Verification
The cycle 2588 predicates cover both producers, the lint exit code and the refusal of an item with no surface.

## Compatibility
The item schema gains only the existing `files` key.

## Limitations
The listed files name the likely fix surface, not a proven one.
