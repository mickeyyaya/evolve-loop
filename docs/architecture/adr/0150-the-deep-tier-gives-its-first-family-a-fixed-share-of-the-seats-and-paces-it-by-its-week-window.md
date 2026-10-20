# ADR-0150: the deep tier gives its first family a fixed share of the seats and paces it by its week window

- **Status:** Accepted (2026-10-21).
- **Plan:** [deep-tier-share-2026-10](../../plans/deep-tier-share-2026-10.md). **Research:** [deep-tier-share-2026-10](../../research/deep-tier-share-2026-10.md). **Design:** [deep-tier-share](../deep-tier-share.md).
- **Relates to:**
  - [ADR-0119](0119-one-routing-table-one-resolver.md): the share is data of the one routing table, and the resolver stays deterministic.
- **Evidence:**
  - The deep tier lists agy first, so Argon takes every deep seat until its week runs out (research F1.1).
  - On 2026-10-17 Argon was at 100% of its week with 153 hours to the reset. Claude then carried the deep work for six days (F1.2).
  - The usage record already holds the Argon week window with its reset (F1.4).
  - A hash of a stable key splits traffic by percent with no stored state (P2.3).

## Context

A first-in-chain rule is an order, not a share. The operator asked for a true share of the deep seats between Argon and Claude. The router must stay deterministic, and the walk must still fall through an exhausted target.

## Decision

1. `cli_routing.tiers.<tier>.share_pct` gives the first family of the tier chain that percent of the seats.
2. A seat is a bucket from 0 to 99: an FNV-1a hash of the cycle, the agent and the phase.
3. A bucket below the share keeps the seat on the shared family.
4. A family whose week is used faster than the week goes gives the seat away.
5. A seat that is given away moves the shared family to the end of the chain.
6. The checked-in policy sets the deep share to 50.
7. An explicit pin wins over the share. The stage skips the share when an env override leads the chain, for example `EVOLVE_BUILDER_CLI=agy`.
8. For a profile-rule agent, a caller CLI or an advisor CLI in the shared family also skips the share.

## Consequences

- Argon takes about half of the deep seats, and its week lasts longer. Claude takes the other half.
- The split is the same for the same cycle, agent and phase, so a retry of a phase gets the same first family.
- A dispatch with no cycle always gets one bucket. Only operator verbs dispatch without a cycle.
- When no usage window is readable, only the share decides. The trace says that the pace does not apply.
- A pinned seat stays on the shared family even when the family is paced. The trace says `share N% at <tier> skipped: <pin> pins <family>`.
- A contract escalation resolves its route with the cycle, so its deep seat gets the bucket of that cycle.

## Rejected

- **Smooth weighted round-robin:** its counters must persist across the lanes of a wave. The router has no shared store (P2.4).
- **A token bucket:** the router does not know the token cost of a dispatch before the launch (P2.5).
- **A pace only:** it sends every seat to Argon while Argon is on pace. Thus the early days of a week repeat the burn of F1.2.
- **A share that wins over a pin:** an env pin is the strongest control of the operator for one agent. The other stages of the resolver already let it win over the headroom order.
- **A share that the advisor sets:** it moves a deterministic decision into a prompt (AGENTS.md rule 5).

## What changes this decision

- The weeks after one full week show that one family still runs out first. Then the share value changes, through `evolve cli-routing set tiers.deep --share-pct N`.
- The usage probe stops recording the per-model week window. Then the pace has no input, and this decision must be checked again.
