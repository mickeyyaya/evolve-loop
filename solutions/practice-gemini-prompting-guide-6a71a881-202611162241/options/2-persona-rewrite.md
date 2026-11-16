# Option 2 — Rewrite agy personas to follow the prompting guide

Citations: [assumptions-and-evidence.md](../assumptions-and-evidence.md).

## What changes

Each agy persona is rewritten against the prompting strategies of the source. This needs the content of the guide, which the delta does not carry (E1, E4).

## Causal chain to the goal

A rewrite changes prompt format. The source gives no changed rule, so the chain has no cause (E1).

## Expected effect

No effect can be quantified, because the delta has no guidance to apply (E1, E4).

## Cost and time to effect

Several persona files and an ADR-0133 review for each (A2). Several cycles.

## Risks and early detection

1. A persona regression with no source reason. Detection: the persona eval scores fall.
2. A rewrite that treats pasted lines as instructions (A1). Detection: the audit reads the diff against the delta.
