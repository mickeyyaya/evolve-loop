# Option 3 — answer in driver config, not in a persona

All numbers cite `assumptions-and-evidence.md`.

## What changes, for whom

File one inbox item for the agy manifest. It adds a canary that the footer marker and the Argon limit lines still match after each `evolve cli update` (E4). The operator owns the manifest. The canary also covers the `agy plugin validate` and `agy plugin install` calls of the skills-publish command in `go/cmd/evolve/cmd_skills_publish.go:490-497` (E8).

## Causal chain to the goal metric

A driver check catches a break that a persona cannot, with no prompt bytes. A reworded limit line can hide an exhausted Argon model group, and then the loop retries it.

## Quantified expected effect

0 breaks seen in 1.4.3. The manifest holds at least 2 text matches that a release can break (E4). The gain is real only if a release rewords them.

## Cost and time to effect

One build cycle for the check and its test. The caller proof rule needs a production caller for each new symbol.

## Top two risks

1. The canary is dead code if the drivers already fail loudly (E4). The build floor rejects it.
2. A canary on changelog text has false alarms, because the text changes at each release (A1).
