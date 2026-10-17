# Model tiers and quota pools: what each CLI offers, how its quota is counted, and how the loop uses it (research, 2026-10)

> The plan that uses this research: [model-tier-policy-2026-10](../plans/model-tier-policy-2026-10.md). The design: [model-routing](../reference/model-routing.md), section "The tier policy". The decision record: [ADR-0136](../architecture/adr/0136-each-model-tier-walks-separate-quota-pools-and-agy-runs-argon-medium-at-deep-and-top.md).

Date: 2026-10-17. Method:

- a count of the tier defaults of the phase profiles;
- the CLI and tier of each phase in cycles 1879–1914, from the run directories;
- the agy `/model` list, the agy `/usage` screen and a live launch of each Gemini 4 Argon variant;
- a read of the catalog refresh and the policy pin code;
- the bench records of `.evolve/cli-health.json`.

Each claim names its command or file. "Synthesis" marks our own inference.

## Table of contents

- [1. The request](#1-the-request)
- [2. Local evidence](#2-local-evidence)
- [3. What agy says about its quota](#3-what-agy-says-about-its-quota)
- [4. Adopt or reject](#4-adopt-or-reject)
- [5. Refinements to adopt](#5-refinements-to-adopt)

## 1. The request

On 2026-10-17 the operator asked:

- "Make sure we leverage token usage from agy".
- "let's also include the gemini top / deep model as Gemini 4 Argon with medium effort. I would let you design the optimized policy for different usage (deep, fast, balance, etc...)".
- "Agy gemini 4 argon has its own quota, use the /usage to learn from the different LLM limitations and separated quotas through agent bridge".
- "/model and /usage should be used together to learn the latest model published and how much quota can be leveraged."

## 2. Local evidence

**T1** On main, 67 tracked phase profiles default to `balanced` and 26 to `deep`. None defaults to `fast` or `top`. The runtime plane adds 8 minted profiles, for 72 and 29 in all (a count of `model_tier_default` over `.evolve/profiles/*.json`, 2026-10-17).
*Implication:* `fast` and `top` carry no work today. A change to their chain has no effect until a profile uses them.

**T2** The advisor raised the build of 8 of 17 recent cycles to `deep` (`phase-plan.json` of cycles 1879–1914).
*Implication:* the profile says `balanced` for the builder, but half of the builds run at `deep`.

**T2b** 11 of 18 build dispatches in cycles 1879–1896 ran on Claude Code, and 7 ran on agy (the `cli` field of `build-events.ndjson`).
*Implication:* the `deep` builds of T2 are the builds on Claude Code, because the `deep` chain had no agy entry with quota.

**T3** On 2026-10-17 at 02:17 (+08:00), agy answered Gemini 4 Argon with HTTP 429: "Individual quota reached … Resets in 148h8m53s". Gemini 3.8 Flash still answered `OK` to the same probe (`agy -p … --model …`).
*Implication:* Argon and Flash use separate quota in agy.

**T4** That one Argon wall benched the whole `agy` family for 30 minutes. Wave 111 started inside that window. Every phase of cycles 1907 and 1908 ran on Claude Code (`.evolve/cli-health.json`; the `cli` fields of the run directories).
*Implication:* the bench counts the family, while agy counts its quota per pool.

**T5** PR #910 took `agy` out of `cli_routing.tiers.deep` and `.top` again, after T4 (`.evolve/policy.json` on main).

**T6** agy has no effort flag. The effort is part of the model name, for example `Gemini 4 Argon (Medium)`. agy-claude also takes its effort as a name suffix, and Claude Code takes `--effort` (`docs/reference/model-routing.md`, "Effort for each tier").
*Implication:* "Argon at medium effort" means the model name `Gemini 4 Argon (Medium)`.

**T7** The live catalog picks the model of each tier through a classifier, and it overrides the manifest (`go/internal/modelquery/query.go:87-117`; the resolution order "policy pin > live catalog > manifest baseline" in CLAUDE.md). On 2026-10-17 it picked `Gemini 4 Argon (High)` for agy `top`.
*Implication:* a manifest change alone cannot hold Argon at Medium.

**T8** No setting can fix the model of one tier for one family. A pin names a phase, not a tier, and a `pins` key beside the routing table refuses every launch (`go/internal/policy/core.go:104-139`).
*Implication:* the plan needs a new, small mechanism.

**T9** Three builds passed the Claude audit and shipped. One ran on Gemini 4 Argon (cycle 1899). Two ran on Gemini 3.8 Flash (cycles 1900 and 1902). The records are those of waves 109 and 110.
*Implication:* the Gemini models can carry builds at the bar of the audit.

**T10** Claude Code was at 52% of its weekly quota on 2026-10-17 (`/usage` of Claude Code). The floor seats need it in every cycle.

**T11** The operator directive of 2026-10-08 starts every effort at medium and raises it only on a measured gain (`docs/reference/model-routing.md`).

**T12** Both Argon variants boot. A launch with `--model "Gemini 4 Argon (Medium)"` showed the footer `Gemini 4 Argon · medium`, and the High variant showed `Gemini 4 Argon · high` (a launch of agy 1.4.0 in a tmux pane, 2026-10-16).
*Implication:* the name is valid. agy silently runs its default model on an invalid name, so this check is necessary.

**T13** The shadow catalog first listed Gemini 4 Argon in its fetch of 2026-10-15T23:07Z. The live catalog still named Gemini 3.1 Pro on 2026-10-16, because `catalog.refresh_stage` is `shadow` (`.evolve/model-catalog.shadow.json`, `.evolve/model-catalog.json`, `.evolve/policy.json`).
*Implication:* the loop learns a new model, but no live tier uses it until an operator acts.

## 3. What agy says about its quota

agy 1.4.0 showed this `/usage` screen on 2026-10-17:

| Pool (agy group) | Models in the pool | Weekly left | 5-hour left |
|---|---|---|---|
| `GEMINI MODELS` | Gemini Flash, Gemini Pro | 91.90%, refreshes in 165 h | 73.66%, refreshes in 2 h 21 m |
| `CLAUDE AND GPT MODELS` | Claude Opus, Claude Sonnet, GPT-OSS | 0.00%, refreshes in 75 h | disabled while the weekly limit is hit |
| `GEMINI 4 ARGON` | Gemini 4 Argon only | 98.62%, refreshes in 167 h | none shown |

**P3.1** The screen says: "For paid Pro plans, Argon quota is separate."

**P3.2** The screen says: "Within each group, models share a weekly limit and a 5-hour limit. Quota is consumed proportionally to the cost of the tokens."

**P3.3** On launch, agy said: "'Gemini 3.5 Flash (Medium)' is no longer available. Using 'Gemini 4 Argon (Medium)'." This is agy's saved default model.

*Implications (synthesis):*
- agy has three pools: Flash and Pro share one, Claude and GPT share one, and Argon has its own.
- A cheaper model uses less of its pool for the same task.
- Each agy launch without `--model` uses quota from the Argon pool, because the saved default is gone.
- The Argon wall of T3 was over when this screen was read.

**P3.5** Online sources, read on 2026-10-17, agree on two of the three pools. Google's plans page says that the Pro quota is "refreshed every five hours until the weekly limit is reached" ([Antigravity plans](https://antigravity.google/docs/plans/)).

A guide says that the usage screen shows the Gemini group apart from the Claude and GPT group ([SmartScope](https://smartscope.blog/en/blog/antigravity-claude-quota-five-hour-weekly-limits-2026/)). It also says that an exhausted weekly limit does not reset after five hours. A second guide says that Flash and Pro share one quota ([sanj.dev](https://sanj.dev/post/google-antigravity-quota-problems-fix/)). Google publishes no numeric quota ([Layer3 Labs](https://www.layer3labs.io/guides/google-antigravity-pricing)).
*Implication:* no online source documents the separate Argon pool, and no source gives numbers. So the loop must read the pools from the live `/usage` screen, not from a fixed table.

**P3.6** Current practice for LLM gateways routes on live signals, remaining quota among them. A provider at its token ceiling gets no more traffic, even while its endpoint answers ([Maxim, five routing strategies](https://www.getmaxim.ai/articles/5-llm-routing-strategies-every-ai-gateway-needs-in-2026/), [Gravitee, routing and fallback](https://gravitee.io/corpus/gen-1223/electric-on-the-eel/model-routing-and-fallback-strategies-for-multi-provider-llm-gateways.html)).

The same sources separate capacity failures (429, an exhausted quota) from transport failures. They warn that retries multiply the tokens spent. A fallback must pass the same policy checks as the first choice.
*Implication:* the pool bench and the headroom order match current practice. The floor-seat rule is the policy check that each fallback must pass.

**P3.4** The bridge already reads `/usage` through `controls.usage.windows` in `go/internal/bridge/manifests/agy-tmux.json`. The reader was made for the agy 1.3.0 screen:

- Its `section_regex` takes a group title that ends in `MODELS`. The Argon title is `GEMINI 4 ARGON`.
- Its labels are exactly `Weekly Limit Remaining` and `Five Hour Limit Remaining`. The Argon label is `Gemini 4 Argon Weekly Limit`.
- After the `CLAUDE AND GPT MODELS` group, the reader keeps that group's scope for the lines that follow (`go/internal/quotastate/windows.go:175-190`). So it can read the Argon value into the Claude window.
- A scope with `per_model: true` already makes a window name its model (`windows.go:41-44`), but no scope sets it.

## 4. Adopt or reject

| Practice | Decision | Reason |
|---|---|---|
| Read the pools from agy's own `/usage` screen | Adopt | P3.1, P3.2: the screen names each pool, its headroom and its reset before a wall. A 429 names one model, after the wall. |
| Bench a pool, not a family | Adopt | T3, T4, P3.1. |
| Hold Argon at Medium with a catalog tier pin | Adopt | T6–T8, T12. |
| Raise Argon to High at `top` | Reject for now | T11, and P3.2: High costs more tokens, so it uses the Argon pool faster. |
| Move `balanced` builds off Gemini | Reject | T9: Gemini builds pass the audit. |
| Join `/model` and `/usage` in one view | Adopt | T13, P3.2: a new model is useful only when its pool has quota. |
| Route on live quota signals, as LLM gateways do | Adopt | P3.6 |
| A fixed quota table for each plan | Reject | P3.5: Google publishes no numbers, and the Argon pool is not documented online. |

## 5. Refinements to adopt

- **R1** (T3, T4, P3.1, P3.4): add a `GEMINI 4 ARGON` scope with `per_model: true`, accept every group title and limit label, and bench the pool of a wall.
- **R2** (T6–T8, T12): a catalog tier pin holds `Gemini 4 Argon (Medium)` for agy `deep` and `top`. A pin to an absent model fails the refresh.
- **R3** (P3.3): every agy launch passes `--model`, so a probe uses the Flash pool.
- **R4** (T2, T2b, T10): the advisor raises a build to `deep` only for named risk classes. The Claude pools then stay for the work that needs them.
- **R5** (T13, P3.2): one boundary view joins the catalog and the usage windows, and writes signals to the wave notes.
