# Plan: the deep tier shares its seats between Argon and Claude by budget

- **Status:** built, 2026-10-21.
- **Design:** [deep-tier-share](../architecture/deep-tier-share.md). **Decision:** [ADR-0150](../architecture/adr/0150-the-deep-tier-gives-its-first-family-a-fixed-share-of-the-seats-and-paces-it-by-its-week-window.md). **Research:** [deep-tier-share-2026-10](../research/deep-tier-share-2026-10.md).

## 1. Request

On 2026-10-17 the operator asked to share the Opus deep phase with Argon, and approved "a true share" (inbox item `deep-tier-shares-argon-by-budget`). The item says: "A first-in-chain rule is not a share: every deep dispatch went to Argon until its group was exhausted."

## 2. Facts

| Fact | Research id |
|---|---|
| The deep tier lists agy first, and the agy deep seat runs Gemini 4 Argon. | F1.1 |
| The order spent the Argon week in about four waves. Claude then carried the deep work for six days. | F1.2 |
| No stage reads the Argon window, and no stage splits the seats of a tier. | F1.3 |
| The usage record holds the Argon window with its used percent and its reset. | F1.4 |
| The resolution must stay deterministic. The cycle is a stable key for each dispatch. | F1.5 |
| An exhausted Argon window benches only the Argon target, and the walk falls through. | F1.6 |

## 3. Goals and non-goals

Goals:

- **G1** Argon takes about the configured share of the deep seats, not all of them.
- **G2** When Argon spends its week faster than the week goes, the next deep seat goes to the next healthy family.
- **G3** `evolve cli-routing explain <agent>` prints the share and the pace.
- **G4** The walk does not change. An exhausted target still falls through, and an exhausted Argon window still benches only Argon.

Non-goals:

- A share for the other tiers. The rule works for any tier, but only `deep` gets a value now.
- A share that the advisor sets for each phase (research §3).

## 4. Candidate designs

Each criterion scores 1 (poor) to 5 (good).

| Design | Gain | Risk | Change size | Total |
|---|---|---|---|---|
| A. A fixed share for each tier, by a hash of the cycle | 4 | 4 | 4 | 12 |
| B. A pace only: the family takes the seat while it is on pace | 3 | 4 | 4 | 11 |
| C. Smooth weighted round-robin with stored counters | 4 | 2 | 2 | 8 |
| D. A share that the advisor sets for each phase | 3 | 2 | 3 | 8 |
| **A + B.** The share decides, and the pace can only give a seat away | 5 | 4 | 3 | **12** |

A alone keeps the share when Argon runs ahead of its week. B alone sends every seat to Argon while it is on pace, which repeats F1.2 early in each week. A + B ties A on the total, but it is the only design with a gain of 5. C needs a store that each lane of a wave writes. D moves a deterministic decision into a prompt.

## 5. Decisions

| Id | Decision | Research id |
|---|---|---|
| D1 | `cli_routing.tiers.<tier>.share_pct` (1 to 99) gives the first family of the tier chain that percent of the seats. | R1 |
| D2 | The bucket of a seat is an FNV-1a hash of the cycle, the agent and the phase, modulo 100. | R2 |
| D3 | A family whose week window is ahead of the elapsed week gives the seat away. | R3 |
| D4 | A seat that is given away moves the shared family to the end of the chain. | R4 |
| D5 | Explain prints a `share:` note for each shared tier whose family is in the chain. | R5 |
| D6 | `evolve cli-routing set tiers.<tier> --share-pct N` writes the share, and `unset tiers.<tier>.share_pct` removes it. | F1.5 |
| D7 | The checked-in policy sets `tiers.deep.share_pct` to 50. | F1.2 |

## 6. Components

| Id | Component | Red test first | Owner |
|---|---|---|---|
| S1 | `policy.TierRule.SharePct` and its JSON form | `TestTierRule_AShareKeepsTheObjectFormThroughARoundTrip` | console |
| S2 | The `shareTier` stage and the compile check in `internal/cliroute` | `TestShare_ADeepWalkOverAHundredDispatchesSendsArgonAboutItsShare` | console |
| S3 | `usageprobe.TierWeek` and the host reader `projectTierWeek` | `TestTierWeek_TheModelWindowOfTheTierModelWinsOverTheFamilyWindow` | console |
| S4 | The cycle in the route request (runner and bridge chain) | `TestRouteRequest_CarriesTheCycleToTheRouter` | console |
| S5 | The explain note and the `--share-pct` writer | `TestCLIRoutingExplain_PrintsTheShareAndThePaceOfASharedTier` | console |
| S6 | `tiers.deep.share_pct: 50` in `.evolve/policy.json`, through `evolve cli-routing set` | the explain output of the checked-in table | operator |

## 7. Order

1. S1, because every other part reads the field.
2. S2, the rule itself, with an injected usage reader.
3. S3 and S4, the production inputs of S2.
4. S5, the operator surfaces.
5. S6 last, so that the value lands with the code that reads it.

## 8. Verification

- **G1:** over 100 simulated dispatches at 0% used and 7 days to the reset, Argon leads between 40 and 60 seats at a 50% share. Before the change it led 100.
- **G2:** at 60% used with 14% of the week gone, a seat that the share keeps goes to Claude.
- **G3:** `evolve cli-routing explain builder` prints `share: when the agent dispatches at deep, agy takes 50% of the seats; ...` with the pace.
- **G4:** the existing walk and bench tests pass unchanged (`internal/cliroute`, `internal/bridgechain`).
- **Live measure:** the Argon week at the next reset is below 100% with Argon still in the deep chain. This is an operator check after one week.

## 9. Risks

| Risk | Control |
|---|---|
| The pace reads the window of the last week after its reset. | `TierWeek` drops a window whose reset has passed, so a new week starts with no pace. |
| A dispatch without a cycle (an operator verb) always gets the same bucket. | Such dispatches are rare, and the share still holds for the cycle seats. |
| A wrong manifest tier model matches no window. | The pace then does not apply, and the trace says so. The share still holds. |

## 10. Open questions

- The share value 50 is a first value. After one week, compare the Argon and Claude weeks and set a new value with `evolve cli-routing set tiers.deep --share-pct N`.
