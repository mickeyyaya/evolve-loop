# Converging review feedback within limited rounds: research (2026-10-07)

| Field | Value |
|-------|-------|
| title | The best policy to converge review feedback within a limited number of rounds |
| date | 2026-10-07 |
| trigger | Operator, 2026-10-07: *"I would like you to research online to learn the best policy to coverage the review feedback with limited rounds"*, after L2 needed seven review fix rounds ([incident](../incidents/2026-10-07-l2-seven-review-rounds.md)) |
| backs | [ADR-0126](../architecture/adr/0126-every-iterative-loop-converges-or-escalates.md) · design [convergence-policy.md](../architecture/convergence-policy.md) · plan [convergence-policy-2026-10.md](../plans/convergence-policy-2026-10.md) |
| sources | 9 fetched and read on 2026-10-07; 3 more from search summaries, marked *secondary*. It builds on the repo's earlier 60-source survey ([ship-rate sources](ship-rate-harness-reliability-2026-09-02-sources.md)). |
| status | accepted research; it shaped refinements CR1–CR8 of the design |

## Question

A review loop judges a change, the author fixes, and the judge looks again. Which policy makes that loop **converge in few rounds without losing real defects**?
- **When should it stop?**
- **What should change between rounds?**
- **When is escalating to a stronger model or a higher effort worth it?**
- **How do strong human code-review cultures keep from "pursuing perfection with no output"?**

## Findings

### F1. Most of the gain is in the first two rounds

- **The measurement** (*How Many Tries Does It Take? Iterative Self-Repair Across Scales*, arXiv 2604.10508, fetched):
  - "two repair rounds capture the bulk of the benefit": **76–95%** of the total achievable improvement;
  - round 1 gives the largest gain for every model, and rounds 3–4 contribute minimally;
  - assertion (logic) errors are the hardest to repair, at about 45% success.
- **The paper's recommendations:**
  - "Allocate two rounds for cost-sensitive applications";
  - "route assertion-dominated failures to alternative strategies", meaning a different strategy, not more rounds.
- **The repo's earlier survey agrees:** "cap repair rounds at ~2, then change something (feedback, context, or tier)".

**Shaped:**
- **CR1:** two sequential rounds; a third round only after a strategy change; never a fourth.
- **CR5:** logic and correctness findings that survive two rounds go to a different strategy.

### F2. A fixed budget can make things worse; stop on marginal gain and keep the best

*Verify, Repair, Repeat, or Stop? Robust Stopping for Noisy Verify-Repair Loops in LLM Agents* (arXiv 2607.17641, fetched).
- **The problem it names.** "Both the verifier and the repairer are noisy, repair can damage already-correct plans": "reported acceptance keeps rising while true validity falls", and "true validity peaks at an interior round and then collapses".
- **The policy, VRR-Stop:** stop when the expected marginal gain `(1−b)·α − b·β ≤ 0`, where α is the repair success rate and β the repair damage rate.
  - Against a fixed 5-round repair it gave **+60.6 points** of validity (0.722 against 0.116), using **0.72** repair rounds on average.
- **The fallback, VRR-Guard.** When the verifier cannot discriminate (J = 1 − ρ₀ − ρ₁ near 0), it keeps the best candidate and replaces it only with a sufficient verification margin.

**Shaped:**
- **CR2:** a round whose repair damage (regressions in its own hunks, or previous FIXED findings reopened) is at least its repairs has a marginal gain ≤ 0. The loop stops sequential repair at once.
- **CR3:** keep-best. The landed candidate is the best qualifying round, never automatically the last.

### F3. LLM reviewers are noisy across runs; inject determinism and verify, do not re-hunt

- ***OpenCodeReview: Determinism over Non-Determinism*** (arXiv 2608.09290, fetched).
  - **The problem:** unbounded agent review shows "variance across runs": "the same PR produces different findings depending on which repository paths an agent explores".
  - **The fix:** rule-guided dispatch, so "the same PR always yields the same file and criterion assignment"; bounded tools; and an **independent reflection** filter that sees only the diff.
  - **The result:** the same model reached **2.17×** SEM-F1 and **33.9% against 7.2%** precision, at **5–15× fewer tokens**. "System design matters more than model capability."
