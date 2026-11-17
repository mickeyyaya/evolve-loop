# Option 1 — one package for each verb

Citations: [assumptions-and-evidence.md](../assumptions-and-evidence.md).

## What changes

About 80 verbs each get a package under `internal/cli`, with a shim and a note.

## Causal chain to the goal

Smaller cells let more lanes run. But `cycle` and `loop` share 24 commits (E2), so one item still holds two cells.

## Expected effect

Low relief: most verbs had 0 to 2 touches in 30 days (E1). Score 2.00 on the weighted criteria (A2).

## Cost and time to effect

About 80 shims and 80 notes. Many cycles.

## Top risks

- Each verb needs its own `Deps` (E3). Detect: the outbound counts per verb.
- Note sprawl. Detect: the docs index check.
