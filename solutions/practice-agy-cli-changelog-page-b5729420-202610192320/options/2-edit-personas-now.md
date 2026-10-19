# Option 2 — edit the personas now

All numbers cite `assumptions-and-evidence.md`.

## What changes, for whom

Add a rule to the builder and scout personas for the run-behavior fixes of E5. Tell them about stall retry and the `/compact` command.

## Causal chain to the goal metric

New rules steer a phase to the fixed behavior, so fewer retries. The chain needs a baseline to show a gain.

## Quantified expected effect

Unknown. No baseline exists (E9), so no gain or loss can be measured (A3).

## Cost and time to effect

One persona edit plus 10 + 10 cycles of D8 measures for each persona (E8). With two personas, ADR-0133 D7 is also broken: one persona per change.

## Top two risks

1. The edit breaks D7 and D8 (E8), so a quality loss has no cause that can be named.
2. The CLI already owns the fixed behavior (E5). The new rule repeats it and adds prompt bytes.
