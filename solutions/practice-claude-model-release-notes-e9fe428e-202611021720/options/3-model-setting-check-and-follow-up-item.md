# Option 3 — answer in code and config, not in a persona

All numbers cite `assumptions-and-evidence.md`.

## What changes, for whom

File one inbox item for a check in `evolve cli update` or the manifest tests. The check fails in two cases. A manifest or profile sends `speed: "fast"` to a model that the release notes drop. Or it sets a thinking field on Opus 5.5. The operator gets an alarm in code and not in a prompt.

## Causal chain to the goal metric

A code check catches a bad setting before dispatch, with no prompt bytes. The chain is weak today: the search of E5 finds 0 live targets.

## Quantified expected effect

0 live targets today (E5). The check finds nothing until a person adds such a setting. The page lists 1 dated removal that is open: fast mode for Claude Opus 4.8 on 2026-11-30 (E2).

## Cost and time to effect

One build cycle for a check with no live target (E8).

## Top two risks

1. The check is dead code until a bad setting appears. Detection: no test failure ever reaches it.
2. The list of dropped modes goes stale. Detection: the next watch diff of the page (A1).
