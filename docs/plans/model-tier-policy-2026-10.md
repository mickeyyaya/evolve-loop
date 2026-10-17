# Plan: each model tier has one purpose, one chain across separate quota pools, and Gemini 4 Argon (Medium) for agy at deep and top

- **Status:** proposed, 2026-10-17.
- **Decision:** [ADR-0136](../architecture/adr/0136-each-model-tier-walks-separate-quota-pools-and-agy-runs-argon-medium-at-deep-and-top.md). **Spec of record:** [model-routing](../reference/model-routing.md), section "The tier policy". **Earlier work:** [cli-routing-table-2026-10](cli-routing-table-2026-10.md) and [ADR-0119](../architecture/adr/0119-one-routing-table-one-resolver.md).

## Table of contents

- [1. Request](#1-request)
- [2. Terms](#2-terms)
- [3. Facts](#3-facts)
- [4. Goals and non-goals](#4-goals-and-non-goals)
- [5. The tier policy](#5-the-tier-policy)
- [6. How to hold Argon at Medium: three mechanisms](#6-how-to-hold-argon-at-medium-three-mechanisms)
- [7. Decisions](#7-decisions)
- [8. Components and order](#8-components-and-order)
- [9. Verification](#9-verification)
- [10. Risks](#10-risks)
- [11. Open questions](#11-open-questions)

## 1. Request

On 2026-10-17 the operator asked: "let's also include the gemini top / deep model as Gemini 4 Argon with medium effort. I would let you design the optimized policy for different usage (deep, fast, balance, etc...)".

Earlier the same day the operator asked to use agy's tokens: "Make sure we leverage token usage from agy". The operator also gave a standing rule: "whenever you planned to write the code, make sure we can always map to the doc that explains why and what and how".

## 2. Terms

- **Tier:** one of four steps of model strength: `fast`, `balanced`, `deep` and `top`. A phase profile names its default tier.
- **Chain:** the ordered list of CLI families that a dispatch tries, one after another, until one answers.
- **Family:** a routing family: `claude` (Claude Code), `agy` (the Gemini models of Antigravity) and `agy-claude` (the Claude models that the agy binary serves).
- **Quota pool:** one usage limit that a provider counts on its own. When a pool is empty, every model in it stops, and models in other pools still work.
- **Bench:** the router marks a family as unusable until a time, after a quota wall.
- **Floor seat:** a phase that runs on Claude Code only, so that its review stays independent of the builder. The floor seats are the auditor, the adversarial review, the tdd engineer, the code reviewer and the spec verifiers.

## 3. Facts

| Id | Fact | Source |
|---|---|---|
| T1 | On main, 67 tracked phase profiles default to `balanced`, 26 to `deep`, and none to `fast` or `top`. The runtime plane adds 8 minted profiles, for 72 and 29 in all. | `.evolve/profiles/*.json` on main and on the runtime plane, 2026-10-17 |
| T2 | The advisor raised the build of 8 out of 17 recent cycles to `deep`. Those builds then ran on Claude Code. | `phase-plan.json` of cycles 1879–1914 |
| T2b | 11 of 18 build dispatches in cycles 1879–1896 ran on Claude Code. 7 ran on agy. | the `cli` field of `build-events.ndjson`, 2026-10-17 |
| T3 | agy counts quota in separate pools. On 2026-10-17, Gemini 4 Argon was exhausted for 148 h, while Gemini 3.8 Flash still answered. agy's Claude and GPT group was at 100% of its week. | `agy -p` probes and `evolve clihealth`, 2026-10-17 |
| T4 | One Argon wall benched the whole agy family, Flash included, for 30 min. Wave 111 then ran every phase on Claude Code. | `.evolve/cli-health.json`; inbox `agy-model-429-benches-whole-family` |
| T5 | Because of T4, PR #910 took `agy` out of `tiers.deep` and `tiers.top` again. | `.evolve/policy.json` on main |
| T6 | agy has no effort flag. Its effort is part of the model name: `Gemini 4 Argon (Medium)`. agy-claude also takes its effort as a name suffix. Claude Code takes `--effort`. | `docs/reference/model-routing.md` "Effort for each tier" |
| T7 | The live catalog picks the model of each tier through a classifier, and it overrides the manifest. On 2026-10-17 it picked `Gemini 4 Argon (High)` for agy `top`. | `go/internal/modelquery/query.go:87-117`, `.evolve/model-catalog.json` |
| T8 | No setting can fix the model of one tier for one family. A `pins` key beside the routing table refuses every launch, and a pin names a phase, not a tier. | `go/internal/policy/core.go:104-139`, CLAUDE.md "Routing" |
| T9 | A build on Gemini 4 Argon (cycle 1899) and builds on Gemini 3.8 Flash (cycles 1900, 1902) passed the Claude audit and shipped. | wave 109 and 110 records |
| T10 | Claude Code was at 52% of its weekly quota on 2026-10-17. The floor seats need it every cycle. | `/usage`, 2026-10-17 |
| T11 | The operator directive of 2026-10-08 is to start every effort at medium, and to raise it only on a measured gain. | `docs/reference/model-routing.md` |

## 4. Goals and non-goals

**Goals**

- **G1:** each tier has one written purpose. A reader can tell from the purpose which tier a new phase needs.
- **G2:** a quota wall in one pool moves the work to the next pool. It never stops all of agy (T3, T4).
- **G3:** agy runs Gemini 4 Argon (Medium) at `deep` and `top`, as the operator asked.
- **G4:** Claude Code quota goes first to the floor seats. Other work uses agy's pools before Claude Code (T10).

**Non-goals**

- A new tier, or a change to the floor seats.
- A change to the effort of Claude Code. The 2026-10-08 directive stays (T11).
- The per-model bench fix itself. It is inbox `agy-model-429-benches-whole-family`, and this plan depends on it.

## 5. The tier policy

The policy gives each tier a purpose, a chain and an effort. The chain order follows one rule.

**The chain rule:** walk from the pool with the most headroom to the pool that the floor seats need. Inside one tier, never put two models of the same quota pool next to each other.

**Why this rule.** A quota wall empties one pool at a time (T3). The split of Flash and Argon into two pools rests on one observation, not on a documented limit of agy. If the next hop is in another pool, the dispatch still gets an answer. If two hops share a pool, the second one fails the same way and wastes a dispatch. Claude Code goes last, because the floor seats have no other place to run (G4).

Today each chain has one entry for each family, so the rule holds trivially. It is a design check for a future chain with two models of one family.

| Tier | Purpose: use it when the phase… | Chain | Model and effort per family | Why this order |
|---|---|---|---|---|
| `fast` | does a mechanical transform that a later step checks: condense a context, format a list. | `agy` → `agy-claude` → `claude` | agy: Gemini 3.8 Flash (Low). agy-claude: Sonnet 5.5 (Low). claude: haiku, `--effort low`. | Flash (Low) uses the largest pool. No profile uses `fast` today (T1), and this plan keeps it unused until a measurement shows no quality loss. |
| `balanced` | reads and writes code or reports where a reviewer checks the result: scout, triage, scans, routine builds. | `agy` → `agy-claude` → `claude` | agy: Gemini 3.8 Flash (High). agy-claude: Sonnet 5.5 (Medium). claude: sonnet, medium. | Flash carries the bulk on the Gemini pool, and builds on Flash ship (T9). agy-claude comes before Claude Code (G4). When agy-claude's pool is empty, the router benches it, and the walk skips it. This is the current order. |
| `deep` | must reason about correctness, failure or design, and no floor seat checks it at the same depth: debugger, architecture design, router, deep builds. | `agy-claude` → `agy` → `claude` | agy-claude: Opus 5.5 (Medium). agy: **Gemini 4 Argon (Medium)**. claude: opus, medium. | Opus on agy's Claude pool first, for Claude quality. Then Argon on its own Gemini pool. Claude Code last (G4). |
| `top` | decides a dispute, or a decision that is expensive to undo. No profile uses it today (T1). | `agy-claude` → `agy` → `claude` | agy-claude: Opus 5.5 (Medium). agy: **Gemini 4 Argon (Medium)**. claude: opus, medium. | The same chain as `deep`. The tier exists for a later stronger setting. Medium follows T11 until a measurement shows a gain from High. |
| floor seats | review the work of another phase. | `claude` only | opus or sonnet per profile, medium | Independence: the reviewer must not share the builder's model family. This is unchanged. |

**Why Argon (Medium) and not (High) at `top`.**
- **Operator:** the operator asked for medium.
- **Directive:** the effort directive starts every tier at medium (T11).
- **Quota:** High uses more of the Argon pool for each call, and that pool was the first to run out (T3).

**Why `deep` builds need a second look.** The advisor raises about half of all builds to `deep` (T2). A `deep` build uses the Claude pools, which are scarce. The policy asks the advisor to raise a build only for these risk classes:
- a protected surface;
- concurrency or locks;
- security;
- the kernel and the state machine.

The C1 telemetry of the CI plan counts the share of `deep` builds and their pass rate. Thus the change has a measured before and after.

## 6. How to hold Argon at Medium: three mechanisms

The manifest already maps agy `deep` to Argon (Medium). But the live catalog overrides the manifest, and it picked Argon (High) for `top` (T7). No current setting can fix it (T8). Three mechanisms can.

### M1. Change the manifest only

Set `top` to `Gemini 4 Argon (Medium)` in `go/internal/bridge/manifests/agy-tmux.json`.

- **Advantage:** one line, and it is the offline default when the catalog is absent.
- **Weakness:** the live catalog wins over the manifest (T7), so the dispatch still gets Argon (High).

### M2. A tier pin per family in the catalog policy

Add `catalog.tier_pins` to `.evolve/policy.json`, for example `{"agy": {"top": "Gemini 4 Argon (Medium)", "deep": "Gemini 4 Argon (Medium)"}}`. The catalog applies a pin after the classifier, and a pin wins over the classifier.

- **Advantage:**
  - It is config, not code. The operator can change it at a wave boundary.
  - It names exactly what the operator asked for.
  - It works for any family.
- **Weakness:** a pin can name a model that agy no longer offers. `Refresh` must then return an error. Today `snapshotFor` turns each `liveTiers` error into a WARN and a fallback (`go/internal/modelquery/query.go:58-78`). So the pin check must sit outside that path, in `Refresh` itself.
- **Where it hooks:** in `liveTiers` (`query.go:87-117`), after `CompleteTiers(PromoteLatest(...))`, and also on the early return for a reusable prior. The pins also join the `Fingerprint` input, so a changed pin forces a new classification.

### M3. Make effort select the agy variant

Treat `(Low)`, `(Medium)` and `(High)` as effort for agy, as agy-claude already does. The tier picks the model, `Gemini 4 Argon`, and `tiers.top.effort` picks the variant.

- **Advantage:**
  - One meaning of effort for all families.
  - The current effort commands (`evolve cli-routing set tiers.top --effort medium`) then work for agy too.
- **Weakness:** today `balanced` runs Gemini 3.8 Flash (High), while its effort is `medium`. M3 then moves `balanced` to Flash (Medium), a change that no one asked for. A per-family effort override must come first.

### Scored comparison

Scores are 1 (worst) to 5 (best).

| Criterion (weight) | M1 manifest | M2 tier pin | M3 effort variant |
|---|:-:|:-:|:-:|
| Gives Argon (Medium) at `top` today (×3) | 1 | 5 | 4 |
| No change that no one asked for (×3) | 5 | 5 | 2 |
| Effort and risk (×2) | 5 | 4 | 2 |
| One model of effort across families (×1) | 1 | 2 | 5 |
| **Weighted total (out of 45)** | **29** | **40** | **27** |

**The winner is M2, with M1 as its offline default.** M2 is the only mechanism that gives the operator's model today with no side effect. M1 stays as the default when no catalog exists. M3 has the best long-term shape, and it becomes a follow-up after a per-family effort override exists.

## 7. Decisions

| Id | Decision | Facts |
|---|---|---|
| D1 | Each tier has the purpose and chain of §5. | T1–T4, T10 |
| D2 | The chain of `deep` and `top` is `agy-claude` → `agy` → `claude`. | T3, T10 |
| D3 | agy runs `Gemini 4 Argon (Medium)` at `deep` and `top`, through a catalog tier pin (M2). | T6–T8 |
| D4 | The manifest maps agy `top` to `Gemini 4 Argon (Medium)` as the offline default (M1). | T7 |
| D5 | `agy` returns to the `deep` and `top` chains only after the per-model bench fix lands. | T4, T5 |
| D6 | The advisor raises a build to `deep` only for the four risk classes of §5. | T2 |
| D7 | `fast` stays unused until a measurement shows no quality loss. | T1 |
| D8 | A pin that names a model that the CLI does not offer makes `Refresh` return an error. The catalog does not fall back to the detect map. | M2 weakness |

## 8. Components and order

| Id | Component | Red test first | Depends on |
|---|---|---|---|
| P1 | D4: the manifest maps agy `top` to `Gemini 4 Argon (Medium)`, and the pin test follows | `TestAgyTierModelsPinnedToVerifiedNames` expects Medium at `top` | — |
| P2 | D3 and D8: a `catalog.tier_pins` field in the strict policy schema, a `TierPins` field on `RefreshDeps`, and the pin step in `liveTiers` | Red tests: a refresh with a pin returns the pinned model. A pin to an absent model makes `Refresh` return an error. A pin applies when the prior is reusable. A changed pin changes the fingerprint. | — |
| P3 | the per-model bench fix | the tests of inbox `agy-model-429-benches-whole-family` | — |
| P4 | D2 and D5: `agy` returns to `tiers.deep` and `tiers.top` | `evolve cli-routing explain debugger` shows `agy-claude → agy → claude` | P2, P3 |
| P5 | D6: the advisor rule for `deep` builds | the advisor test: a build with no risk class stays `balanced` | the CI plan's C1, for the measure |

The order is P1, P2, P3, P4, P5.
1. P1 and P2 are safe now. `agy` is out of the `deep` and `top` chains (T5), so they change no dispatch.
2. P3 must come before P4, or one Argon wall benches all of agy again (T4).
3. P5 waits for the telemetry, so its effect has a measure.

## 9. Verification

| Goal | How it is measured | Target |
|---|---|---|
| G2 | the share of phases that run on Claude Code while one agy pool is empty | the agy phases stay on agy |
| G3 | `evolve models list` and the booted model label of a `deep` agy launch | `Gemini 4 Argon · medium` |
| G4 | the share of non-floor dispatches on Claude Code, before and after P4 and P5 | down from 11 of 18 build dispatches (T2b) |
| quality | the audit pass rate of builds per family and tier, over 20 cycles | no drop against the `balanced` baseline |

## 10. Risks

| Risk | Control |
|---|---|
| Argon (Medium) makes weaker `deep` builds than Opus. | Opus on agy-claude stays first. Argon runs only when agy's Claude pool is empty. The audit pass rate per family is measured. |
| A pin outlives the model that it names. | D8: the refresh fails loudly. The boundary refresh then shows the error. |
| Argon's quota runs out again. | After P3, an Argon wall benches Argon only. The chain walks on to Claude Code. |
| The advisor rule for `deep` builds hides a real risk. | The four risk classes come from the protected-surface list. A FAIL on a `balanced` build goes to the retrospective, as today. |

## 11. Open questions

- **Q1:** keep `fast` unused, or try it on `context-condense` only? We propose to keep it unused until the CI telemetry exists.
- **Q2:** does `top` need a different chain from `deep`? We propose one chain until a profile uses `top`.
- **Q3:** which verb writes `catalog.tier_pins`? The `evolve cli-routing` verbs write only `cli_routing`. We propose a direct edit of `.evolve/policy.json` in a reviewed PR, as for the other `catalog` keys, until a verb exists.
