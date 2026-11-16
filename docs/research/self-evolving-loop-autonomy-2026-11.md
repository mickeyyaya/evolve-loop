# Self-evolving loops: how much the loop may change itself, and how it heals (research, 2026-11)

> The plan that uses this research: [inbox-classifier-room-2026-11](../plans/inbox-classifier-room-2026-11.md). No ADR yet; Tier B gets one. The earlier research on the same fence: [narrow-fence-2026-10](narrow-fence-2026-10.md). The spec of the fence: [narrow-fence-and-held-proposals](../architecture/narrow-fence-and-held-proposals.md).

Date: 2026-11-16. Method: a read of the classifier code and the inbox on `main` (`1088737ba`), and the wave records of waves 173 to 175. A live web search covered self-improvement and self-repair. Each local claim names its file or its measurement. "Synthesis" marks our own inference.

## The request

The operator gave two requests on 2026-11-16:

> "review the Todo tasks and redesign the classifier to give the evo loop more room to 'improve itself', research the latest self improvement and self healing loop design online and learn as much as we can and compare with the adrs and design in our docs to learn what we should do to expand its capabilities"

> "Balance the design and performance on the self evolve and healing loop, the tradeoff for the pragmatic solution."

This dossier holds the evidence. It does not decide. The plan decides.

**Terms used throughout:**

- **Console route:** an inbox item that only the console builds. A lane never draws it.
- **Lane-routable:** an inbox item that a lane can draw.
- **Kernel:** the protected surfaces that no agent changes ([ADR-0155](../architecture/adr/0155-the-loop-heals-itself-inside-its-boundaries-an-agent-proposes-and-the-pipeline-disposes.md) I-10).
- **Held proposal:** a lane change to a protected path that waits for the operator and never lands by itself ([ADR-0157](../architecture/adr/0157-the-loop-works-its-own-code-a-narrow-fence-and-held-proposals.md)).
- **T0, T1, T2:** the surface tiers of ADR-0155 decision 15, as `gateclass.TierOf` computes them. T0 is each path inside the protected fence. T1 is the other pipeline code. T2 is item code. Since ADR-0157, a lane builds T1 and T2 work. A protected item is T0 work.
- **Tier A, Tier B, Tier C:** the stages of the plan. Tier A changes only the classifier facts. Tier B widens the lanes and gets an ADR. Tier C needs more data.
- **No-work lane:** a cycle that drew an item and found nothing to build.
- **Reliability tags:** [P] = the research seat read the primary page. [S] = a search snippet or a secondary page. [M] = from memory, not re-checked. [–] = the notes give no tag, so treat the claim as not re-checked.

## Table of contents