- **Search summaries (*secondary*):** raw LLM reviews vary run to run, "some runs flagging an issue while others miss it entirely". Single-pass reviews carry 2–3 false positives in 8 findings.
- **The repo's earlier Refute-or-Promote finding:** each finding must survive an attempt to disprove it.

**Shaped:** CR4.
- Rounds 2 and later are verify-only, with a fixed scope: the previous findings plus the fix diff's own hunks.
- A new finding on code **unchanged since `J_0`**, which `J_0` did not report, is treated as judge noise and filed. A late CRITICAL or HIGH blocks only when its certificate survives the falsification check.
- Each blocking finding passes a falsification check that sees only the diff.

### F4. More iterations do not converge a review; actionable feedback that reduces uncertainty does

*When AI Teammates Meet Code Review* (arXiv 2602.19441, fetched; 33,596 agent-authored PRs in 2,807 repositories).
- "Neither iteration intensity nor test additions are significantly associated with merge outcomes once reviewer engagement and coordination stability are accounted for."
- "Producing more revisions or adding tests alone does not increase integration likelihood without reducing reviewer uncertainty."
- "Actionable review convergence enables success"; "iteration without convergence is insufficient".

**Shaped:** CR4 again. The delta judge's job is to **resolve the previous concerns** (reduce uncertainty), not to raise new ones. Every blocking finding keeps its concrete fix and certificate.

### F5. Strong human review cultures bound review by "better", not "perfect"

- ***Google Engineering Practices, the standard of code review*** (fetched).
  - "Reviewers should favor approving a CL once it is in a state where it definitely improves the overall code health of the system being worked on, even if the CL isn't perfect."
  - "There is no such thing as 'perfect' code—there is only better code."
  - Polish is prefixed "Nit:" as "a point of polish that they could choose to ignore".
  - On disagreement: "Don't let a CL sit around because the author and the reviewer can't come to an agreement". Escalate to a tech lead or a maintainer.
- ***Conventional Comments*** (*secondary*): every comment carries a label and a **(blocking)** or **(non-blocking)** decoration. Nitpicks are non-blocking by nature.

**Shaped:**
- **CR6:** every finding carries an explicit `blocking` decision, derived from its severity, its kind and its location, plus a `kind` (defect or capability). The base blocking bar falls on "does this change make the code worse", and polish never blocks.
- A dispute goes to an adjudicator (in the code-review design, the audit adjudicates a DISPUTED finding), never to another round.

### F6. Spend on the hard cases: sequential refinement for near-misses, a strategy change for the hard ones

*Scaling LLM Test-Time Compute Optimally* (Snell et al., ICLR 2025, arXiv 2408.03314, fetched in summary).
- Sequential revision acts as "local refinement" of a response "already somewhat on the right track", while parallel sampling acts as "global search".
- The compute-optimal mix depends on difficulty.

**Shaped:**
- CR1 and CR5: rounds 1–2 are local refinement.
- A finding that survives them signals a hard case. The next step is a **global** change: a fresh context, re-planning the approach, or (phase B) parallel candidates on cheap tiers. It is never a third local revision.

### F7. Escalate effort first, then the model, only with headroom and for reasoning failures

- ***Codified Effort and Escalation Policy*** (agentpatterns.ai, fetched).
  - The order: "Raise effort first … low → medium → high → xhigh", then "switch models on evidence", and "stop at high, not max".
  - "Raising effort only pays where the model has headroom below its ceiling and the failure is a reasoning failure." Capability gaps need a different model, and formatting or boilerplate never needs effort.
- ***Is Escalation Worth It?*** (arXiv 2605.06350, fetched). Escalation pays when the stronger model's failures are **not correlated** with the weaker one's, and when the deferral rule can tell which cases are hard.
- **The operator, 2026-10-07:** "The latest top model is opus 5.5 with xhigh effort, not fable 5.1."
  - Claude's deep tier is Opus 5.5 at high effort, and top is Opus 5.5 at **xhigh**.
  - Headroom is therefore an **effort** step, within one model whose failure modes are correlated.

**Shaped:** CR7.
- Headroom is defined over (model, effort).
- An effort raise (high → xhigh) is used only for **reasoning-class** findings: correctness, concurrency, architecture.
- Hygiene, docs and format findings never escalate.
- A disagreement between Opus-high and Opus-xhigh is settled by **deterministic evidence** (a certificate or test), not by a third LLM opinion, because their errors are correlated.

