# Deep tier share: the first family of a tier takes a fixed share of the seats, paced by its week

> [ADR-0150](adr/0150-the-deep-tier-gives-its-first-family-a-fixed-share-of-the-seats-and-paces-it-by-its-week-window.md) · plan: [deep-tier-share-2026-10](../plans/deep-tier-share-2026-10.md) · research: [deep-tier-share-2026-10](../research/deep-tier-share-2026-10.md).
> This file is the spec of record for the share stage of the router. The package note [internal-cliroute.md](packages/internal-cliroute.md) holds the stage order of the resolver.
>
> **Terms used throughout:**
>
> - **Seat:** one dispatch of one agent for one phase of one cycle.
> - **Shared family:** the first family of the tier chain of a tier that has `share_pct`.
> - **Bucket:** a number from 0 to 99 that a hash of the seat gives.
> - **Pace:** the used percent of the week window against the percent of the week that is gone.

## Table of contents

1. [Goals](#1-goals)
2. [The share](#2-the-share)
3. [The pace](#3-the-pace)
4. [The operator surfaces](#4-the-operator-surfaces)
5. [Guards](#guards)
6. [Limits](#limits)

## 1. Goals

| Goal | Measure | Target |
|---|---|---|
| G1 Argon takes its share of the deep seats | seats that Argon leads over 100 dispatches | about `share_pct` (40 to 60 at 50) |
| G2 A paced week gives the seat away | the first family of a paced seat | the next family of the chain |
| G3 The operator sees the share and the pace | `evolve cli-routing explain` | one `share:` note |
| G4 The walk does not change | the walk and bench tests | unchanged |

G4 comes first: a share that breaks the fall-through costs more than the share gains.

## 2. The share

### Issue

The deep tier lists `agy`, `agy-claude` and `claude`. Argon took every deep seat until its week ran out (research F1.1, F1.2).

### Gap

The router orders a chain. It has no stage that splits the seats of a tier between two families.

### Solution

The resolver runs `shareTier` after `applyLadder`, when the tier of the seat is final (`go/internal/cliroute/share.go`).

1. The stage reads the tier of the seat: the first entry of `plan.Tiers`, or `plan.Model`.
2. When the tier has a share and the chain starts with the shared family, the stage hashes the seat. The hash is FNV-1a of `<cycle>|<agent>|<phase>`, modulo 100.
3. A bucket below `share_pct` keeps the seat on the shared family, unless the pace gives it away (§3).
4. Otherwise the shared family moves to the end of the chain. The walk can still fall back to it.

A worked example with `share_pct: 50`: the builder of cycle 1741 at deep gets bucket 1, so Argon keeps the seat. The builder of cycle 1742 gets bucket 52, so the chain becomes `agy-claude`, `claude`, `agy`.

The runner and the bridge chain put the cycle into the route request (`Request.Cycle`). Thus a retry of the same phase gets the same first family.

The contract escalation of the cycle (`go/internal/core/contract_escalation.go`) also puts the cycle into its route request.

An explicit pin wins over the share (`sharePin`). The stage skips the share in two cases:

- An env override leads the chain, for example `EVOLVE_BUILDER_CLI=agy`. The plan source then has the `env(` prefix.
- The agent has no table rule, and the bridge caller CLI or the advisor CLI is in the shared family.

The trace then reads `share 50% at deep skipped: env(EVOLVE_BUILDER_CLI) pins agy`.

## 3. The pace

### Issue

A fixed share still lets Argon spend its week in the first days when the share is high.

### Gap

`usageprobe.WeekUsed` skips each per-model window, so the router did not see the Argon week (research F1.3).

### Solution

The host reader `TierWeek` (`projectTierWeek` in `go/cmd/evolve/cli_routing_lanes.go`) finds the tier model of the family in its manifest. For agy at deep this is Gemini 4 Argon (Medium). It then calls `usageprobe.TierWeek`. That function returns the live week window whose model is a prefix of the tier model, or else the week window of the family.

The share stage compares the used percent with the percent of the week that is gone: `100 × (1 − time to the reset ÷ 7 days)`. When the used percent is higher, the family is paced and gives the seat away. With 60% used and 6 days to the reset, 14% of the week is gone, so Argon is paced.

When several windows match, the longest model name wins, and then the highest used percent. A window model matches the whole tier model, or a start of it that a space follows.

When the reader is absent, fails, finds no window, or finds no reset time, the pace does not apply. The trace says why, and only the share decides.

## 4. The operator surfaces

- `evolve cli-routing set tiers.deep --share-pct 50` writes the share. `evolve cli-routing unset tiers.deep.share_pct` removes it.
- The compile step refuses three shares: a value outside 1 to 99, a tier with no `clis`, and a chain of one family.
- `evolve cli-routing explain <agent>` prints a `share:` note when the chain of the agent holds the shared family (`Router.ShareNotes`). The note names the tier, the family, the percent and the pace.
- The resolver trace of each seat records the bucket, the pace and the result.

## Guards

| Rule | Guard | State |
|---|---|---|
| About the share over 100 seats | `TestShare_ADeepWalkOverAHundredDispatchesSendsArgonAboutItsShare` | built |
| The same seat gets the same family | `TestShare_TheSameDispatchAlwaysGetsTheSameSeat` | built |
| A paced week gives the seat away | `TestShare_APacedGroupGivesTheSeatToTheNextHealthyFamily` | built |
| No usable reading leaves only the share | `TestShare_WithoutAUsageReadingOnlyTheShareDecides` | built |
| Other tiers and other chains are untouched | `TestShare_AnotherTierIsUntouched`, `TestShare_AChainThatDoesNotLeadWithTheSharedFamilyIsUntouched` | built |
| The compile check of `share_pct` | `TestCompile_AShareNeedsATierChainAndAPercentInRange` | built |
| The cycle reaches the router | `TestRouteRequest_CarriesTheCycleToTheRouter`, `TestDefaultPlanResolver_TheCycleReachesTheDeepTierShare` | built |
| The Argon window is read for the deep tier | `TestTierWeek_TheModelWindowOfTheTierModelWinsOverTheFamilyWindow`, `TestResolveModelTier_TheAgyDeepSeatIsArgon` | built |
| An env pin or a caller CLI pin wins over the share | `TestShare_AnOperatorEnvPinKeepsTheSharedFamilyOnTheSeat`, `TestShare_ACallerOrAdvisorCLIPinKeepsTheSharedFamilyOnTheSeat` | built |
| The escalation route carries the cycle | `TestRoutingDecision_TheEscalationRouteCarriesTheCycleToTheDeepShare` | built |
| A given seat keeps `agy-claude` as its own family | `TestShare_AGivenSeatKeepsAgyClaudeAsItsOwnFamilyBeforeClaude` | built |
| The window pick is the same on each read | `TestTierWeek_TwoWindowsOfOneModelReadTheMostUsed`, `TestTierWeek_TheLongestMatchingWindowModelWins` | built |
| Explain prints the share and the pace | `TestCLIRoutingExplain_PrintsTheShareAndThePaceOfASharedTier`, `TestShareNotes_NameEachSharedTierAndItsPace` | built |

## Limits

- The share splits the seats between the shared family and the rest of the chain. It does not set a share for the second family.
- The pace compares percents of one week. It does not know the token cost of a seat.
- A dispatch with no cycle always gets one bucket. Only operator verbs dispatch without a cycle.
- The stage runs once, at the tier of the seat. When the walk steps down a tier, the chain keeps the order that the share gave it.
- No test proves the live result. The operator compares the Argon and Claude weeks after one full week (plan §8).
