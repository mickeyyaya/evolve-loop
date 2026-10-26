# Option 3 — answer in driver config, not in a persona

All numbers cite [assumptions-and-evidence.md](../assumptions-and-evidence.md).

## What changes, for whom

File an inbox item for the native agy bridge driver.
Verify that the `prompt_marker` and `exhausted_regex` in `manifests/agy-tmux.json` still match upstream output (E9).
The operator owns the driver manifest.

## Causal chain to the goal metric

A driver check catches broken regex patterns without adding persona prompt bytes.
A changed prompt marker causes timeouts in tmux sessions.
Verifying strings keeps bridge dispatch reliable.

## Quantified expected effect

Zero breakages detected today.
The manifest holds two text matches that a release can alter (E9).
The gain is real only if an upstream release changes these exact strings.

## Cost and time to effect

One build cycle for the check and its test.
The caller proof rule requires a production caller for each new symbol.

## Top two risks

1. The check is redundant if bridge drivers already report errors loudly (E9).
2. A canary that monitors page text can trigger false alerts when marketing copy shifts (A4).
