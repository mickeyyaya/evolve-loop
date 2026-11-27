# Research skill: an LLM-steered research loop with diverse views and a justified stop rule (research, 2026-11)

> The plan that uses this research: [research-diversity-and-depth-2026-11](../plans/research-diversity-and-depth-2026-11.md). The design: [research-loop](../architecture/research-loop.md). The decision record: [ADR-0171](../architecture/adr/0171-research-is-an-llm-steered-loop-inside-a-code-envelope-gated-in-depth-blocks-by-evidence-coverage.md). The skill that changes: [skills/research/SKILL.md](../../skills/research/SKILL.md).
>
> This dossier builds on [oss-harness-architectures-2026-11](oss-harness-architectures-2026-11.md). It does not repeat it. It does not decide. The plan decides.

Date: 2026-11-27. Method: a read of the research skill and a first round of nine web searches. F1 describes the skill before this change, at commit [6dd24b4](https://github.com/mickeyyaya/evolve-loop/tree/6dd24b416b9288e5c6fcc7ec70ebfc8509f6a30a). A second round gave four research notes: commercial systems, open source frameworks, academic work on adaptive depth, and research-to-decision documents. Each outside fact has an evidence label.

**Evidence labels.** *Measured* is a number from the own experiments of a paper. *Maker claim* is a statement of a vendor or a project with no independent number. *Search summary* is a fact seen only in a search summary, not on the primary page. *Synthesis* is our own inference.

## Table of contents

1. [Request, answer and local evidence](#1-request-answer-and-local-evidence)
2. [Outside survey: control loops](#2-outside-survey-control-loops)
3. [Outside survey: stop signals](#3-outside-survey-stop-signals)
4. [Outside survey: budget and width](#4-outside-survey-budget-and-width)
5. [Outside survey: diversity](#5-outside-survey-diversity)
6. [Outside survey: research to decision documents](#6-outside-survey-research-to-decision-documents)
7. [Adopt or reject](#7-adopt-or-reject)
8. [Refinements to adopt](#8-refinements-to-adopt)
9. [Gaps](#9-gaps)
10. [Decisions that the dossier supports](#10-decisions-that-the-dossier-supports)

## 1. Request, answer and local evidence

The operator made three requests on 2026-11-27.

1. "I would like to improve the research skill to have a more diverse views and be able to justify how many layer / depth / level to dive deep before stop"
2. "It should be ai driven research skill and dynamic / adaptively controlled by LLM and be able to integrate into the research, adr, design, and plan flow. Research online to find out how ai do the online deep research"
3. "When it reach to multiple layers search (e.g. every 3 layer depth) it report the status and update summary to the main session and decide if it has enough confidence to end the research or continue to the next multiple layer search"

**Answer.** The systems that this dossier read have one shape. An LLM chooses what to search, what to read and when to stop. Code sets the outer limits. One project has no turn limit (II-Researcher, P2.4), and the RL products are end-to-end (P2.1, maker claim).

The stop rules with the best measured results do not ask the model how sure it is. They check the gathered evidence against the required claims. Diversity comes from independent starts with different stances, not from debate. The block design of request 3 matches this pattern when each barrier returns a ledger and not a story.

| Id | Finding | Source |
|---|---|---|
| F1.1 | The skill before this change split a question into angles. An angle is a topic, not a stance. Most requests need three to five angles. | [skills/research/SKILL.md:39 at 6dd24b4](https://github.com/mickeyyaya/evolve-loop/blob/6dd24b416b9288e5c6fcc7ec70ebfc8509f6a30a/skills/research/SKILL.md#L39) |
| F1.2 | The research method table of the old skill had six rules about sources. No rule said when a researcher stops or how deep a researcher follows a lead. | [skills/research/SKILL.md:53-62 at 6dd24b4](https://github.com/mickeyyaya/evolve-loop/blob/6dd24b416b9288e5c6fcc7ec70ebfc8509f6a30a/skills/research/SKILL.md#L53-L62) |
| F1.3 | The old skill asked each notes file for the gaps. It did not ask why the research stopped. | [skills/research/SKILL.md:46 at 6dd24b4](https://github.com/mickeyyaya/evolve-loop/blob/6dd24b416b9288e5c6fcc7ec70ebfc8509f6a30a/skills/research/SKILL.md#L46) |
| F1.4 | The fan-out of the old skill ran once. No barrier existed where the lead reads the results and decides to continue or to stop. | [skills/research/SKILL.md:64-66 at 6dd24b4](https://github.com/mickeyyaya/evolve-loop/blob/6dd24b416b9288e5c6fcc7ec70ebfc8509f6a30a/skills/research/SKILL.md#L64-L66) |
| F1.5 | The old skill had no link from a decision document back to research. A writer who found a missing fact had no route to ask for it. | [skills/research/SKILL.md:48 at 6dd24b4](https://github.com/mickeyyaya/evolve-loop/blob/6dd24b416b9288e5c6fcc7ec70ebfc8509f6a30a/skills/research/SKILL.md#L48) |
| F1.6 | The first run of the skill used six notes, one for each topic. No note had the task to argue against the chosen design. | [oss-harness-architectures-2026-11](oss-harness-architectures-2026-11.md) |

*Implication:* the skill can collect many sources and still read them all from one stance. It cannot show that a deep dive paid off, or that a short one was enough. It cannot take a request from a document that is half written.

## 2. Outside survey: control loops

| Id | Finding | Source |
|---|---|---|
| P2.1 | Commercial systems have two families. The RL policies are end-to-end, so the code envelope applies to the prompted family. Prompted orchestration, documented fully only by Anthropic, uses a lead that plans, starts parallel subagents, waits for the batch and decides if it starts more. An end-to-end RL policy (OpenAI, Kimi, Tongyi) trains one model to decide when to search, read and finish. Label: maker claim. | [Anthropic engineering](https://www.anthropic.com/engineering/multi-agent-research-system), [OpenAI](https://openai.com/index/introducing-deep-research/), [Kimi-Researcher](https://moonshotai.github.io/Kimi-Researcher/) |
| P2.2 | Tongyi Heavy mode wraps a learned policy in a fixed outer structure of rounds. Each round rebuilds a compact workspace from the earlier outputs. Rebuilding the workspace as a prompt method, with no training, beat ReAct on BrowseComp by 12.7 points with o3 and 19.2 points with DeepSeek-V3.1. Label: measured. | [Tongyi blog](https://tongyi-agent.github.io/blog/introducing-tongyi-deep-research/), [IterResearch](https://arxiv.org/html/2511.07327v2) |
| P2.3 | Open source code has three families. Fixed trees (GPT Researcher deep mode, dzhng) run breadth times depth with breadth halved at each level. Graphs routed by an LLM under code limits (LangChain Open Deep Research, smolagents, Jina, Co-STORM). Single ReAct loops in a reasoning model (Tongyi, MiroThinker). | [dzhng deep-research.ts](https://github.com/dzhng/deep-research/blob/1f8f3e285bbc23e80b98a66a64effab9069f3ad4/src/deep-research.ts#L216-L293), [ODR deep_researcher.py](https://github.com/langchain-ai/open_deep_research/blob/1b7d2e80db9faa586165c60e09096dbbfd483a64/src/open_deep_research/deep_researcher.py#L178-L222) |
| P2.4 | Every project keeps a code backstop under the stop decision of the LLM. The exception is II-Researcher, whose `while True` loop has no turn limit. The backstops are supervisor iterations (ODR, default 6) and a token budget with a forced final answer (Jina, 15% held back). Tongyi uses calls, a clock and a context ceiling. | [ODR configuration.py](https://github.com/langchain-ai/open_deep_research/blob/1b7d2e80db9faa586165c60e09096dbbfd483a64/src/open_deep_research/configuration.py#L42-L150), [Jina agent.ts](https://github.com/jina-ai/node-DeepResearch/blob/5ece300e77c84c871f4f46a5ab13414ab53aae9f/src/agent.ts#L498-L519), [Tongyi react_agent.py](https://github.com/Alibaba-NLP/DeepResearch/blob/f72f75d8c3eb842f2bbbab096a12206ff66e270f/inference/react_agent.py#L128-L224) |
| P2.5 | The worst backstop turns a failure into "done". In ODR the condition `if is_token_limit_exceeded(e, …) or True:` ends the research phase on any error, and the report goes on as if the research were complete. GPT Researcher force-accepts a draft after its revision limit. Jina issue 118 shows more than 100 steps with an unchanged gap list. No project stops on a lack of progress. | [ODR issue 283](https://github.com/langchain-ai/open_deep_research/issues/283), [GPT Researcher editor.py](https://github.com/assafelovic/gpt-researcher/blob/9d39a2f86d55e264f69dfeff2365f27fad3c1144/multi_agents/agents/editor.py#L151-L164), [Jina issue 118](https://github.com/jina-ai/node-DeepResearch/issues/118) |
| P2.6 | MiroThinker handles a limit with honesty. At `max_turns` it writes a failure summary and retries with that summary as input. It does not invent an answer. | [MiroThinker answer_generator.py](https://github.com/MiroMindAI/MiroThinker/blob/1c4253f6774bf40314271a827304b842100e054c/apps/miroflow-agent/src/core/answer_generator.py#L466-L500) |

*Implication (synthesis):* the shape "an LLM inside a code envelope" is the common one, so the design is not novel. The seat status must be a closed set that code sets. "Capped", "stalled" and "errored" must never read as "complete" (P2.5). A rebuilt context at each barrier is a measured gain (P2.2), and it fits the barrier of request 3.

## 3. Outside survey: stop signals

| Id | Finding | Source |
|---|---|---|
| P3.1 | Makers state their stop rules in plain words. The Anthropic lead prompt says to stop at diminishing returns. A subagent has a novelty rule, a hard limit of 20 calls or about 100 sources, and a target of about 15 calls. No maker publishes a calibrated threshold. Label: maker claim. | [research_lead_agent.md](https://github.com/anthropics/anthropic-cookbook/blob/main/patterns/agents/prompts/research_lead_agent.md), [research_subagent.md](https://github.com/anthropics/anthropic-cookbook/blob/main/patterns/agents/prompts/research_subagent.md) |
| P3.2 | In HALT, a 7B agent stopped by itself on 51% of HotpotQA, 64% of 2Wiki and 79% of MuSiQue questions. At that stop, its evidence did not cover every required claim. A forced continuation on the under-covered HotpotQA subset doubled exact match from 0.162 to 0.323. Label: measured. | [HALT](https://arxiv.org/html/2608.02009) |
| P3.3 | The claim coverage stop cut 17–18% of the loops at non-inferior exact match. It stops when evidence supports every required claim. The model made the claims. It beat a zero-shot LLM sufficiency judge. Label: measured. | [HALT](https://arxiv.org/html/2608.02009) |
| P3.4 | Answer stability with a calibrated logit margin (TASR) kept 94.8% of the macro-F1 of a fixed five loops at 62.6% of the calls. The seats of this repository run through CLIs without logits, so only the stability half carries over. Label: measured. | [TASR](https://arxiv.org/abs/2606.13814) |
| P3.5 | A verifier per constraint that chooses SUCCESS, CONTINUE or PIVOT (BATS) lifted BrowseComp. Without the verifier the score fell from 18.7 to 15.4. Label: measured. | [BATS](https://arxiv.org/html/2511.17006v1) |
| P3.6 | Uncertainty triggers on tokens (FLARE, DRAGIN) cut loops by 38–79% but failed the non-inferiority test on exact match. FLARE lost up to 6.3 points. Label: measured. | [HALT](https://arxiv.org/html/2608.02009) |
| P3.7 | Verbal confidence is close to noise. In one study 96.5% of the 1–5 self-ratings were 5. OpenAI Deep Research had a calibration error of 91%. OpenAI lists weak calibration among its own limits. Label: measured. | [TASR](https://arxiv.org/abs/2606.13814), [BrowseComp](https://arxiv.org/html/2504.12516) |
| P3.8 | ReAct agents on BrowseComp-ZH stayed at 30.7% accuracy for every budget of 30 calls or more. They thought they had an answer, or they gave up. HF Open Deep Research abstained on 32.3% of the samples. Label: measured. | [BATS](https://arxiv.org/html/2511.17006v1), [Lost in the Maze](https://arxiv.org/html/2510.18939) |
| P3.9 | Over-search is also real. In 20.2% of its search steps, R1-Searcher had no need to search. Search-R1 had no need in 27.7%. Label: measured. | [Search Wisely](https://arxiv.org/html/2505.17281v2) |
| P3.10 | Hennink, Kaiser and Marconi (2017) separate code saturation from meaning saturation. In 25 interviews the range of issues was complete at 9. A rich understanding needed 16 to 24. 15 of 18 articles claimed saturation without a definition. Label: search summary. | [Hennink and others 2017](https://pmc.ncbi.nlm.nih.gov/articles/PMC9359070), [Hennink escholarship](https://escholarship.org/uc/item/2q0136v7) |
| P3.11 | Information has no value when it cannot change the decision. A stop rule asks "will the next document change our decision?" and gathers a new item only when its expected value is more than its cost. Label: search summary. | [Berkeley CS188, value of perfect information](https://inst.eecs.berkeley.edu/~cs188/textbook/vpis/vpi.html) |

All measured stop results come from multi-hop question answering or browsing benchmarks with short answers, small models and runs of about eight loops. No study measures a stop rule on the quality of a long report.

*Implication (synthesis):* the most transferable mechanism splits the question into required claims at plan time and checks each one at stop time (P3.3). The model can make the claim list itself. Two errors need two controls. A coverage requirement guards against a stop that is too early (P3.2). Hard limits guard against a runaway (P2.4).

Verbal confidence is a log field and never a gate input (P3.7). The answer stability of P3.4 becomes "the leading option of each decision is unchanged since the last barrier". A decision-driving claim needs the deeper standard of meaning saturation (P3.10).

## 4. Outside survey: budget and width

| Id | Finding | Source |
|---|---|---|
| P4.1 | Accuracy rises about log-linearly with the effort of browsing and then flattens. With 2048 turns allowed, the IterResearch agent used 80.1 turns on average. Label: measured. | [IterResearch](https://arxiv.org/html/2511.07327v2) |
| P4.2 | The Budget Tracker of Google appends a used and remaining block after each tool result. It advises 3–5 queries when 70% or more of the budget is left, and one query below 30%. A budget of 10 matched ReAct at a budget of 100 (12.8% against 12.6%) with 40.4% fewer search calls. Label: measured. | [BATS](https://arxiv.org/html/2511.17006v1) |
| P4.3 | Anthropic reports that token use alone explains 80% of the variance in BrowseComp performance. It embeds effort tiers in the prompts, because agents judge effort badly. A fact gets 1 agent and 3 to 10 calls. A comparison gets 2 to 4 subagents and 10 to 15 calls. Label: maker claim. | [Anthropic engineering](https://www.anthropic.com/engineering/multi-agent-research-system) |
| P4.4 | On hard browsing tasks the leading failure is unfocused search, in 33.7–58.7% of samples. Confirmation bias is 6.7–9.7%. "Answer ignored", where the right answer was retrieved and missed, is 26.0–30.7% in the stronger frameworks. Label: measured. | [Lost in the Maze](https://arxiv.org/html/2510.18939) |
| P4.5 | Width helps. Salesforce W&D reached 62.2% on BrowseComp with parallel tool calls on GPT-5-Medium, against 54.9% reported for GPT-5-High. Label: measured. | [W&D](https://arxiv.org/abs/2602.07359) |
| P4.6 | A router for the first effort is useful and unreliable. The complexity classifier of Adaptive-RAG was right 54.52% of the time. An oracle router raised F1 from 50.91 to 62.80. Label: measured. | [Adaptive-RAG](https://arxiv.org/html/2403.14403) |
| P4.7 | The marginal value theorem says a forager leaves a patch when the yield rate of the patch falls to the mean rate of the environment. Information foraging applies the rule to sources. Label: search summary. | [Kilpatrick and others](https://arxiv.org/pdf/2004.10671) |

*Implication (synthesis):* show each seat its remaining calls (P4.2). Go wide first, then deep on the leads that a verifier marks promising and unverified (P4.5). Re-read the evidence that you have before you buy more search (P4.4). Let the run raise its own tier, because the first estimate is wrong about half the time (P4.6). The numbers of the tiers and the block size are design choices. No source measures them.

## 5. Outside survey: diversity

| Id | Finding | Source |
|---|---|---|
| P5.1 | In shipped products "diverse perspectives" means coverage diversity plus source filters. The Anthropic lead asks for distinct tasks and, for depth-first queries, 3–5 different methods or perspectives. A subagent flags spin and marketing and passes unresolved conflicts up. No maker documents a step that searches for counter-evidence, or a measured diversity effect. Label: maker claim. | [research_lead_agent.md](https://github.com/anthropics/anthropic-cookbook/blob/main/patterns/agents/prompts/research_lead_agent.md) |
| P5.2 | STORM finds the perspectives of a topic from tables of contents of related articles. A writer with each perspective questions a grounded expert. It gained about 25% absolute in articles judged well organized and 10% in broad coverage over an outline-driven RAG baseline. Label: measured. | [STORM](https://arxiv.org/abs/2402.14207), [STORM project](https://storm-project.stanford.edu/research/storm/) |
| P5.3 | The moderator of Co-STORM raises unused snippets, ranked by dissimilarity to what the team already asked and cited. Jina has seven fixed query personas, among them an "Expert Skeptic" told to search "Evidence against X". It is the only project with a counter-evidence search, and it works at the query level, not for each claim. | [co_storm_agents.py](https://github.com/stanford-oval/storm/blob/fb951af7744dab086e34962e9bc6fe878e145f83/knowledge_storm/collaborative_storm/modules/co_storm_agents.py#L159-L246), [Jina query-rewriter.ts](https://github.com/jina-ai/node-DeepResearch/blob/5ece300e77c84c871f4f46a5ab13414ab53aae9f/src/tools/query-rewriter.ts#L30-L45) |
| P5.4 | Debate induces a martingale over beliefs. Debate alone does not improve the expected correctness, and a majority vote explains most of the measured gain. Label: measured. | [Debate or Vote](https://arxiv.org/abs/2508.17536) |
| P5.5 | Homogeneous teams of ten agents with 7–8B models took the modal answer up to 85.5% of the time. They fell up to 32.3 points below the oracle. They used 2.1–3.4 times the tokens of isolated self-correction. Label: measured. | [The Cost of Consensus](https://arxiv.org/abs/2605.00914) |
| P5.6 | Model diversity helps when the models have similar strength. A trio of different families reached 91% on GSM-8K against 82% for three instances of one model. Label: measured. | [Diversity of Thought](https://arxiv.org/abs/2410.12853) |
| P5.7 | Diversity collapse in multi-agent idea generation comes from the interaction structure more than from the persona text. A persona prompt alone does not restore it. Label: search summary. | [arXiv 2604.18005](https://arxiv.org/pdf/2604.18005) |
| P5.8 | Anchoring is measurable in search agents. The final answer depends on the order of the first evidence. A history of dead ends reduces the change of strategy. More counter-evidence went sometimes with lower accuracy. Label: measured. | [Cognitive Bias Signatures](https://arxiv.org/abs/2611.05720) |
| P5.9 | In lab studies, groups with devil's advocacy or dialectical inquiry decided better than consensus groups (Schweiger and others, 1989). Devil's advocacy groups considered more alternatives (Valacich and Schwenk, 1995). The cost: members accepted the decision less. Label: search summary. | [Strategic Management Journal 10(1)](https://ideas.repec.org/a/bla/stratm/v10y1989i1p31-43.html) |
| P5.10 | The analysis of competing hypotheses (Heuer) lists hypotheses as columns and rates each item of evidence against each one. An item that agrees with all hypotheses has a weak diagnostic value. The empirical support for the method is thin. Label: search summary. | [Analysis of competing hypotheses](https://en.wikipedia.org/wiki/Analysis_of_competing_hypotheses) |

*Implication (synthesis):* more seats do not give more views when each one reads from the same stance with the same model (P5.5, P5.7). Diversity needs four things. First, the stances come from the subject (P5.2). Second, the seats stay independent until a barrier (P5.4). Third, one seat attacks the leading option (P5.9). Fourth, a seat that did not write a claim checks it, because a counter-search alone does not help (P5.8).

A second model family goes to the skeptic or contrarian seat (P5.6). The diagnostic test of P5.10 is the same as the value-of-information gate of P3.11.

## 6. Outside survey: research to decision documents

| Id | Finding | Source |
|---|---|---|
| P6.1 | GitHub Spec Kit closes the loop between research and plan. `/plan` marks each unknown `NEEDS CLARIFICATION` and makes a research task of each. It writes `research.md` as Decision, Rationale and Alternatives considered. It errors out while a marker is open. | [spec-kit plan.md](https://raw.githubusercontent.com/github/spec-kit/main/templates/commands/plan.md) |
| P6.2 | `/clarify` asks at most five questions, ranked by Impact times Uncertainty, and skips a question when its answer does not change the work. `/analyze` is a read-only audit that rates findings from CRITICAL to LOW. A CRITICAL finding is fixed upstream. | [clarify.md](https://raw.githubusercontent.com/github/spec-kit/main/templates/commands/clarify.md), [analyze.md](https://raw.githubusercontent.com/github/spec-kit/main/templates/commands/analyze.md) |
| P6.3 | Kiro, OpenSpec, BMAD, Cursor and Claude Code research inside the design step. None keeps a separate evidence file or a citation link from plan to research. The documented Claude Code pattern returns "a summary" and saves nothing. | [Claude Code common workflows](https://code.claude.com/docs/en/common-workflows) |
| P6.4 | MADR has Considered Options, Decision Drivers, pros and cons for each option, and a Confirmation field that records how compliance is checked. Rust RFCs split unresolved questions into "before merge" and "during implementation". KEPs mark disputed text inline. | [MADR template](https://raw.githubusercontent.com/adr/madr/develop/template/adr-template.md), [Rust RFC template](https://raw.githubusercontent.com/rust-lang/rfcs/master/0000-template.md), [KEP template](https://raw.githubusercontent.com/kubernetes/enhancements/master/keps/NNNN-kep-template/README.md) |
| P6.5 | GPT-4 writes relevant ADR decisions that "fall short of human-level performance". A study of 750 repositories found that a window of the 3–5 most recent ADRs gives the best balance. Context matters more than model scale. Label: measured, at the level of the abstract. | [Dhar and others](https://arxiv.org/abs/2403.01709), [Context Matters](https://arxiv.org/abs/2604.03826) |
| P6.6 | Citation accuracy and volume trade off. Perplexity had 90.24% citation accuracy with 31.26 effective citations for each task. Gemini had 81.44% with 111.21. The best system in ResearchQA fully addressed fewer than 11% of the citation items. Label: measured. | [DeepResearch Bench](https://arxiv.org/html/2506.11763), [ResearchQA](https://arxiv.org/abs/2509.00496v2) |
| P6.7 | Checkers typically cover only sentences with a citation, so uncited claims go unchecked. No maker documents a runtime check that rejects a citation whose span does not support its claim. Label: search summary. | [DEER](https://arxiv.org/html/2512.17776v1), [Cited but Not Verified](https://arxiv.org/pdf/2605.06635) |
| P6.8 | The ways to scale depth with the stakes are qualitative. Design docs have "little value" when the solution is clear. Kiro Quick Spec skips the approval gates for well understood work. Bezos splits one-way doors from two-way doors. No source shows an AI agent that sets research depth by the reversibility of a decision. | [Design Docs at Google](https://www.industrialempathy.com/posts/design-docs-at-google/), [kiro.dev](https://kiro.dev/docs/specs/) |

*Implication (synthesis):* this repository needs a chain. The links are: source, dossier finding, ADR option or driver, spec rule, plan component and test. No tool automates it (P6.3). The practical form is a link check by code plus a support check by a seat that did not write the claim (P6.7). A marker in the text of the writer becomes a narrow research run (P6.1). The cap of five markers and the rank come from P6.2.

## 7. Adopt or reject

| Practice | Decision | Reason |
|---|---|---|
| Required claims at plan time, a claim ledger at each barrier (P3.3) | Adopt | It is the most transferable measured stop mechanism. |
| A band that code computes from the ledger (P3.2, P3.5) | Adopt | Verbal confidence is noise (P3.7). Under-search is the common failure. |
| A stop on verbal confidence (P3.7) | Reject | 96.5% of the self-ratings were the maximum. Log the value only. |
| A stop on token uncertainty (P3.6) | Reject | The seats have no logits, and the method failed the non-inferiority test. |
| Novelty count as the only stop (P3.1; no paper measures it: gap, synthesis) | Reject | No paper measures it. It stops a hard question too early. Use it as a stall signal. |
| A fixed depth for every question (P2.3, P3.8, P3.9) | Reject | One number overspends on an easy question and stops a hard one too early. |
| Perspective discovery before the fan-out (P5.2) | Adopt | It finds stances that a fixed list does not name. |
| A fixed lens floor: builder, skeptic, measurer (P5.1, P5.3) | Adopt as a floor | It guarantees a skeptic on each request, also when the discovery finds only builders. |
| Debate rounds between seats (P5.4, P5.5) | Reject | Debate of one model converges on the majority and costs two to three times the tokens. |
| Independent seats until a barrier (P5.7) | Adopt | The interaction structure, not the persona, causes the collapse. |
| One contrarian seat after block 1 (P5.9) | Adopt | It gave better decisions than consensus and costs one seat. |
| A full ACH matrix for each request (P5.10) | Adopt in part | Use the diagnostic test to pick leads. The empirical support is thin. |
| Budget display after each call (P4.2) | Adopt | It is the cheapest measured gain. |
| Effort tiers set by stakes (P4.3, P4.6) | Adopt, a tier only rises | A router is right about half the time. |
| A rebuilt context at each barrier (P2.2) | Adopt | A measured gain with no training. |
| A re-read pass before more search (P4.4) | Adopt | The answer was retrieved and missed in 26–31% of samples. |
| A seat status that code sets (P2.5, P2.6) | Adopt | An error must never read as "covered". |
| Spec Kit markers and Decision, Rationale, Alternatives entries (P6.1) | Adopt | They are the proven route from a decision document back to research. |
| MADR Confirmation field (P6.4) | Adopt | It ties a decision to the check that confirms it. |
| A recency window of 3–5 ADRs for the ADR writer (P6.5) | Adopt | It is the best measured context size. |
| An LLM supervisor that directs seats on its own | Reject | AGENTS.md: no LLM in the pipeline starts or directs another. The lead is the console. |

## 8. Refinements to adopt

| Id | Refinement | From |
|---|---|---|
| R1 | The lead splits the question into decisions, and each decision into required claims. The ledger holds each claim with its state, sources and label. | F1.2, P3.3, P3.5 |
| R2 | Two axes: angles split the topic, lenses split the stance. The lens floor is builder, skeptic and measurer. | F1.1, P5.1, P5.3 |
| R3 | The lead finds the perspectives of the subject from three to five overview sources before the fan-out. | P5.2 |
| R4 | Seats never see the notes of other seats before a barrier. No debate rounds. A second model family takes the skeptic or contrarian seat. | P5.4, P5.5, P5.6 |
| R5 | One contrarian seat runs after block 1. It reads the ledger and argues against the leading option. | F1.6, P5.9 |
| R6 | A stakes tier sets the seats, the blocks and the calls before the first query. A tier only rises. | P4.3, P4.6, P6.8 |
| R7 | A block is up to three levels of follow-up for each seat. Each seat sees its remaining calls after each call. | F1.4, P2.2, P4.2 |
| R8 | A seat follows a lead only when it names the claim or decision that the lead can change. | P3.11, P5.10 |
| R9 | A barrier report has a status that code sets, the ledger delta, open leads, exhausted queries and conflicts. Verbal confidence is logged only. | F1.3, P2.5, P3.7 |
| R10 | Code (C2, planned; until then the lead in shadow) computes a band from coverage, stability and independent verification. Progress does not set the band: it detects a stall and blocks DEEPEN. The lead cannot stop below the band of its tier. | P3.2, P3.3, P3.4, P3.5 |
| R11 | The lead re-reads the gathered evidence before it orders more search. At a cap, code forces a stop and the open claims become written gaps. | P2.4, P2.5, P4.4 |
| R12 | The lead reports to the main session at each barrier in one fixed short form. | operator request 3 |
| R13 | The dossier ends with Decision, Rationale and Alternatives entries. Each ADR option cites a finding id, and the ADR names its Confirmation check. | F1.5, P6.1, P6.4, P6.5 |
| R14 | A writer marks a missing fact `NEEDS-EVIDENCE: <claim>`. Each marker is a narrow one-block run. At most five markers for each pass. | F1.5, P6.1, P6.2 |
| R15 | Run in shadow first. Record the band, the choice and the stop reason of each run, and tune the thresholds against later rework. | P3.2, the closing note of the source report |

## 9. Gaps

- No paper measures a novelty stop for research agents. The makers state novelty rules with no measured threshold (P3.1). The rejection of a novelty-only stop is synthesis.
- No source measures a lens roster, a block size of three, or the caps of the tiers for LLM research. They are design choices (synthesis).
- All measured stop rules come from short-answer benchmarks. No study measures a stop rule on a long report. The move to our dossiers is synthesis.
- No study measures fabricated or strawman alternatives in ADRs written by an LLM.
- No source shows an AI agent that sets research depth by the reversibility of a decision.
- Citation hallucination rates of 11% to 57% across models come from a search summary only. A later run must fetch the paper.
- No source verified the saying "decide with 70% of the information". This dossier does not cite it.
- The thresholds of the band are first values. The shadow log must tune them.

## 10. Decisions that the dossier supports

Each entry has the form Decision, Rationale, Alternatives. The plan owns the decision. The entry here only shows the evidence.

- **Decision.** The research skill is an LLM-steered loop inside a code envelope, gated in depth blocks by evidence coverage.
- **Rationale.** The prompted systems and the open source projects that we read have this shape (P2.1, P2.3, P2.4). The exception is II-Researcher, which has no turn limit (P2.4). Coverage beats verbal confidence (P3.3, P3.7). A rebuilt context at each barrier is a measured gain (P2.2).
- **Alternatives.** Fixed quotas: they ignore the question (P2.3) and over- or under-search (P3.8, P3.9). Free LLM judgment: it stops too early (P3.2). A code-only novelty loop: no paper measures it (gap, synthesis).

- **Decision.** Diversity has two axes, a lens floor and independent seats, and no debate.
- **Rationale.** Debate converges (P5.4, P5.5). Personas alone fail (P5.7). A stance found from the subject adds coverage (P5.2).
- **Alternatives.** Debate rounds: 2.1–3.4 times the tokens for no gain (P5.5). One seat with many personas: the structure collapses (P5.7).

- **Decision.** The stakes tier sets the envelope, and a tier only rises.
- **Rationale.** Effort judged by the agent is poor (P4.3). A router is right 54.52% of the time (P4.6). Reversibility is the practical measure of stakes (P6.8).
- **Alternatives.** One depth for all work (P2.3). A tier that can fall: a wrong first estimate locks the run in a low tier.

- **Decision.** A block is up to three levels, and the barrier returns a ledger, not a story.
- **Rationale.** A rebuilt workspace gains 12.7 to 19.2 points (P2.2). A status set by code stops the "failure as done" error (P2.5).
- **Alternatives.** One flat fan-out (F1.4). A barrier at each level: more reports than the main session can read.

- **Decision.** Code computes the band from the ledger. The lead decides inside the band and cannot stop below the band of its tier.
- **Rationale.** Under-search is the common failure (P3.2, P3.8). Verbal confidence is noise (P3.7).
- **Alternatives.** The lead states its own confidence (P3.7). Code decides alone: it cannot judge if a source supports a claim.

- **Decision.** A decision document cites finding ids, and a missing fact goes back as a `NEEDS-EVIDENCE` marker.
- **Rationale.** Spec Kit proves the loop (P6.1, P6.2). MADR gives the Confirmation field (P6.4).
- **Alternatives.** A summary pasted into the plan (P6.3): it has no link back to the evidence.
