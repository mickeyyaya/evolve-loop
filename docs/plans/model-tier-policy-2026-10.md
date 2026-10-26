# Plan: each model tier has one purpose, one chain across separate quota pools, and Gemini 4 Argon (Medium) for agy at deep and top

- **Status:** accepted, 2026-10-17 (the operator approved it).
- **Research:** [model-tier-policy-2026-10](../research/model-tier-policy-2026-10.md). **Decision:** [ADR-0136](../architecture/adr/0136-each-model-tier-walks-separate-quota-pools-and-agy-runs-argon-medium-at-deep-and-top.md). **Spec of record:** [model-routing](../reference/model-routing.md), section "The tier policy". **Earlier work:** [cli-routing-table-2026-10](cli-routing-table-2026-10.md) and [ADR-0119](../architecture/adr/0119-one-routing-table-one-resolver.md). **Bridge rule:** every component that queries a CLI follows [bridge-factory-2026-10](bridge-factory-2026-10.md); its §8 names the layer of each component.

## Table of contents

- [1. Request](#1-request)
- [2. Terms](#2-terms)
- [3. Facts](#3-facts)
- [4. Goals and non-goals](#4-goals-and-non-goals)
- [5. The tier policy](#5-the-tier-policy)
- [6. How to hold Argon at Medium: three mechanisms](#6-how-to-hold-argon-at-medium-three-mechanisms)
- [6a. Quota pools come from agy's own /usage screen](#6a-quota-pools-come-from-agys-own-usage-screen)
- [6b. /model and /usage together](#6b-model-and-usage-together-what-is-new-and-how-much-of-it-we-can-use)
- [7. Decisions](#7-decisions)
- [8. Components and order](#8-components-and-order)
- [9. Verification](#9-verification)
- [10. Risks](#10-risks)
- [11. Open questions](#11-open-questions)

## 1. Request

On 2026-10-17 the operator asked: "let's also include the gemini top / deep model as Gemini 4 Argon with medium effort. I would let you design the optimized policy for different usage (deep, fast, balance, etc...)".

Earlier the same day the operator asked to use agy's tokens: "Make sure we leverage token usage from agy". Later the operator added: "Agy gemini 4 argon has its own quota, use the /usage to learn from the different LLM limitations and separated quotas through agent bridge". The operator also gave a standing rule: "whenever you planned to write the code, make sure we can always map to the doc that explains why and what and how".

## 2. Terms

- **Tier:** one of four steps of model strength: `fast`, `balanced`, `deep` and `top`. A phase profile names its default tier.
- **Chain:** the ordered list of CLI families that a dispatch tries, one after another, until one answers.
- **Family:** a routing family: `claude` (Claude Code), `agy` (the Gemini models of Antigravity) and `agy-claude` (the Claude models that the agy binary serves).
- **Quota pool:** one usage limit that a provider counts on its own. When a pool is empty, every model in it stops, and models in other pools still work.
- **Bench:** the router marks a family as unusable until a time, after a quota wall.
- **Floor seat:** a phase that runs on Claude Code only, so that its review stays independent of the builder. The floor seats are the auditor, the adversarial review, the tdd engineer, the code reviewer and the spec verifiers.

## 3. Facts

| Id | Fact | Source (research id) |
|---|---|---|
| T1 | On main, 67 tracked phase profiles default to `balanced`, 26 to `deep`, and none to `fast` or `top`. The runtime plane adds 8 minted profiles, for 72 and 29 in all. | [research T1](../research/model-tier-policy-2026-10.md#2-local-evidence) |
| T2 | The advisor raised the build of 8 out of 17 recent cycles to `deep`. Those builds then ran on Claude Code. | [research T2](../research/model-tier-policy-2026-10.md#2-local-evidence) |
| T2b | 11 of 18 build dispatches in cycles 1879–1896 ran on Claude Code. 7 ran on agy. | [research T2b](../research/model-tier-policy-2026-10.md#2-local-evidence) |
| T3 | agy counts quota in separate pools. On 2026-10-17, Gemini 4 Argon was exhausted for 148 h, while Gemini 3.8 Flash still answered. agy's Claude and GPT group was at 100% of its week. | [research T3](../research/model-tier-policy-2026-10.md#2-local-evidence) |
| T4 | One Argon wall benched the whole agy family, Flash included, for 30 min. Wave 111 then ran every phase on Claude Code. | [research T4](../research/model-tier-policy-2026-10.md#2-local-evidence); inbox `agy-model-429-benches-whole-family` |
| T5 | Because of T4, PR #910 took `agy` out of `tiers.deep` and `tiers.top` again. | [research T5](../research/model-tier-policy-2026-10.md#2-local-evidence) |
| T6 | agy has no effort flag. Its effort is part of the model name: `Gemini 4 Argon (Medium)`. agy-claude also takes its effort as a name suffix. Claude Code takes `--effort`. | [research T6](../research/model-tier-policy-2026-10.md#2-local-evidence) |
| T7 | The live catalog picks the model of each tier through a classifier, and it overrides the manifest. On 2026-10-17 it picked `Gemini 4 Argon (High)` for agy `top`. | [research T7](../research/model-tier-policy-2026-10.md#2-local-evidence); `.evolve/model-catalog.json` |
| T8 | No setting can fix the model of one tier for one family. A `pins` key beside the routing table refuses every launch, and a pin names a phase, not a tier. | [research T8](../research/model-tier-policy-2026-10.md#2-local-evidence) |
| T9 | A build on Gemini 4 Argon (cycle 1899) and builds on Gemini 3.8 Flash (cycles 1900, 1902) passed the Claude audit and shipped. | [research T9](../research/model-tier-policy-2026-10.md#2-local-evidence) |
| T10 | Claude Code was at 52% of its weekly quota on 2026-10-17. The floor seats need it every cycle. | [research T10](../research/model-tier-policy-2026-10.md#2-local-evidence) |
| T11 | The operator directive of 2026-10-08 is to start every effort at medium, and to raise it only on a measured gain. | [research T11](../research/model-tier-policy-2026-10.md#2-local-evidence) |
| T12 | Both Argon variants boot. A launch with `--model "Gemini 4 Argon (Medium)"` showed the footer `Gemini 4 Argon · medium`. agy runs its default model on an invalid name, so this check is necessary. | [research T12](../research/model-tier-policy-2026-10.md#2-local-evidence) |
| T13 | The shadow catalog first listed Gemini 4 Argon on 2026-10-15T23:07Z. The live catalog still named Gemini 3.1 Pro on 2026-10-16, because `catalog.refresh_stage` is `shadow`. | [research T13](../research/model-tier-policy-2026-10.md#2-local-evidence) |

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

## 6a. Quota pools come from agy's own /usage screen

### Issue

The policy of §5 needs to know the quota pools. Until now this plan inferred them from failures (T3). agy's own `/usage` screen names the pools directly. agy 1.4.0 showed this screen on 2026-10-17:

| Pool (agy group) | Models in the pool | Weekly left | 5-hour left |
|---|---|---|---|
| `GEMINI MODELS` | Gemini Flash, Gemini Pro | 91.90%, refreshes in 165 h | 73.66%, refreshes in 2 h 21 m |
| `CLAUDE AND GPT MODELS` | Claude Opus, Claude Sonnet, GPT-OSS | 0.00%, refreshes in 75 h | disabled while the weekly limit is hit |
| `GEMINI 4 ARGON` | Gemini 4 Argon only | 98.62%, refreshes in 167 h | none shown |

The screen also says: "For paid Pro plans, Argon quota is separate." It says that agy charges quota "proportionally to the cost of the tokens", so a cheaper model lasts longer.

Two more facts came from the same screen:

- **U1:** Argon had quota again. The wall of T3 was over when this screen was read.
- **U2:** agy's saved default model, "Gemini 3.5 Flash (Medium)", no longer exists. agy then used "Gemini 4 Argon (Medium)" with only a warning. So each agy launch without a model, such as a probe, uses quota from the Argon pool.

The research file holds the screen and these facts. The screen and its quotes are P3.1 and P3.2. U1 is the last implication of the screen, and U2 is P3.3. The reader findings are P3.4. All are in [research §3](../research/model-tier-policy-2026-10.md#3-what-agy-says-about-its-quota).

### Gap

The bridge already reads `/usage` (`controls.usage.windows` in `go/internal/bridge/manifests/agy-tmux.json`). But the reader was made for the agy 1.3.0 screen, and three of its assumptions are now false:

| Reader assumption | The agy 1.4.0 screen | Result |
|---|---|---|
| A group title ends in `MODELS` (`section_regex`) | The third group title is `GEMINI 4 ARGON` | The Argon pool is not read. |
| A window label is `Weekly Limit Remaining` | The Argon label is `Gemini 4 Argon Weekly Limit` | The Argon percentage is not read. |
| Each group maps to one routing family (`scopes`) | `GEMINI MODELS` and `GEMINI 4 ARGON` are two pools of the one family `agy` | The `scopes` map has no `GEMINI 4 ARGON` entry. A scope with `per_model: true` already makes a window name its model (`go/internal/quotastate/windows.go:41-44`). But no scope sets it, and the bench does not use it yet (T4). |

After the `CLAUDE AND GPT MODELS` group, the reader keeps that group's scope for the lines that follow (`windows.go:175-190`). So it can read the Argon percentage into the Claude window.

On 2026-10-17, `evolve clihealth usage agy` reported "no usage window read" for agy. The cause is not known yet. The reader can also miss the screen if the `/usage` panel hides the prompt marker that the reader waits for.

### Solution

The quota pool becomes the unit of the bench and of the chain order, and the bridge learns the pools from `/usage`. Components Q1–Q4 in §8 do this.

1. **Read every group.** The reader takes any group title, not only `…MODELS`, and any label with "Weekly Limit" or "Five Hour Limit". A fixture of the 1.4.0 screen above pins the reader.
2. **Map a group to a pool, not to a family.** The reader already has the tool: a scope with `per_model: true`. The manifest adds a `GEMINI 4 ARGON` scope with family `agy` and `per_model: true`.
3. **Bench a pool.** A quota wall benches the pool of the model that hit it. A dispatch to another pool of the same family still runs. This is the fix of inbox `agy-model-429-benches-whole-family`.
4. **Order by headroom.** The chain rule of §5 uses the remaining percent from `/usage`. Inside one tier, the router tries the family with the most weekly headroom first. It chooses only among the families that the table allows. The choice of a model inside a family stays with the catalog. The floor seats keep Claude Code only.
5. **Give agy a valid default model.** Each agy launch passes `--model`. A launch without a tier gets the `fast` model of the agy manifest, not agy's saved default. Thus a probe uses quota from the Flash pool, not the Argon pool.

**Why the screen and not the 429 text.** The 429 text names one model and one reset, and only after the wall. The screen gives every pool, its headroom and its reset before the wall. A router can avoid a wall with the screen. With the 429 text it can only react to one.

## 6b. /model and /usage together: what is new, and how much of it we can use

### Issue

The operator asked: "/model and /usage should be used together to learn the latest model published and how much quota can be leveraged." Today each screen feeds its own system, and no view joins them.

- `/model` feeds the model catalog (`evolve models refresh`). It learns the models and maps them to tiers. Its refresh runs in shadow (`catalog.refresh_stage`), so the live catalog keeps its old models until an operator refreshes it. The shadow catalog first listed Gemini 4 Argon in its fetch of 2026-10-15T23:07Z. The live catalog still named Gemini 3.1 Pro on 2026-10-16.
- `/usage` feeds the quota windows (`evolve clihealth usage`). It learns the headroom of each pool, but it does not know which tier each pool serves.

### Gap

A new model is useful only if its pool has quota. A pool with quota is useful only if the tiers use its models. With two separate views, an operator must join them by hand. The router cannot join them at all.

### Solution

One joined view, `evolve models capacity`, reads the catalog and the usage windows of every family. It writes one row for each pool:

| Column | Source | Example on 2026-10-17 |
|---|---|---|
| family and pool | `/usage` group | `agy` · `GEMINI 4 ARGON` |
| models in the pool | `/usage` "Models within this group", or the pool name | Gemini 4 Argon |
| tiers that use the pool | catalog tier map (`/model`) | `deep`, `top` |
| new since the last live catalog | the shadow catalog against the live catalog | yes: the shadow catalog first listed Argon on 2026-10-15T23:07Z, and no live tier used it |
| weekly and 5-hour headroom | `/usage` windows | 98.62%, no 5-hour window |
| reset | `/usage` "Refreshes in" | 167 h |

The view gives three signals. The boundary runs it, and each signal goes to the wave notes:

1. **New model, no tier.** The shadow catalog has a model that no live tier uses. The operator then decides on a refresh or a pin (§6).
2. **Tier on an empty pool.** A live tier maps to a model whose pool has less than `capacity.empty_below_pct` weekly headroom (start value: 5). The router then skips it (Q2, Q3), and the note says why.
3. **Unused headroom.** A pool has more than `capacity.idle_above_pct` weekly headroom (start value: 50). No live tier used it in the last wave. The operator then reviews the chain order, as for Argon on 2026-10-17.

Both values are start values in `.evolve/policy.json`, not in code. Tune them from the wave-note telemetry (P5).

**Why join them at the boundary.** Each screen needs a pane and a few seconds. The boundary already refreshes the catalog in shadow and probes the CLIs, so one more read there costs little. A read in each dispatch adds that time to every phase.

**Where it plugs in.** Q5 adds one read-only step to `evolve wave next`, after the catalog shadow refresh and before the wave goal is written. The step appends the three signals to the wave notes. It never fails the wave, and it changes no cycle state.

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
| D9 | The quota pool from `/usage` is the unit of the bench and of the chain order. | §6a |
| D10 | Every agy launch passes `--model`. A launch without a tier gets the `fast` model of the agy manifest (today `Gemini 3.8 Flash (Low)`). | U2 |
| D11 | One joined view, `evolve models capacity`, reads `/model` (the catalog) and `/usage` (the windows). A read-only step of `wave next` runs it and writes three signals to the wave notes. Its thresholds are policy keys. | §6b |

## 8. Components and order

| Id | Component | Red test first | Depends on |
|---|---|---|---|
| P1 | D4: the manifest maps agy `top` to `Gemini 4 Argon (Medium)`, and the pin test follows | `TestAgyTierModelsPinnedToVerifiedNames` expects Medium at `top` | — |
| P2 | D3 and D8: a `catalog.tier_pins` field in the strict policy schema, a `TierPins` field on `RefreshDeps`, and the pin step in `liveTiers`. **Open: item `model-tier-policy-p2-catalog-tier-pins`.** | Red tests: a refresh with a pin returns the pinned model. A pin to an absent model makes `Refresh` return an error. A pin applies when the prior is reusable. A changed pin changes the fingerprint. | — |
| Q1 | the `/usage` reader reads every agy 1.4.0 group and label, and finds why `clihealth usage agy` read no window. **Built: commit `105281615`.** | a fixture of the 1.4.0 screen gives three pools with 91.90, 0.00 and 98.62 | — |
| Q2 | D9: a `GEMINI 4 ARGON` scope with `per_model: true`, and a bench that honours a model-scoped window. This is the fix of inbox `agy-model-429-benches-whole-family`. **Built: PR #987, commit `716e9bd2a`.** | an Argon wall benches the Argon pool, and a Flash dispatch still runs; the tests of that inbox item | Q1 |
| Q3 | D9: `cliroute.Router.Resolve` orders the families inside one tier by weekly headroom, from an injected usage-window source. The floor seats keep Claude Code only. This amends ADR-0119, whose `Resolve` never re-reads | with `agy-claude` at 0% and `agy` at 98%, a `deep` chain starts with `agy` | Q2, P4 |
| Q4 | D10: a valid model for every agy launch, also for the `agy -p` doctor probe and for a launch whose tier token does not resolve. **Open: item `model-tier-policy-q4-model-on-every-agy-launch`.** | each such launch argv carries the manifest's `fast` model | — |
| Q5 | D11: `evolve models capacity` joins the catalog and the usage windows, one row for each pool, with the three signals. It adds the subcommand to the `evolve models` usage line, the `capacity.empty_below_pct` and `capacity.idle_above_pct` policy keys, and a read-only step in `wave next`. **Open: item `model-tier-policy-q5-models-capacity`.** | a fixture catalog with a new shadow model and a fixture `/usage` screen give the three signals. `wave next --dry-run` lists the step | Q1 |
| P4 | D2 and D5: `agy` returns to `tiers.deep` and `tiers.top` | `evolve cli-routing explain debugger` shows `agy-claude → agy → claude` | P2, Q2 |
| P5 | D6: the advisor rule for `deep` builds | the advisor test: a build with no risk class stays `balanced` | the CI plan's C1, for the measure |

The order is P1, P2, Q4, Q1, Q5, Q2, P4, Q3, P5.
1. P1 and P2 are safe now. `agy` is out of the `deep` and `top` chains (T5), so they change no dispatch.
2. Q4 stops the hidden spend of the Argon pool (U2), so it comes early.
3. Q1 and Q2 (the pool bench) must come before P4, or one Argon wall benches all of agy again (T4).
4. Q5 comes right after Q1, because it needs the reader. It gives the operator the joined view before the router acts on it.
5. Q3 comes after P4, because it orders a chain that holds `agy`.
6. P5 waits for the telemetry, so its effect has a measure.

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
