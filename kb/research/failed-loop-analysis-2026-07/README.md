# Failed-Loop Analysis vs. the Literature — 2026-07-30

Console research pass over batches 17-20 (~55 graded FAILs, 6 ships, 2 honest
halts), cross-checked against current research. Verdict up front: **our
architecture independently converged on the field's main answers** (deterministic
gates outranking LLM judgment, per-phase verification, failure-identity
normalization, bounded flake retries) — and the literature names the next rung
of each ladder we are already climbing.

## 1. Our failure taxonomy (observed, batches 17-20)

| Class | Share | Examples | Standing mitigation |
|---|---|---|---|
| A. Honest task defects | ~55% | gc-from-cwd HIGH, warm-session unreachable, monotonic revert | Convergence machinery: 2-4 generations → ship (proven: 1172→1176→1180) |
| B. Environment-sensitive test flakes | ~20% | seal/lease family (1173/1175/1178 + CI runner), integration-tier tmux | verifylock (P1), retryFlakyReds, red-evidence (#374); queued: deflake 0.96, metapredicate-scope 0.95 |
| C. Infra/timeout aborts | ~15% | exit-81 storms, leaked `yes` load incident | failure adapter, health bench, epilogue cause distinguishers (#376) |
| D. Pipeline/gate defects | ~10% | verify-read race, contract-correction non-escalation, fingerprint identity | console-first fixes (#368/#370/#372-376); queued: verify-race 0.94, cli-escalation 0.9 |
| E. Config/pin drift | rare | profile pin lag (#375), retro worktree guard | advisory gates + boundary reconciliation |

Notable inversion vs. [MAST (NeurIPS 2025, 1,600+ traces)](https://www.augmentcode.com/guides/why-multi-agent-llm-systems-fail-and-how-to-fix-them),
which finds 79% of multi-agent failures are specification/coordination, not
infrastructure: our specification layer (triage cards, acceptance predicates,
continuation briefs) is strong enough that **environment classes (B+C)
dominate our waste** instead. The MAST lens still applies to class D: its
"verification gaps" category is exactly our verify-read race and the judge
calibration story below.

## 2. Literature → what it validates → what it adds

### Flaky tests ([Luo FSE'14 taxonomy](https://dl.acm.org/doi/10.1145/2635868.2635920), [FTW@ICSE'25](https://conf.researchr.org/home/icse-2025/ftw-2025), [multivocal review](https://arxiv.org/pdf/2212.00908))
45% async-wait / 20% concurrency / 12% order-dependency — our seal/lease
family sits squarely in the first two. Validates the bounded-rerun absorber;
adds **authoring-time flakiness prediction** (DeepFlaky 2026 class): flag
flaky-shaped predicate code BEFORE it enters the corpus, not after it burns
cycles.

### Predictive test selection ([Meta, 99.9% regression catch running 1/3 of tests](https://engineering.fb.com/2018/11/21/developer-tools/predictive-test-selection/), [paper](https://arxiv.org/pdf/1810.05286))
Directly validates metapredicate-scope AND goes further: our EGPS runs the
FULL ~200-predicate regression suite every cycle. A deterministic
changed-package selection (we already own `internal/changedpkgs`) with a
periodic full sweep is the proven shape — Meta's numbers say the risk of
selection is small and the cost win is ~3x. For us it is ALSO a flake win:
fewer predicate-seconds under load = fewer class-B reds.

### Crash deduplication ([Igor CCS'21](https://dl.acm.org/doi/10.1145/3460120.3485364), [ECHO LCS-based](https://www.mdpi.com/2079-9292/14/14/2914), [GPTrace embeddings](https://arxiv.org/html/2512.01609))
Stack-hash dedup both over-splits (noise tokens) and over-merges (generic
frames) — the EXACT arc of our fingerprint saga (#368 content-free templates,
#370 duration tokens, #376 constant epilogue reasons). The field's next rung:
**structural similarity (normalized LCS) instead of exact hash** for the
near-identical tier. Our breaker could add an LCS band (e.g. ≥0.9 token
overlap counts as recurrence) — deterministic, Go-native, ECHO-shaped.

### LLM-as-judge calibration ([judge blind spots](https://arxiv.org/pdf/2606.10315), [calibration probes](https://arxiv.org/pdf/2512.22245), [one-token attacks](https://arxiv.org/pdf/2507.08794))
Judges have LOW error-detection recall and systematic overconfidence — our
"gate outranks the narrative" policy is precisely the field's conclusion.
Better: our verdict-conflict records BOTH readings per cycle, which IS a
calibration dataset the literature says to exploit. We have months of
(narrative, gate) pairs in dossiers — an empirical auditor
sensitivity/specificity table is one Go tool away, and it tells us whether
the auditor's WARN threshold needs a persona rubric adjustment.

### Learning from failure ([inference-time self-improvement](https://arxiv.org/html/2606.31270v1), [recursive self-improvement survey](https://arxiv.org/pdf/2607.07663))
Our continuation chain (preserved worktree + served findings) is
trajectory-level learning within a task. The literature's add: **structured
failure-mode libraries reused ACROSS tasks** — an LLM meta-controller
organizing failed trajectories into reusable behavioral rules. Our recurrence
ledger + instincts are the substrate; the missing move is projecting the
month's top failure modes INTO the scout/builder briefs (the queued
sleep-time-kb-consolidation item is the natural vehicle).

### Compound reliability ([0.99^10 ≈ 90.4%](https://www.zartis.com/the-compounding-errors-problem-why-multi-agent-systems-fail-and-the-architecture-that-fixes-it/))
Validates per-phase gates + single-writer triage; no change needed.

## 3. Improvements queued from this review

| Item | Weight | Source |
|---|---|---|
| `egps-regression-tia-selection` | 0.88 | Meta PTS — changed-package selection + boundary full-sweep; token AND flake win |
| `breaker-fingerprint-similarity-tier` | 0.86 | Igor/ECHO — LCS near-identical band on the identical-fingerprint rule |
| `auditor-calibration-report` | 0.84 | judge-calibration literature — mine dossier (narrative, gate) pairs |
| deflake item note: Luo taxonomy mapping + authoring-time flaky-pattern lint | (0.96 existing) | FSE'14 + DeepFlaky class |

## Sources

- https://dl.acm.org/doi/10.1145/2635868.2635920 · https://arxiv.org/pdf/2212.00908 · https://conf.researchr.org/home/icse-2025/ftw-2025
- https://engineering.fb.com/2018/11/21/developer-tools/predictive-test-selection/ · https://arxiv.org/pdf/1810.05286
- https://dl.acm.org/doi/10.1145/3460120.3485364 · https://www.mdpi.com/2079-9292/14/14/2914 · https://arxiv.org/html/2512.01609
- https://arxiv.org/pdf/2606.10315 · https://arxiv.org/pdf/2512.22245 · https://arxiv.org/pdf/2507.08794
- https://arxiv.org/html/2606.31270v1 · https://arxiv.org/pdf/2607.07663
- https://www.augmentcode.com/guides/why-multi-agent-llm-systems-fail-and-how-to-fix-them · https://www.zartis.com/the-compounding-errors-problem-why-multi-agent-systems-fail-and-the-architecture-that-fixes-it/
