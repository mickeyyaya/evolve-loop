# Option 1 — One review note in the agy model notes

Citations: [assumptions-and-evidence.md](../assumptions-and-evidence.md).

## What changes

One bullet is added to `skills/model-notes-agy/COMPACT.md`. It cites the source and records the two CLI-reachable URLs as data. It states that no persona changes under ADR-0133. It keeps the Docs MCP server unused (A1, A2, A3, E1, E2).

## Causal chain to the goal

The note makes the review durable. The next agy dispatch loads the note and sees the verdict. No persona text changes.

## Expected effect

One line added, no behavior change. The note meets both acceptance criteria (E3).

## Cost and time to effect

One file, one cycle. The effect is immediate (E3).

## Risks and early detection

1. The notes file grows one bullet for each review. Detection: the bullet count in the lint check.
2. A later guide change is missed. Detection: the next practice-watch item names it.
