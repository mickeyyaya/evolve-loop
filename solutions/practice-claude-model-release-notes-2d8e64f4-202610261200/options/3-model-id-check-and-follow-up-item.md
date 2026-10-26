# Option 3 — answer in code and config, not in a persona

All numbers cite `assumptions-and-evidence.md`.

## What changes, for whom

File one inbox item for a check in `evolve cli update` or the manifest tests. The check fails in two cases. A manifest, profile or policy file names a model ID that the release notes retire. Or `go.mod` adds an Anthropic SDK. The operator gets an alarm in code and not in a prompt.

## Causal chain to the goal metric

A code check catches a retired ID or a new SDK before dispatch, with no prompt bytes. The chain is weak today: the searches of E4 and E5 find 0 live targets.

## Quantified expected effect

0 live targets today (E4, E5). The check finds nothing until a person adds a retired ID or an SDK. The page lists the retirement of Claude Sonnet 4.5 on 2026-11-30 (E7).

## Cost and time to effect

One build cycle for a check with no live target (E8).

## Top two risks

1. The check is dead code until a retired ID or an SDK appears. Detection: no test failure ever reaches it.
2. The retired-ID list goes stale. Detection: the next watch diff of the page (A1).
