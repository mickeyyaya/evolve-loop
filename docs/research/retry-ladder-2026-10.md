# Research: when a retry gets a stronger model or more effort

- **Status:** complete, 2026-10-19.
- **Plan:** [retry-ladder-2026-10](../plans/retry-ladder-2026-10.md). **Decision:** [ADR-0142](../architecture/adr/0142-a-capability-failure-climbs-one-bounded-retry-ladder.md). **Design:** [retry-ladder](../architecture/retry-ladder.md).
- **Method:**
  - a read of the Go code of the runtime plane at `origin/main` 6a7d6d97f;
  - the run artifacts of cycles 1800 to 1970 (`llm-calls.ndjson`, `*-interactions.ndjson`, `signals.ndjson`);
  - about 25 live web searches on 2026-10-19.

## Table of contents

- [1. The request](#1-the-request)
- [2. Local evidence](#2-local-evidence)
- [3. Outside survey](#3-outside-survey)
- [4. Adopt or reject](#4-adopt-or-reject)
- [5. Limits of this research](#5-limits-of-this-research)

## Terms used throughout

- **Try:** one dispatch of a phase agent for one deliverable. A correction round is a try.
- **Rung:** the resources of one try: the context (fresh or kept), the effort, the tier and the CLI family.
- **Tier:** the model class of a dispatch: fast, balanced, deep or top (`internal/policy/core.go`).
- **Effort:** the reasoning level of a dispatch: low, medium, high, xhigh or max (`internal/policy/dispatch.go`).
- **Capability failure:** the request is clear and possible, and the agent did not meet it.
- **Spec failure:** the request is wrong, impossible or outside the scope of the lane. A stronger model cannot fix it.
- **Fingerprint:** a normalized form of the first failed check of a try. Two tries with the same fingerprint failed in the same way.

## 1. The request

The operator asked on 2026-10-19: "could we design a more dynamic retry, said if the cycle failed because agent cannot get the request and deliverables right, could we move this phase agent to the next tier model (e.g. sonnet upgrade to opus, or balanced tier model to deep, for example) or bump up the effort to the next level (medium to high for example)".

## 2. Local evidence

### The loop already has three raises, and the policy turns two of them off

**F2.1** The audit repair round raises tdd and build to the profile tier `model_tier_overrides.audit_retry_2plus`, which is `deep` (`go/internal/core/retry_tier_escalation.go:84`, `.evolve/profiles/builder.json:13`). This is ADR-0096.
*Implication:* a tier raise on retry is an accepted design. This research does not have to argue for it from zero.

**F2.2** An item with `failure_count >= 1` starts its next build at deep (`go/internal/core/retry_tier_escalation.go:31`, `BuildDeepEscalateAtFailures: 1` at `go/internal/policy/policy_failure.go:68`). This is ADR-0076 slice D.
*Implication:* the loop also raises the tier across cycles.

**F2.3** The policy sets `cli_routing.agents.builder.model` and `agents.tdd-engineer.model` to `balanced` (PR #981, 2026-10-18). `fixAgentModel` then marks the tier as fixed (`go/internal/cliroute/resolve.go:70`), and `overlayTier` drops every tier overlay with "advisor tier %s kept off: agents.%s.model fixes the tier" (`go/internal/cliroute/resolve.go:126`).
*Implication:* the pin of #981 turns off the raises of F2.1 and F2.2 for build and tdd. The pin had one purpose: keep the first build try on Gemini 3.8 Flash. It removed the retry raise as a side effect.

**F2.4** The contract-gate correction ladder changes only the CLI family. On the second identical block it sets `ModelRoutingCLI` to a CLI of another family (`go/internal/core/cyclerun_correction.go:111`, `contractEscalateAtBlock = 2` at `go/internal/core/contract_escalation.go:20`). It resets the CLI after the loop (`go/internal/core/cyclerun_correction.go:202`). The tier and the effort never change between correction rounds.
*Implication:* the most frequent retry in the loop has no tier or effort axis.

**F2.5** Effort has no per-try field. `PhaseRequest` carries `ModelRoutingCLI` and `ModelRoutingTier` only (`go/internal/core/phase.go:102`). `bridge.LaunchEffort` derives the effort from the tier at launch (`go/internal/bridge/profile.go:26`).
*Implication:* "the same tier with more effort" cannot be expressed today.

**F2.6** Gate remediation clears both overlay fields before each fix round (`go/internal/core/cyclerun_remediate.go:53`).
*Implication:* a raise that one retry path makes, a second path can undo.

**F2.7** The convergence package already computes a one-tier raise of tier and effort for a fixer and a judge (`go/internal/convergence/decide.go:99`). It uses the tier tables of the family manifests. No pipeline loop calls it. It is the CLI verb `evolve convergence decide` only (ADR-0126, status Proposed).
*Implication:* the raise arithmetic exists. The gap is the wiring and the trigger.

### What the retries did in cycles 1800 to 1970

**F2.8** The run artifacts hold 145 correction re-dispatches in 62 cycles. Build has 59 `accepted`, 65 `rejected_again` and 2 `quota_deferred`. Triage has 17 `accepted`. Measured on 2026-10-19 from `*-interactions.ndjson`.
*Implication:* about half of build correction rounds fail again. The ladder of F2.4 does not fix the build in half of its rounds.

**F2.9** Take each `rejected_again` round and the round after it. The two rounds had the same failure fingerprint in 12 of 44 pairs (27%), and a different one in 32 (73%). The fingerprint was the first 160 characters of the correction payload, with numbers masked.
*Implication:* the same fingerprint is a clear "stuck" signal. A different fingerprint often shows progress, so it calls for a cheaper step.

**F2.10** In 15 cycles the build aborted after its corrections. The cycles are 1804, 1808, 1811, 1814, 1853, 1887, 1928, 1933, 1935, 1938, 1946, 1950, 1958, 1965 and 1966. In cycles 1946, 1950, 1958, 1965 and 1966, every try ran at the balanced tier. Cycles 1933, 1935 and 1938 ran at deep and also aborted.
*Implication:* the recent aborts never tried a stronger tier. A deep tier is not a guarantee, so a ladder needs a stop rule.

**F2.11** The causes of the recent aborts differ in class:

| Cycle | First failed check | Class |
|---|---|---|
| 1946, 1950 | 100% coverage of branches that the lane cannot reach in a protected file | spec |
| 1953 | an acceptance that needs a protected writer the lane may not touch (audit class `intent-malformed`) | spec |
| 1965 | a copied ACS predicate test under `testdata/` that the floor runs (handoff H2) | capability |
| 1966 | the red predicate `TestC1966_002` never turned green | capability |

*Implication:* a stronger model cannot fix a spec failure. The ladder must classify first. Cycles 1946 and 1950 now end at the blocker exit of PR #983, and cycle 1953 has the item `reintent-decision-ends-the-cycle-not-rebuilds`.

**F2.12** The telemetry cannot measure a ladder today. `llm-calls.ndjson` records `effort` as null on every build call. `attempt` stays 1 through the correction rounds. `model` is the tier alias (`balanced`), not the served model.

- In cycle 1966 the CLI moved from agy to claude because agy exited 85 (the quota wall), not because of an escalation.

*Implication:* the first component must record the rung of each try. Without it, no one can tell if a raise helped.

## 3. Outside survey

**F3.1** A cascade that defers from a cheap model to a strong one on a verifier signal keeps quality at a much lower cost. FrugalGPT reports up to 98% cost reduction at GPT-4 quality ([arXiv 2305.05176](https://arxiv.org/abs/2305.05176)). AutoMix uses self-verification plus a meta-verifier and cuts compute by more than 50% ([arXiv 2310.12963](https://arxiv.org/abs/2310.12963)).
*Implication:* "start cheap, raise on a verified failure" is the right shape. Our verifier is deterministic: the build floor and the audit gates.

**F3.2** For code, the result of the tests is the natural gate of a cascade ([arXiv 2405.15842](https://arxiv.org/html/2405.15842v1)).
*Implication:* the trigger is the build floor and the predicate tests, not a self-report of the agent.

**F3.3** Two tiers capture most of the gain. In 118 of 135 comparisons a third model did not help, and it added at most 0.43 points. The gain of depth goes away when the models fail on the same items (Bouchard, [arXiv 2605.06350](https://arxiv.org/abs/2605.06350), v2 2026-10-05).
*Implication:* the ladder has at most one tier step. Two tiers that fail on the same check point to a spec failure.

**F3.4** Self-repair with the same model is limited by the quality of its feedback. Feedback from a stronger model raises the repair rate a lot (Olausson and others, ICLR 2024, [arXiv 2306.09896](https://arxiv.org/abs/2306.09896)).
*Implication:* a structured digest of the failure is part of each rung, not only more compute.

**F3.5** A model that keeps its context loses 60% to 80% of its debugging effect in 2 to 3 tries. A fresh start at the right point raised accuracy at no added compute ([arXiv 2506.18403](https://arxiv.org/abs/2506.18403), preprint).
*Implication:* each raised rung starts a fresh session with the digest. It does not continue the old conversation.

**F3.6** Anthropic recommends to raise effort when the evals show shallow reasoning ([effort docs](https://platform.claude.com/docs/en/build-with-claude/effort), read 2026-10-19). It warns that `max` adds cost for small gains and can overthink. OpenAI says to raise effort only when evals show a gain, and to fix weak success criteria first ([GPT-5.5 guide](https://developers.openai.com/api/docs/guides/gpt-5.5)).
*Implication:* an effort raise is one step at most, never to `max` without the operator, and never a fix for a spec failure.

**F3.7** Longer reasoning can lower accuracy on tasks with distractors or many constraints ("Inverse Scaling in Test-Time Compute", [Anthropic, 2025](https://alignment.anthropic.com/2025/inverse-scaling)).
*Implication:* the effort axis has a ceiling, and the ladder records the outcome of each raise so the ceiling can move on evidence.

**F3.8** Many failures of agent systems come from the specification or the system design, not from the model (MAST, NeurIPS 2025, [arXiv 2503.13657](https://arxiv.org/abs/2503.13657)).
*Implication:* this confirms F2.11. The classifier comes before the ladder.

**F3.9** An executor that asks a strong advisor for a short correction is a vendor pattern. Sonnet with an Opus advisor scored 2.7 points over Sonnet alone on SWE-bench Multilingual at 11.9% lower cost per task ([Anthropic, 2026-04-09](https://claude.com/blog/the-advisor-strategy)).
*Implication:* a deep-tier critic that writes the digest is a candidate rung. It goes through our agent bridge as a short phase, because every model query uses the bridge.

**F3.10** A learned router can choose the first tier from logged outcomes with bandit feedback (BaRP, [arXiv 2510.07429](https://arxiv.org/abs/2510.07429); PILOT, [arXiv 2508.21141](https://arxiv.org/abs/2508.21141)).
*Implication:* this is a later stage. It needs the telemetry of F2.12 first.

## 4. Adopt or reject

| Practice | Decision | Because |
|---|---|---|
| Classify the failure before a raise | adopt | F2.11, F3.8 |
| A deterministic trigger (floor, predicates, audit gates) | adopt | F3.1, F3.2 |
| A fresh session with a failure digest on each raised rung | adopt | F3.4, F3.5 |
| One effort step, then one tier step, then stop | adopt | F3.3, F3.6, F3.7 |
| Skip the effort step when the fingerprint repeats | adopt | F2.9 |
| Stop and reclassify when two tiers fail on the same check | adopt | F3.3 |
| `max` effort or the top tier on retry | reject | F3.6, F3.3 |
| A deep-tier critic rung | defer to a shadow experiment | F3.9; no local measure yet |
| A learned first-tier router | defer | F3.10, F2.12 |
| Raise the auditor on retry | reject | the judge must stay independent of the try it grades (ADR-0141) |

## 5. Limits of this research

- The local counts mix task types and periods. They show where the ladder acts, not how much it helps. The shadow stage measures that.
- The fingerprint of F2.9 is a text prefix. The contract escalation already has a stronger identity: the set of violation codes (`go/internal/core/contract_escalation.go`). The design uses that.
- Several outside sources were read as abstracts only: arXiv 2506.18403, 2605.06350 and 2604.10739.
- No published study compares a same-model retry with a model switch at the same cost in an agent pipeline.