### F8. Cascades and routers: default cheap, spend on proof (*secondary*)

The 2026 routing guides agree on three things:
- cascades escalate only when a scoring rule says the cheaper answer falls short;
- routers should buy failover, not judgment;
- most traffic does not need the best model.

**Shaped:** CR8. Escalation is a rung decided by evidence (`Decide`), never a default. A decision records why it escalated.

## Synthesised policy (what the design adopts)

1. **Round 1 is a full review.** Every finding carries severity, kind, blocking and a certificate with a fix (F3–F5).
2. **Round 2 is verify-only, with a fixed scope** (the previous findings plus the fix's hunks) and a falsification check. Effort rises to xhigh only for reasoning-class blockers (F3, F4, F7).
3. **Stop on marginal gain.** A round with repair damage at or above its repairs stops sequential repair (F2).
4. **Round 3 only after a strategy change:** a fresh context or a re-plan. The bar rises to HIGH, and the rest is deferred and filed (F1, F6).
5. **Never round 4.** Exit by split, by accept-with-limits for a fail-safe residue, or by stop. **Keep the best** qualifying round (F2).
6. **Disagreements go to an adjudicator,** never another round (F5).
7. **Judge noise is not a defect.** New findings on unchanged code are filed. A late CRITICAL or HIGH blocks only when it survives falsification (F3).

## Where the sources disagree

| Tension | Positions | Resolution here |
|---|---|---|
| How many rounds? | Two rounds capture 76–95% (F1); VRR-Stop averages 0.72 rounds and caps at 5 (F2) | Two sequential rounds by default; the marginal-gain stop can end earlier; a third only after a strategy change; never four |
| Escalate the model or the effort? | Effort first (F7); a stronger model only when its errors are uncorrelated (F7, 2605.06350) | Effort high → xhigh (the operator's top), for reasoning-class findings only. Disagreements are settled by deterministic evidence. |
| Sequential or parallel? | Sequential beats parallel at equal budget (F6), but parallel searches globally | Sequential for rounds 1–2. Parallel (best-of-N) is a phase-B option for cheap tiers after a strategy change. |

## Sources

Fetched and read on 2026-10-07:
1. *How Many Tries Does It Take? Iterative Self-Repair in LLM Code Generation Across Model Scales and Benchmarks*, arXiv 2604.10508: https://arxiv.org/html/2604.10508v1
2. *Verify, Repair, Repeat, or Stop? Robust Stopping for Noisy Verify-Repair Loops in LLM Agents*, arXiv 2607.17641: https://arxiv.org/html/2607.17641v1
3. *OpenCodeReview: Determinism over Non-Determinism for Cost-Effective Agent-Based Code Review*, arXiv 2608.09290: https://arxiv.org/html/2608.09290v1
4. *When AI Teammates Meet Code Review: Collaboration Signals Shaping the Integration of Agent-Authored Pull Requests*, arXiv 2602.19441: https://arxiv.org/html/2602.19441
5. Google Engineering Practices, *The Standard of Code Review*: https://google.github.io/eng-practices/review/reviewer/standard.html
6. *Codified Effort and Escalation Policy in the Instruction File*, AgentPatterns.ai: https://www.agentpatterns.ai/instructions/codified-effort-escalation-policy/
7. *Is Escalation Worth It? A Decision-Theoretic Characterization of LLM Cascades*, arXiv 2605.06350: https://arxiv.org/pdf/2605.06350
8. Snell et al., *Scaling LLM Test-Time Compute Optimally can be More Effective than Scaling Model Parameters*, arXiv 2408.03314: https://arxiv.org/abs/2408.03314. Read through its abstract and the search summary.
9. The repo's earlier survey: [ship-rate-harness-reliability-2026-09-02-sources.md](ship-rate-harness-reliability-2026-09-02-sources.md) (self-repair limits, cascades, "upgrade the finding").

*Secondary* (search-result summaries only; nothing rests on them alone):

10. Conventional Comments (blocking and non-blocking decorations): https://lyz-code.github.io/blue-book/conventional_comments/
11. LLM code-review nondeterminism and false positives (G-Research engineering notes): https://www.gresearch.com/news/building-a-code-review-tool-the-llm-patterns-that-actually-work/
12. 2026 model routing and cascades guides: https://www.tmls.nyc/research/model-routing-cascades
