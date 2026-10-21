# Deep tier share: how the deep seats split between Argon and Claude (research, 2026-10)

> The plan that uses this research: [deep-tier-share-2026-10](../plans/deep-tier-share-2026-10.md). The design: [deep-tier-share](../architecture/deep-tier-share.md). The decision record: [ADR-0150](../architecture/adr/0150-the-deep-tier-gives-its-first-family-a-fixed-share-of-the-seats-and-paces-it-by-its-week-window.md).

Date: 2026-10-21. Method: a read of the routing code and the checked-in policy, the usage record of 2026-10-17, and a live web search on 2026-10-21. Each local claim names its file. "Synthesis" marks our own inference.

## Table of contents

1. [Local evidence](#1-local-evidence)
2. [Current practice](#2-current-practice)
3. [Adopt or reject](#3-adopt-or-reject)
4. [Refinements to adopt](#4-refinements-to-adopt)

## 1. Local evidence

**F1.1** The deep tier lists `agy`, `agy-claude` and `claude` in that order (`.evolve/policy.json`, `cli_routing.tiers.deep`). The agy deep seat runs Gemini 4 Argon (Medium) (`go/internal/bridge/manifests/agy-tmux.json`, `model_tier_map.deep`).
*Implication:* the first family of the tier takes every deep seat until its quota stops it.

**F1.2** On 2026-10-17 at 22:15 (Asia/Taipei), `evolve clihealth usage agy` showed the Argon week at 100%, with 153 hours to the reset. Claude Code was at 67% of its week (inbox item `deep-tier-shares-argon-by-budget`).
*Implication:* the order spent the Argon week in about four waves, and Claude then carried all deep work for six days.

**F1.3** The headroom order (`go/internal/cliroute/headroom.go`) sorts the chain of one agent by the week of each family. It reads `usageprobe.WeekUsed`, which skips each per-model window (`go/internal/usageprobe/week_used.go`).
*Implication:* no current stage reads the Argon window, and no stage splits the seats of a tier.

**F1.4** The usage probe records the Argon window as its own week window, with `family: agy` and `model: Gemini 4 Argon` (`agy-tmux.json`, `controls.usage.windows.scopes`). Each window has `percent_used` and `resets_at` (`go/internal/quotastate/windows.go`).
*Implication:* the data for a pace already exists. The router needs a reader for it.

**F1.5** One table decides each dispatch, and the resolution is deterministic (ADR-0119). A route request carries the agent, the phase and the environment, but not the cycle (`go/internal/cliroute/cliroute.go`, `Request`).
*Implication:* a share needs a stable key for each dispatch. The cycle is a natural key, and the runner already has it (`core.PhaseRequest.Cycle`).

**F1.6** An exhausted Argon window benches only the Argon target since cycle 1911. The walk then falls through to the next family (`docs/architecture/packages/internal-cliroute.md`).
*Implication:* the share must keep the walk. A seat that the share gives away keeps the shared family as a fallback at the end of the chain.

## 2. Current practice

**P2.1** Ad delivery paces a budget with a throttle. The system compares the spend with a target curve. When the spend runs ahead, it lowers the chance to take part ([A Practical Guide to Budget Pacing Algorithms, arXiv 2503.06942](https://arxiv.org/pdf/2503.06942); [Mystique, Yahoo](https://yahooinc.com/research/publications/mystique-a-budget-pacing-system-for-performance-optimization-in-online-advertising)).
*Implication:* a pace compares the used part of the week with the part of the week that is gone.

**P2.2** Hard throttling uses a weighted coin for each auction. Soft throttling scales the bid ([Budget Pacing, machinelearningatscale.com](https://machinelearningatscale.com/ads-systems-at-scale/budget-pacing/); [Smart Pacing, arXiv 1506.05851](https://arxiv.org/pdf/1506.05851)).
*Implication:* the router has no bid, so only the hard form applies: take the seat or give it away.

**P2.3** A traffic split hashes a stable key into 100 buckets and sends the bucket below the percent to the canary. It keeps no state ([split_clients, Angie](https://angie.software/en/configuration/modules/http/http_split_clients); [traffic splitting, Gravitee](https://www.gravitee.io/traffic-splitting.html)).
*Implication:* a hash of the cycle gives a deterministic share with no stored counter.

**P2.4** Smooth weighted round-robin interleaves picks by weight with a counter for each peer ([nginx commit](https://nginx.googlesource.com/nginx/+/52327e0627f49dbda1e8db695e63a4b0af4448b1)).
*Implication:* it gives an exact share, but each pick changes the counters, so the state must persist across processes.

**P2.5** A token bucket limits the long-run rate with a refill rate and allows a burst up to the bucket size ([token bucket, Gravitee](https://www.gravitee.io/token-bucket-basics.html)).
*Implication:* a bucket needs a count of the tokens that each dispatch spends. The router does not know the token cost of a dispatch before it runs.

**P2.6** LLM gateways route by weight across providers and keys, and fall back to the next provider on a 429 ([5 LLM routing strategies, Maxim](https://www.getmaxim.ai/articles/5-llm-routing-strategies-every-ai-gateway-needs-in-2026/)).
*Implication:* a weighted split in front of a fallback chain is common practice. The source is a vendor blog, so we use it only for the pattern.

## 3. Adopt or reject

| Practice | Decision | Reason |
|---|---|---|
| A hash split of a stable key (P2.3) | Adopt | It is deterministic and needs no state (F1.5). |
| A pace against the elapsed week (P2.1, P2.2) | Adopt | The usage record holds the used percent and the reset (F1.4). |
| Smooth weighted round-robin (P2.4) | Reject | Its counters need a shared store across the lanes of a wave. |
| A token bucket (P2.5) | Reject | The router does not know the cost of a dispatch before the launch. |
| A share that the advisor sets per phase | Reject for now | It moves the decision into a prompt (AGENTS.md rule 5). The table holds the share as data. |

## 4. Refinements to adopt

- **R1** The share is a percent on the tier rule: `cli_routing.tiers.<tier>.share_pct`. The first family of the tier chain takes that share of the seats (F1.1, P2.3).
- **R2** The key of a seat is the cycle, the agent and the phase, hashed into 100 buckets (F1.5, P2.3).
- **R3** The pace reads the week window of the tier model of the family. When the used percent is above the percent of the week that is gone, the family gives the seat away (F1.4, P2.1).
- **R4** When the share gives a seat away, the shared family moves to the end of the chain. The walk keeps it as a fallback (F1.6).
- **R5** `evolve cli-routing explain` prints the share and the pace for each agent whose chain holds the shared family (inbox item acceptance).
