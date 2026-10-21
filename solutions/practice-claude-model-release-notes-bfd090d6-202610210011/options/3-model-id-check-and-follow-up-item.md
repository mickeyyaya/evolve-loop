# Option 3 — answer in code and config, not in a persona

All numbers cite `assumptions-and-evidence.md`.

## What changes, for whom

File one inbox item for a check in `evolve cli update` or the manifest tests. The check fails when a manifest, profile or policy file names a model ID that the release notes retire. The operator gets an alarm in code and not in a prompt.

## Causal chain to the goal metric

A code check catches a retired ID before dispatch, with no prompt bytes. The chain is weak today: the search of E5 finds 0 live retired IDs.

## Quantified expected effect

0 live targets today (E5). The check finds nothing until a person adds a retired ID. The page lists 2 retirements: Claude Opus 4.1 (August 5) and Claude Sonnet 4.5 (November 30, 2026) (E2).

## Cost and time to effect

One build cycle for a check with no live target (E8).

## Top two risks

1. The check is dead code until a retired ID appears. Detection: no test failure ever reaches it.
2. The retired-ID list goes stale. Detection: the next watch diff of the page (A1).
