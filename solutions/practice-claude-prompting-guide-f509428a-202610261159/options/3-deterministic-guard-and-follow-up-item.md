# Option 3 — answer in code and config, not in a persona

All numbers cite `assumptions-and-evidence.md`.

## What changes, for whom

File one inbox item for a deterministic prompt linter or bridge prefill check in `go/internal/`. Add no prompt text to any persona file (E8).

## Causal chain to the goal metric

A code check prevents malformed tool calls or deprecated prefill usage deterministically. No prompt bytes are added. However, the loop uses native drivers that already handle request parameters (E5).

## Quantified expected effect

0 bugs caught today (E5). A future gain occurs only if the bridge enables unsupported prefill requests.

## Cost and time to effect

One build cycle for the Go check, plus tests, covering no active failure mode (E5, E9).

## Top two risks

1. The code check is dead code without an active failure mode, violating the caller-proof rule.
2. Static rules on prompt wording can cause false alarms on legitimate instructions (A4).
