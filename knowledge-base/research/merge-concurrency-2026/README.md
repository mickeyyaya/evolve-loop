# Merge concurrency under multi-lane fleets — research synthesis (2026-07-13)

Two-agent web sweep (industrial merge queues + AI-agent fleet practice) commissioned to reduce failed cycles from concurrent lanes landing on main. Drives the four-rung merge ladder, campaign `merge-efficiency-2026-07`: `merge-rung0-trivial-rebase-carryforward` (0.98) → `merge-rung1-package-graph-disjoint` (0.97) → `merge-rung2-scoped-merge-review` (0.95) → rung 3 = today's full re-audit (no new work), plus the landing layer `prefix-speculation-landing-queue` (0.93). `ship-window-lease` (0.89) is the demoted interim.

## The five findings that changed our design

1. **Nobody re-reviews after a base move; everybody re-verifies.** Review verdicts follow the *change identity* (`git patch-id`), mechanical gates follow the *tree*. Gerrit codifies it: `TRIVIAL_REBASE` (conflict-free rebase, byte-identical diff) carries review votes forward **by default**; only `REWORK` re-reviews — and then only the delta. Our AUDIT_BINDING_HEAD_MOVED → full-re-audit recovery re-answered a question no one in industry re-asks.
2. **Disjointness must be build-graph reachability, never file paths.** Uber SubmitQueue (EuroSys'19: "two changes conflict iff they affect a common set of build targets", transitive) and Aviator affected-targets both land target-disjoint changes in parallel with no cross-verification; Google TAP's whole presubmit rests on Blaze reachability. File-level disjointness is explicitly insufficient (merge skew: rename + call-site). For Go: transitive package import graph via `go list`, with a global-zone list (go.mod, policy, hooks, generated files) treated as always-conflicting.
3. **At 3-10 lanes, prefix speculation beats batch-then-bisect.** Zuul verifies queue prefixes (L1, L1+L2, …) so the first failing prefix names the culprit positionally — bisection machinery is only needed at Bors/TAP batch scale. Batch size should not be a constant: Zuul's AIMD window (+1 per green, ×0.5 per red, floor) is the cleanest published answer.
4. **Flake policy is a precondition for any optimism.** Chromium: rerun the failing test *without* the patch; still fails ⇒ blame base, don't eject the lane ("retries mitigate a flake's impact on unrelated CLs exponentially"). Aviator: sibling-speculation agreement ⇒ treat as flake.
5. **Conflict avoidance belongs at dispatch.** Measured on 142K agent PRs (AgenticFlict, arXiv 2604.03551): 27.7% overall conflict rate; ~9.9% at 2-line median churn vs ~30% at 25 lines — **churn size is the dominant lever**. Fleet practice: pinned wave base SHA, real package leases checked pairwise at dispatch, hot-file quarantine (registries/changelogs/manifests → single writer or regenerate-at-land), one-file-one-owner (Claude Code Agent Teams ship file locking as a primitive).

## What not to import

- **Semantic merge as verification** — no production queue uses it; mergiraf-class structural merge is a fine deterministic *resolution assist*, but output must still pass full gates. LLM resolvers are suggestion-grade (MergeBERT lineage 63-68%, peer-reviewed; AgentSpawn's 73% semantic-merge figure is from a single-author preprint whose own validation is listed as future work — treat as illustrative, not load-bearing).
- **ML success prediction** (Uber's 97% logistic model) — needs history volume a 3-10 lane fleet lacks.
- **Full-restart-behind-failure without verdict carry-forward** (GitHub queue default) — reproduces our exact pain.

## Scale ladder (where practices apply)

| Scale | Practice |
|---|---|
| <10 changes/day | Plain serialization (bors "not rocket science") is affordable |
| 10-100/day | Prefix-speculative queue + AIMD window ← **our regime** |
| 100s-1000s/day | Portfolio verification: presubmit subset + postsubmit batch + culprit finder + first-class reverts (Google/Meta/Uber/Chromium) |

## Key sources

Uber SubmitQueue (dl.acm.org/doi/10.1145/3302424.3303970) · Zuul gating (zuul-ci.org/docs/zuul/latest/gating.html) · Gerrit copyCondition (gerrit-review.googlesource.com/Documentation/config-labels.html) · bors-ng (github.com/bors-ng/bors-ng) · Rust rollups (forge.rust-lang.org/release/rollups.html) · Chromium CQ (chromium.googlesource.com docs/infra/cq.md) · SWE-at-Google ch.23 TAP (abseil.io/resources/swe-book) · Meta Predictive Test Selection (arxiv.org/abs/1810.05286) · Aviator affected-targets (docs.aviator.co/mergequeue/affected-targets) · Mergify speculative checks (articles.mergify.com) · GitLab merge trains (docs.gitlab.com/ci/pipelines/merge_trains/) · AgenticFlict (arxiv.org/abs/2604.03551) · AgentSpawn (arxiv.org/abs/2602.07072) · Specification Gap (arxiv.org/pdf/2603.24284) · MergeBERT (arxiv.org/abs/2109.00084) · mergiraf (mergiraf.org) · Cognition multi-agents (cognition.com/blog/multi-agents-working) · Autonoma parallel agent PRs (getautonoma.com/blog/parallel-ai-agent-prs).

*Full agent reports available in the 2026-07-13 operator session transcript; this README is the durable synthesis.*
