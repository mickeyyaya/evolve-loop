# Option 3 — Retire the watch and redirect the budget to the lanes that carry the tokens

All numbers cite `assumptions-and-evidence.md`.

## What changes, for whom

The watch item closes with no scheduled re-check. Both families are marked "not applicable to a
closed-CLI fleet". The effort that quarterly re-checks would cost goes to the lanes that carry 99.9% of
dispatches: agy, claude and codex (E1). There it goes to same-vendor levers that are already productized,
such as server-side compaction and prefix caching (E14), through the existing part-1 context-editing
work.

The operator carries one fewer standing research item. The token-optimization effort concentrates where
the ≈1.22B measured tokens are (E8).

## Causal chain to the goal metric

Stop spending on families with no reachable path ⇒ spend the same small budget on levers that touch
billed tokens ⇒ the goal metric moves on the lanes that carry the tokens.

## Quantified expected effect

- **Tokens:** it saves about one small document cycle per quarter of re-check cost (A4). It gives up the
  counterfactual ≤0.06% of unbilled local prefill tokens (derived figures).
- **Latency:** it gives up ≈2.0 s per counterfactual ollama triage dispatch at 8B (derived figures, A2,
  A6).
- **Redirect value:** not quantified here. It depends on the compaction and caching work already tracked
  in part 1, and this option adds no new evidence for it.

## Cost and time-to-effect

The cost is zero, and the effect is immediate. The research item closes.

## Top risks and early detection

1. **An applicability flip goes unnoticed indefinitely.** Latent work is moving fast. Five relevant
   papers appeared between 2026-06 and 2026-08 (E10–E13), and an ollama KV PR is open (E5).
   *Detection:* none, by construction, which is the core weakness of this option.
2. **The same-vendor compaction redirect duplicates existing work.** *Detection:* part 1's context-editing
   and cache-discipline items already cover it. If they do, this option saves only the re-check cost (A4).
