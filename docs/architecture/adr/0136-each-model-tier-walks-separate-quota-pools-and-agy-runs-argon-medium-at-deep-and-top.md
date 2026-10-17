# ADR-0136: Each model tier walks separate quota pools, and agy runs Gemini 4 Argon (Medium) at deep and top

- **Status:** Proposed (2026-10-17).
- **Plan:** [model-tier-policy-2026-10](../../plans/model-tier-policy-2026-10.md). **Spec of record:** [model-routing](../../reference/model-routing.md), section "The tier policy".
- **Relates to:**
  - [ADR-0119](0119-one-routing-table-one-resolver.md): one routing table decides every dispatch. This ADR sets the order inside that table.
- **Evidence:**
  - T3: agy counts quota in separate pools. Argon was empty for 148 h while Flash still answered.
  - T4: one Argon wall benched all of agy, and a whole wave ran on Claude Code.
  - T7, T8: the live catalog picked Argon (High) for `top`, and no setting can fix one tier of one family.
  - T10: the floor seats need Claude Code quota in every cycle.

## Context

The operator asked for Gemini 4 Argon at medium effort on agy's `deep` and `top` tiers. The operator also asked for a tier policy for each kind of work. The work of 2026-10-17 showed why the order of a chain matters. A quota limit empties one pool, not a whole provider. A chain that puts two models of one pool next to each other fails twice for one cause. A chain that ends early on Claude Code takes quota that the floor seats need.

Two technical facts shape the mechanism. agy carries its effort in the model name, not in a flag. The live catalog picks the model of each tier and overrides the manifest. So the manifest alone cannot hold Argon at Medium.

## Decision

1. Each tier has one written purpose: `fast`, `balanced`, `deep` and `top`.
2. A chain walks from the pool with the most headroom to Claude Code.
3. No chain puts two models of one quota pool next to each other.
4. `deep` and `top` walk `agy-claude`, then `agy`, then `claude`.
5. agy runs `Gemini 4 Argon (Medium)` at `deep` and `top`.
6. A catalog tier pin in the policy holds that model, after the classifier.
7. The manifest also maps agy `top` to Argon (Medium), as the offline default.
8. A pin to a model that the CLI does not offer makes the refresh fail.
9. `agy` returns to the `deep` and `top` chains only after the per-model bench fix.
10. The advisor raises a build to `deep` only for four named risk classes.

## Consequences

**Good:**
- A wall in one pool moves the work to the next pool. It no longer stops all of agy.
- Claude Code quota goes first to the floor seats, which have no other place to run.
- The operator can change a pinned model at a wave boundary, with no code change.
- Each tier has a purpose, so a new phase profile can choose its tier from the purpose.

**Bad:**
- A pin can go stale when agy renames a model. Decision 8 makes this visible. An operator must then update the pin.
- Argon (Medium) can make weaker `deep` work than Opus. Opus stays first, and the audit pass rate per family is measured.
- Effort has two meanings for a while: a flag for Claude Code, and a name suffix for agy. The effort-variant mechanism (M3) can unify them later.

**What changes this decision:**
- If the audit pass rate of Argon `deep` work falls below the `balanced` baseline, Argon moves after Claude Code in the chain.
- If agy merges its pools, the chain rule needs a new check.

## Rejected

| Option | What it offers | Why it is not chosen |
|---|---|---|
| M1: change the manifest only | One line, and the offline default is right. | The live catalog overrides the manifest, so a dispatch still gets Argon (High). M1 stays as the offline default only. |
| M3: make effort select the agy variant | One meaning of effort for every family. | Today it also moves `balanced` from Flash (High) to Flash (Medium), a change that no one asked for. It needs a per-family effort first. |
| Put `agy` back in the `deep` chain now | Argon work starts at once. | One Argon wall still benches all of agy (T4). The bench fix must land first. |
| Argon (High) at `top` | More reasoning for each call. | The operator asked for medium. The effort directive starts at medium, and High drains the Argon pool faster. |
