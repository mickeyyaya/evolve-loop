# Option 3 — answer in code and config, not in a persona

All numbers cite `assumptions-and-evidence.md`.

## What changes, for whom

File one inbox item for a deterministic check in `evolve cli update`. After an update, the smoke test reads the new changelog section for the env-var names the loop sets. Today the loop sets neither of the two env vars named in E2 (E5). The check finds nothing to guard.

## Causal chain to the goal metric

A code check catches a break that a persona cannot, with no prompt bytes. The chain is weak here: with no env var in use (E5), no break can occur.

## Quantified expected effect

0 breaks caught now (E5). A future gain only if the loop starts to set one of these variables.

## Cost and time to effect

One build cycle for the check, plus a test, and it covers no live risk (E5, E9).

## Top two risks

1. The check is dead code until a variable is in use, which AGENTS.md invariant on caller proof forbids.
2. The match on names produces false alarms on unrelated lines (A4).
