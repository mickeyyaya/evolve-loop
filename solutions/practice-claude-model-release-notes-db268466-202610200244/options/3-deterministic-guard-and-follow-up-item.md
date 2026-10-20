# Option 3 — answer in code and config, not in a persona

All numbers cite `assumptions-and-evidence.md`.

## What changes, for whom

File one inbox item for a check in `evolve cli update`. After an update, the smoke test reads the stream for a `thinking` block that the parsers do not classify. The operator gets an alarm in code and not in a prompt.

## Causal chain to the goal metric

A code check catches a parser break that a persona cannot, with no prompt bytes. The chain is weak here: both stream parsers already classify the `thinking` block type (E4).

## Quantified expected effect

0 live targets today (E4). The check finds nothing to guard until the CLI changes its stream format (A2).

## Cost and time to effect

One build cycle for a check with no live target (E9).

## Top two risks

1. The check is dead code until the format changes. Detection: no test failure ever reaches it.
2. The check duplicates the parser tests. Detection: a diff review of the two test files in E4.