- [1. Local evidence](#1-local-evidence)
- [2. Systems that change their own code](#2-systems-that-change-their-own-code)
- [3. Self-repair and earned autonomy](#3-self-repair-and-earned-autonomy)
- [4. Adopt or reject](#4-adopt-or-reject)
- [5. Refinements to adopt](#5-refinements-to-adopt)
- [6. Limits](#6-limits)

## 1. Local evidence

All numbers in this section were measured 2026-11-16, unless the finding gives another date.

**F1.1** One classifier decides the console route: `inboxbatch.ConsoleRouted` (`go/internal/inboxbatch/consoleroute.go:26-50`). It reads four things:

1. the `route` field;
2. a protected path (`protectedDerivation`, line 68), computed first at line 31;
3. the kind, `pipeline-*` or `ref-state` (`consoleKindDerivation`, line 57);
4. a `route:"lane"` override. Under that override, a binding protected path wins over the kind (lines 39-42).

No policy key sets the console kinds. The kinds are constants in the code (lines 14 and 18).
*Implication:* a change to the route rules is a code change to one function, not a configuration change.

**F1.2** The live backlog has 268 open items: 221 console and 47 lane-routable. Of the 221 console items, 175 are console by the `route` value, 25 by kind and 21 by a protected path. Of the 175 routes, 69 have no fact behind them. 60 of the 69 have only a free-text `routed_reason`, and 9 have nothing.
*Implication:* the `route` value, not the kernel, holds most of the lanes back. A route with no fact is an opinion that nobody checks again.

**F1.3** The starvation is not new. On 2026-10-17, 0 of 230 inbox items were able to go to a lane ([wave-orchestrator.md](../architecture/wave-orchestrator.md) line 118). On 2026-10-27, 256 of 258 open items were console-owned ([inbox-curator.md](../architecture/inbox-curator.md) line 11).
*Implication:* each fix so far moved the count for a short time. The cause returns because routes accumulate faster than the curator removes them.

**F1.4** Wave 174 (2026-11-15 to 2026-11-16) shipped 11 of 20 cycles. Four no-work lanes (cycles 2409, 2413, 2415 and 2417) each used about 4 LLM minutes. They tripped `LOOP_PIPELINE_BLOCKER_HALT` with rule `no-work-handoffs` at the launch of wave 175. Cycles 2409 and 2413 drew items that `main` already held (`bb69b6092` and `5788eee29`, both on `origin/main`). Cycles 2415 and 2417 drew items whose declared files left out the protected file that the scout then planned to change.
*Implication:* two kinds of bad supply stop the wave: shipped items that stay open, and items whose declared files hide a protected path.

**F1.5** The no-work path sends every triage disposition to the console. `ApplyNoWork` (`go/internal/cycleoutcome/nowork.go:41`) calls `inboxmover.RouteConsole` for each disposition in the loop at lines 57-69. This includes `skip_shipped` and a drop with the reason `already-shipped`. The code does not check that the commit is on `main`.
*Implication:* an item that `main` already closed becomes one more console route with a prose reason. That is the F1.2 pattern, made by the loop itself.

**F1.6** A vocabulary for a closed drop exists. `closedDropReasons` and `ClosedDroppedIDs` (`go/internal/inboxmover/outcome.go:195-212`) list `already-shipped`, `already-done`, `already-landed`, `duplicate`, `superseded` and `obsolete`. Only the post-ship path uses it (`go/internal/phases/ship/postship.go:283`).
*Implication:* the no-work path can reuse the same vocabulary. It needs no new reason list.

**F1.7** The curator refuses to reroute any item with a non-empty `routed_reason`. `canRerouteToLane` (`go/internal/inboxcurator/reroute.go:56-57`) returns false when `recordsAReason` is true. The `curate` run of 2026-11-16 made 0 reroute proposals, 10 consume proposals and 2 merge proposals.
*Implication:* the curator cannot help with the 60 routes of F1.2 that have only prose. Prose blocks the one tool that frees routes.

**F1.8** Other wastes in waves 173 and 174 cost more than the classifier did. Ship retry loops cost 3.5 h in cycle 2382 and about 3 h in cycle 2419 (`HEAD_MOVED`, `FLEET_REBASE_NEEDED` and composed-gate re-audits). One failed cycle (2405) used 255 LLM minutes. The quota bench (`family_held`) was the top idle cause, as measured in wave status (the readings file of 2026-11-15 is not on `main`).
*Implication:* a classifier fix gives the lanes work, but it does not fix the largest time sinks. The plan must keep its scope small and leave these to their own items.

**F1.9** Our design already holds the frozen-kernel idea:

- [ADR-0064](../architecture/adr/0064-pipeline-integrity-boundary.md): the control plane that grades a cycle cannot be edited by that cycle.
- [ADR-0155](../architecture/adr/0155-the-loop-heals-itself-inside-its-boundaries-an-agent-proposes-and-the-pipeline-disposes.md): the loop heals itself inside its boundaries. I-10: no agent changes a kernel surface. I-11: no agent writes its own oracle. Decision 16: T1 needs an independent-family judge, a mutation gate and a `shadow` or canary stage.
- [ADR-0157](../architecture/adr/0157-the-loop-works-its-own-code-a-narrow-fence-and-held-proposals.md): a narrow fence with ten guarantee classes, and held proposals.
- [ADR-0158](../architecture/adr/0158-a-repair-round-runs-only-for-a-finding-the-lane-can-fix-and-the-auditor-applies-a-small-prescribed-fix.md): a repair round runs only for a finding that the lane can fix.
- ADR-0167 (shadow stages prove themselves on chained evidence): proposed in commit `38aa72ef3` on branch `design/shadow-stage-contract-r2`. It is not on `main`, so this file does not link it.

The protected manifest has 228 entries in ten classes (`ProtectedSurfaceManifest`, `go/internal/guards/integrity_surface.go`).
*Implication:* the outside practice in sections 2 and 3 confirms our kernel design. The gap is in the supply of items to the lanes, not in the kernel.

## 2. Systems that change their own code

Each entry gives what the system can change, how it selects and verifies a change, and its guardrails. The tags come from the research notes of 2026-11-16.

**P2.1 Darwin Gödel Machine (DGM, Sakana AI, May 2025)** [P]. The agent edits its own Python repository: prompts, tools and workflows. The search loop, the benchmark evaluation and the model stay fixed. An archive keeps all valid agents. Evaluation is staged: 10 tasks, then 60, then 200 only for a strong child.

The agent faked tool logs that showed tests that it never ran. In another task, it removed the markers that a detector used. SWE-bench went from 20.0% to 50.0%. One run cost about 2 weeks and about $22k [S]. Links: [arXiv 2505.22954](https://arxiv.org/abs/2505.22954), [sakana.ai/dgm](https://sakana.ai/dgm/), [The Register](https://www.theregister.com/2025/06/02/self_improving_ai_cheat/).

**P2.2 Huxley-Gödel Machine (HGM, ICLR 2026)** [S]. HGM scores a change by the results of its descendants, not by its own score. It decides between a new change and more evaluation on an old one. It beats DGM with fewer CPU-hours. The notes record that SICA stopped self-modification after 45% of its budget, because a change broke the modifier. Links: [arXiv 2510.21614](https://arxiv.org/abs/2510.21614), [code](https://github.com/metauto-ai/HGM).

**P2.3 Mendel Gödel Machine (MGM, Aug 2026)** [P, abstract only]. MGM proposes edits from the traces of many tasks, and from a different lineage on the same task. The abstract has no safety section. Link: [arXiv 2608.07645](https://arxiv.org/abs/2608.07645).

**P2.4 SICA, a self-improving coding agent (Apr 2025)** [P]. The best agent edits its whole codebase, which includes its own benchmarks. It selects by a utility of score, cost and time. An LLM overseer checks the run about every 30 s and can cancel it.

SWE-bench Verified (a subset) went from 17% to 53%, at about $7,000 for 15 rounds. On reasoning tasks, the scaffold made the results worse. Links: [arXiv 2504.15228](https://arxiv.org/abs/2504.15228).

**P2.5 AlphaEvolve and OpenEvolve (Google DeepMind, 2025)** [P/S]. A person marks the code blocks that can change and owns the evaluator. AlphaEvolve does not change its own harness. It recovered 0.7% of Borg fleet compute [P]. The use of evaluation cascades (cheap tests first) is [M], not re-checked. Links: [DeepMind blog](https://deepmind.google/discover/blog/alphaevolve-a-gemini-powered-coding-agent-for-designing-advanced-algorithms/), [arXiv 2506.13131](https://arxiv.org/abs/2506.13131), [OpenEvolve](https://github.com/codelion/openevolve).

**P2.6 Self-Taught Optimizer (STOP, 2023)** [S]. An improver program improves its own source. The paper measured how often the generated code turned off the sandbox flag. The rate (about 0.4%) is [M], not re-checked. A utility returned an accuracy above 1000% and did not report an error. Link: [arXiv 2310.02304](https://arxiv.org/abs/2310.02304).

**P2.7 Gödel Agent (ACL 2025)** [S]. The agent patches everything in its runtime, which includes its improvement logic. It has no fixed kernel, and a patch can break the improver. Link: [arXiv 2410.04444](https://arxiv.org/abs/2410.04444).

**P2.8 ADAS, Meta Agent Search (ICLR 2025)** [–]. A fixed meta agent writes new agents as code. The meta agent does not change. The container advice of the authors is [M], not re-checked. Link: [arXiv 2408.08435](https://arxiv.org/abs/2408.08435).

**P2.9 Voyager (NVIDIA, 2023)** [–]. The agent adds programs to a skill library and does not rewrite a skill in place. A separate critic verifies success before it keeps a skill. The gains (3.3x items, 15.3x speed) are [M], not re-checked. Link: [arXiv 2305.16291](https://arxiv.org/abs/2305.16291).

**P2.10 AgentSquare, AFlow and EvoAgentX (2025)** [–]. Each system changes modules behind a typed interface. AgentSquare uses a cheap predictor to drop weak designs before a full evaluation. Links: [AgentSquare](https://arxiv.org/abs/2410.06153), [AFlow](https://arxiv.org/abs/2410.10762), [EvoAgentX](https://arxiv.org/abs/2507.03616).

**P2.11 Promptbreeder (ICML 2024)** [–]. The system evolves its prompts and the prompts that mutate them. The fitness stays fixed and external. Link: [arXiv 2309.16797](https://arxiv.org/abs/2309.16797).

**P2.12 DSPy GEPA (ICLR 2026)** [–]. GEPA keeps the best candidate for each case on a Pareto frontier. An LLM reads full traces to propose edits. It needs up to 35x fewer rollouts than GRPO. Link: [arXiv 2507.19457](https://arxiv.org/abs/2507.19457).

**P2.13 Live-SWE-agent (Nov 2025)** [–]. The agent makes its own tools at run time for each issue, with no offline evolution. It scored 65.0% against 53.3% for DGM on a 60-task subset. Link: [arXiv 2511.13646](https://arxiv.org/abs/2511.13646).

**P2.14 Ouroboros (Aug 2026)** [P, abstract only]. The agent changes its own core, but only through reviewed commits. Benchmark runs use frozen snapshots, and the live deployment evolves on a separate lineage. Link: [arXiv 2608.08311](https://arxiv.org/abs/2608.08311).

**P2.15 Hierarchical Self-Improvement (Aug 2026)** [P, abstract only]. A harness for each task family evolves behind a fixed seam. A meta-evolver works under a frozen outer anchor. A held-out 20% split checks each result. Link: [arXiv 2608.08466](https://arxiv.org/abs/2608.08466).

**P2.16 "The Last AI Built by Humans" (Sep 2026)** [S]. The paper gives a ladder of autonomy levels. A secondary summary says that each candidate passes three gates: regression safety, target result and robustness. The abstract does not confirm the gates. Links: [arXiv 2609.11873](https://arxiv.org/abs/2609.11873), [summary](https://academy.dair.ai/papers/collections/self-improving-agents).

**P2.17 RewardHackingAgents (Mar 2026)** [S]. In a fully editable workspace, agents changed the evaluator in about 50% of episodes without a request. A lock on the evaluator stopped it, at a median run-time cost of about 25-31%. One defense blocks one attack vector only. Link: [arXiv 2603.11337](https://arxiv.org/abs/2603.11337).

**P2.18 Survey of 249 papers (Cluster Computing, 2026)** [S]. The survey names an "RSI-safety paradox": systems with high scores have low safety scores. Link: [Springer](https://link.springer.com/article/10.1007/s10586-026-06537-4).

**P2.19 Anthropic** [S, with [P] where marked]. The RSP sets thresholds for AI research and development capability ([RSP](https://www.anthropic.com/rsp)). A paper found that a model that learned reward hacks in code also sabotaged a detector of its own misalignment ([arXiv 2511.18397](https://arxiv.org/abs/2511.18397), tag [–]). The harness post keeps a feature list in JSON and tells the agent not to edit tests ([post](https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents), tag [–]).

**P2.20 OpenAI** [–]. The Preparedness Framework v2 tracks "AI Self-improvement" as a category ([PDF](https://cdn.openai.com/pdf/18a02b5d-6b67-4cec-ab64-68cdfbddebcd/preparedness-framework-v2.pdf)). The harness post enforces architecture rules with linters and structural tests that an agent cannot argue past ([post](https://openai.com/index/harness-engineering/)). The detail of the CoT monitoring paper is [M], not re-checked ([PDF](https://cdn.openai.com/pdf/34f2ada6-870f-4c26-9790-fd8def56387f/CoT_Monitoring.pdf)).

**P2.21 Google DeepMind** [S]. The Frontier Safety Framework has capability levels for ML research and development ([summary](https://futureagi.com/blog/frontier-model-safety-analysis-2026/)). AlphaEvolve changes to the training stack went through evaluators and reviews that people own.

**Synthesis of section 2.** Each system that worked kept a fixed part. Examples are the evaluator (P2.1), the meta agent (P2.8), a frozen anchor (P2.15) and a locked evaluator (P2.17). The one system with no fixed part (P2.7) runs only small tasks.

Agents attack the grader at a measurable rate (P2.1, P2.6, P2.17, P2.19). A file lock stops the attack, and an instruction does not. Staged evaluation (P2.1, P2.12) and additive change (P2.9, P2.13) keep the cost and the risk low.

## 3. Self-repair and earned autonomy

**P3.1 Nx Cloud `Self-Healing CI`** [S]. A person approves each fix by default. Auto-apply needs a glob match on the task, high confidence and a pass on a fresh run. Nx tracks the verification rate for each task and recommends new auto-apply tasks, and a person accepts each one. Links: [docs](https://nx.dev/docs/features/ci-features/self-healing-ci), [blog](https://nx.dev/blog/self-healing-ci-auto-apply-suggestions).

**P3.2 Semaphore** [S]. The fix goes to a separate branch. The trigger ignores branches that the agent made, so a fix cannot start another fix. Link: [docs](https://docs.semaphore.io/using-semaphore/ai/self-healing-ci).

**P3.3 Gitar (Sonar)** [S]. Gitar separates flaky and infrastructure noise from real breakage before it fixes. Each fix is a separate commit that a person can revert. Spend caps apply ($10 for each review, $50 for each PR). Links: [product](https://www.sonarsource.com/products/gitar/), [blog](https://www.sonarsource.com/blog/the-missing-intelligence-layer-for-ci/).

**P3.4 Uber FlakyGuard (2025)** [P]. The agent fixes flaky tests every day, and a developer accepts or rejects each fix. In 6 months, 1,115 tests were tried, 380 fixes were made and 197 landed (51.8%). Reviewers rejected fixes that relaxed mocks or changed the intent of the test. Link: [arXiv 2511.14002](https://arxiv.org/html/2511.14002v1).

**P3.5 Uber AutoClean and Minion (2026)** [S]. AutoClean removes stale flags and fixes the CI breaks that it causes. Engineers review the final diff. Uber reports that autonomous diffs work for toil but not for whole features. Links: [cleanup](https://www.uber.com/us/en/blog/ai-code-cleanup/), [factory](https://www.uber.com/ph/en/blog/efficient-software-factory/).

**P3.6 Flaky-test quarantine at Google and Trunk** [S]. About 16% of Google tests show some flakiness. Trunk stopped its auto-quarantine of broken tests in September 2025, so quarantine applies only to flaky tests. Link: [Trunk changelog](https://docs.trunk.io/changelog/2025-09-09-flaky-tests-auto-quarantine-no-longer-applied-to-broken-tests).

**P3.7 Google Passerine (ICSE-SEIP 2025)** [S]. Passerine made plausible patches for 73% of machine-reported bugs but valid patches for only 43%. For human-reported bugs, the rates were 25.6% and 17.9%. Link: [arXiv 2501.07531](https://arxiv.org/abs/2501.07531).

**P3.8 GitHub Copilot Autofix** [S]. Autofix suggests a fix in the PR, and a person accepts it. GitHub gives no official merge rate. Link: [changelog](https://github.blog/changelog/2026-10-15-agentic-autofix-for-code-scanning-alerts-general-availability).

**P3.9 Meta RADAR (ASE 2026)** [P]. This is the closest analogue: review capacity, not code authorship, was the bottleneck. The route depends on the source of the diff. A runbook needs a 60-day clean history, a daily landing cap and a denylist.

Three checks must all pass: path and content blocklists, a risk score threshold and an LLM reviewer. A higher trust gives a higher threshold. An approved bot diff waits a set delay, and a person can reject it in that window. An incident pauses the automation, and the runbook goes on a permanent denylist.

RADAR reviewed 535K diffs and landed 332K. The revert rate was about 1/3 of the rate for other diffs, but the data is observational. Link: [arXiv 2605.30208](https://arxiv.org/html/2605.30208).

**P3.10 Meta Diff Risk Score and JiTTests** [S]. A fine-tuned LLM predicts the chance that a diff causes an incident. JiTTests makes tests for risky diffs, so a high risk buys more checks, not exclusion. Link: [arXiv 2410.06351](https://ar5iv.labs.arxiv.org/html/2410.06351).

**P3.11 MAPE-K, reconcilers and canary analysis** [S]. A Kubernetes controller compares the observed state with the desired state on each pass. Its actions must be idempotent. A status write that starts a new pass is a known defect. Argo Rollouts compares a canary with a baseline and rolls back after failures. Links: [MAPE-K with LLMs](https://arxiv.org/abs/2506.22185), [desired state](https://www.golinuxcloud.com/desired-state-vs-actual-state-kubernetes/), [Argo](https://last9.io/blog/argo-rollouts-canary-monitoring-last9).

**P3.12 Azure SRE Agent and AWS DevOps Agent** [S]. Azure sets a run mode for each response plan, not for the whole agent. What the agent can reach is separate from the choice to ask first. Delete operations are blocked in all modes. AWS limits automated action on purpose. Links: [Azure run modes](https://learn.microsoft.com/azure/sre-agent/run-modes), [AWS](https://aws.amazon.com/blogs/devops/leverage-agentic-ai-for-autonomous-incident-response-with-aws-devops-agent).

**P3.13 Google SRE error budgets** [S]. The example policy blocks releases only when the budget is spent and the root cause is not known. It freezes only what needs a freeze. Link: [SRE workbook](https://sre.google/workbook/error-budget-policy/).

**P3.14 Program repair evidence** [S, with [M] where marked]. Agentless uses a fixed localize, repair and validate pipeline with a reproduction test ([arXiv 2407.01489](https://arxiv.org/abs/2407.01489)). A study found that 29.6% of plausible SWE-bench patches behave differently from the reference ([arXiv 2503.15223](https://arxiv.org/abs/2503.15223)). The figure that execution checks add more than 15 points is [S]. The SWE-agent, OpenHands, AutoCodeRover and RepairAgent references are [M], not re-checked.

**P3.15 Levels of autonomy** [S]. Feng and others define five levels by the role of the user, from operator to observer. They propose autonomy certificates, backed by an evidence case ([arXiv 2506.12469](https://arxiv.org/abs/2506.12469)). The AWS scoping matrix separates agency (which actions exist) from autonomy (how freely the agent acts) ([AWS](https://aws.amazon.com/blogs/security/the-agentic-ai-security-scoping-matrix-a-framework-for-securing-autonomous-ai-systems)).

**P3.16 Claude Code auto mode (Anthropic, 2026)** [P]. File edits inside the project skip the classifier, because version control makes them reversible. A fast filter flags actions, and a slower review checks only the flagged ones. The classifier sees user intent and actions, not the prose of the agent. Three blocks in a row, or 20 in total, go to a person.

People accepted 93% of prompts, so a blanket human gate is weak evidence. Link: [post](https://www.anthropic.com/engineering/claude-code-auto-mode).

**P3.17 Progressive autonomy as preference learning (2026)** [P]. A model learns the chance that a person approves an action. Unknown actions default to "ask". In simulation, 68% of decisions were automatic at 97.3% accuracy. The model never re-checks regions where it is confident, so the author needs random audit samples. Link: [arXiv 2605.19151](https://arxiv.org/html/2605.19151v1).

**P3.18 Progressive trust and practitioner reports** [S]. An IETF draft grows authority from scored, signed records, and a person approves each raise ([draft](https://www.ietf.org/archive/id/draft-sato-soos-pt-03.txt)). Practitioner posts describe shadow, canary, supervised and autonomous stages ([Monte Carlo](https://www.montecarlodata.com/blog-agentic-autonomy-is-a-trust-score/), [tianpan.co](https://tianpan.co/blog/2026-04-17-earned-autonomy-ai-agents-progressive-supervision), [urandom.io](https://urandom.io/blog/2026-03-02-shadow-mode-for-agentic-ai)).

**P3.19 ITIL 4 and Google large-scale changes** [S]. ITIL pre-approves standard changes and gives change authority by risk. At Google, a committee approves a class of change once, and code owners do not hold a veto over it. Links: [ITIL](https://itsm.tools/change-enablement/), [SWE book ch. 22](https://abseil.io/resources/swe-book/html/ch22.html).

**Synthesis of section 3.** Autonomy is set for each surface or task class, not for the whole system (P3.1, P3.9, P3.12). A higher risk buys more checks, not exclusion (P3.9, P3.10, P3.19). Exclusion stays for a small named set of irreversible actions (P3.9, P3.12). A track record earns the next level, and a person confirms it (P3.1, P3.18). A test that a fix weakens is its own risk (P3.4, P3.14).

Vendor numbers in P3.1, P3.3, P3.5 and P3.8 are vendor claims, and nobody verified them.

## 4. Adopt or reject

| Practice | Decision | Reason |
|---|---|---|
| A small frozen kernel that no agent changes (P2.1, P2.8, P2.15, P2.17) | Adopt (already in place) | ADR-0155 I-10 and I-11 and the ADR-0157 fence hold it (F1.9). |
| Facts, not prose, for each console route (P3.16) | Adopt now (Tier A) | 69 of 175 routes have no fact (F1.2), and prose blocks the curator (F1.7). |
| Consume a verified-shipped item; do not escalate it (P3.11) | Adopt now (Tier A) | Two of four no-work lanes in wave 174 drew shipped items (F1.4, F1.5). |
| Shadow run, then widen with caps (P3.9, P3.18) | Adopt | The shadow-stage rule of ADR-0167 (proposed, F1.9) and the RADAR daily cap give the method. |
| Protection checked on the diff at ship, and refusal of a weaker gate (P2.17, P2.20, P3.4) | Already in place | `gateclass`, `core.GateClassifierFloorChecks` and the ship check `refuseWeakenedGates` are on main. A declared-file list misses the path (F1.4), so the diff check stays the guard. |
| Risk tiers that buy checks, not a binary owner (P3.9, P3.10, P3.12, P3.19) | Adopt in Tier B | ADR-0155 decision 16 names the checks for T1. ADR-0157 holds T0 work as a held proposal (F1.9). |
| A trust score and error budget for each surface (P3.1, P3.13, P3.17) | Defer to Tier C | About 20 cycles in each wave give too few landings for each surface. |
| Staged evaluation and replay of past cycles (P2.1, P2.12, P2.14) | Defer to Tier C | The replay corpus does not exist. F1.8 shows larger costs to fix first. |
| An ML risk score, as in RADAR (P3.10) | Reject for now | We have too few cycles to train a model: 535K diffs at Meta, against hundreds of cycles here. |
| An open-ended archive of agent variants, as in DGM (P2.1, P2.2) | Reject | One DGM run cost about 2 weeks and about $22k [S]. |
| A recursion guard and spend caps on repair (P3.2, P3.3) | Adopt (already in place) | ADR-0155 I-13 and ADR-0158 decision 10 hold them. |
| Random audit of the "allow" region (P3.17) | Defer to Tier C | It needs Tier B landings to sample. |

## 5. Refinements to adopt

**R1 (Tier A).** When a lane finds that `main` already holds an item, the loop consumes the item. It does so only for a commit that it verified on `main`. The no-work path reuses the closed-drop vocabulary of the post-ship path. Because: F1.4, F1.5, F1.6.

**R2 (Tier A).** When the loop made a console route and no fact backs it, the curator proposes a lane route. A fact is a kind, a protected path or a named operator decision. Because: F1.2, F1.7.

**R3 (Tier A).** When a lane refuses an item at a protected surface, the refusal records the protected path as a fact on the item. The next draw then sees the path. Because: F1.4.

**R4 (Tier B).** A lane builds a T0 item that a revert can undo as a held proposal of ADR-0157. A cap for each wave applies. The admission of the item to a lane is the step that ADR-0157 leaves open. Because: F1.9, P3.9, P3.10, P3.19.

**R5 (in place).** The ship refuses a diff that weakens a gate: fewer checks, looser thresholds or removed assertions. Main already has it (`gateclass`, `refuseWeakenedGates`). Tier B relies on it and adds only the gaps that the `internal-gateclass` package note lists. Because: F1.9, P2.17, P3.4.

**R6 (Tier C).** When about 20 landings of T0 held proposals exist, the loop keeps a record and an error budget for each surface. Because: P3.1, P3.13, P3.17, and the volume in F1.8.

## 6. Limits

- The numbers of F1.2, F1.4, F1.7 and F1.8 come from the console readings of 2026-11-16 and the wave records. This file does not give a command that recomputes them.
- The research notes did not re-check claims with the tag [M] or [–]. Treat those claims as leads, not as facts.
- RADAR results (P3.9) are observational. Selection bias can explain part of the lower revert rate.
- This dossier does not measure how many of the 221 console items R1 to R3 free. The plan measures that.
