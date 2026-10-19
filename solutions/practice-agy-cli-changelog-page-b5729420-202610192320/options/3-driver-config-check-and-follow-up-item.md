# Option 3 — answer in driver config, not in a persona

All numbers cite `assumptions-and-evidence.md`.

## What changes, for whom

File one inbox item for the agy manifest. It sets the effort explicitly, or confirms that `--model` alone is fine. It adds a canary that the footer marker and the usage-limit text still match after each `evolve cli update` (E10). The operator owns the manifest.

## Causal chain to the goal metric

A driver check catches a break that a persona cannot, with no prompt bytes. A reworded usage-limit message can hide an exhausted model group. Then the loop retries it.

## Quantified expected effect

0 breaks seen today. The manifest holds 2 text matches that a release can break (E10). The gain is real only if a release rewords them.

## Cost and time to effect

One build cycle for the check and its test. The caller proof rule needs a production caller for each new symbol (E10).

## Top two risks

1. The canary is dead code if the drivers already fail loudly (E10). The build floor rejects it.
2. A canary on page text has false alarms, because the page changes often (A4).
