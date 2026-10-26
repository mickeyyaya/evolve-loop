# Option 2 — edit the personas now

All numbers cite `assumptions-and-evidence.md`.

## What changes, for whom

Add rules to the builder and tdd-engineer personas for headless `-p` mode, MCP 10 KB returns, and compaction fixes (E3). Tell them the CLI now supports these behaviors.

## Causal chain to the goal metric

New rules steer a phase to the altered behavior, so fewer retries. The chain needs a baseline to show a gain.

## Quantified expected effect

Unknown. No baseline exists because `evolve prompt` is not built (E7), so no gain or loss can be measured (A4).

## Cost and time to effect

Two persona edits, and 10 + 10 cycles of D8 measures for each persona (E6). Two personas in one change also break ADR-0133 D7.

## Top two risks

1. The edit breaks D7 and D8 (E6), so a quality loss has no cause that can be named.
2. The CLI already owns the fixed behavior (E3). The new rule repeats it and adds prompt bytes.
