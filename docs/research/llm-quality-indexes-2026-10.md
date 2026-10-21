# Quality indexes for an LLM development loop: local baselines and current practice (research, 2026-10)

> The plan that uses this research: [llm-quality-indexes-2026-10](../plans/llm-quality-indexes-2026-10.md). The design: [llm-quality-indexes](../architecture/llm-quality-indexes.md). The decision record: [ADR-0153](../architecture/adr/0153-quality-is-a-panel-of-separate-indexes-with-intervals-that-the-signal-center-publishes.md).

Date: 2026-10-21. Method: a read of the run records, the ledger, the review files and the Git history. Five small Go programs computed the baselines with no LLM call. A live web search ran on 2026-10-21.

Each local claim names its file or its command. "Synthesis" marks our own inference. "Secondary" marks a figure that we read in a summary and not in the primary paper.

## Table of contents

1. [Why standard metrics are not enough here](#1-why-standard-metrics-are-not-enough-here)
2. [Local evidence](#2-local-evidence)
3. [Current practice](#3-current-practice)
4. [Statistics for small samples](#4-statistics-for-small-samples)
5. [Adopt or reject](#5-adopt-or-reject)
6. [Refinements to adopt](#6-refinements-to-adopt)
7. [Strength of the evidence](#7-strength-of-the-evidence)

**Terms used throughout:**

- **Index:** one number that answers one quality question, with a formula, a unit, a window and an interval.
- **Lane:** one autonomous build loop. A **wave** is one batch of lanes between two boundaries. A **cycle** is one item from claim to ship or to stop.
- **Ship rate:** the share of cycles that land a commit on `main`.
- **pass^k:** the probability that all k independent tries of the same task pass. It measures consistency. **pass@k** is the probability that at least one of k tries passes. It measures capability.
- **Wilson interval:** a confidence interval for a proportion that stays inside [0, 1] and keeps its coverage for small samples.
- **MDE (minimum detectable effect):** the smallest true difference that a test finds with a stated power at a stated sample size.
- **Mutant:** a copy of the code with one small, deliberate fault. A test suite **kills** a mutant when a test fails on it. A mutant **lives** when all tests pass.
- **Mutation efficacy:** killed mutants divided by killed plus lived mutants.
- **Equivalent mutant:** a mutant that behaves the same as the original code. No test can kill it, so it lowers the score unfairly.
- **Reward hack:** an agent result that passes the checks but does not do the task. Example: the agent edits a test so that it passes.
- **Held-out check:** an acceptance check that the builder cannot see or change.
- **False PASS:** a PASS verdict on a change that a later fact proves wrong, for example a revert.
- **Escape:** a defect that reaches `main` and that a later item must fix.
- **Churn:** lines that a later change revises or removes within a short time after they land.

## 1. Why standard metrics are not enough here

Standard software metrics make three assumptions. One person writes the code. The same input gives the same output. The author does not optimize against the metric. In this loop none of the three is true. An LLM writes the code, the same item gives a different result on each try, and the builder sees the gates that judge it.

| Assumption of a standard metric | What is true in the evolve loop | Consequence for an index |
|---|---|---|
| One run gives the answer. | The same item passes on one try and fails on the next (F2.4). | A rate needs an interval, and consistency needs pass^k (P3.1). |
| The author does not target the metric. | A builder agent reads its gates and can satisfy the letter of a check (P3.3, F2.7). | Each index needs a data source that the builder cannot write, and a counter-measure. |
| Coverage shows that tests check the code. | Three PRs on 2026-10-21 had 100% coverage and a test that did not probe the change (F2.7). | A test index must measure fault detection (mutants, revert), not lines run. |
| Code is cheap to read and costly to write. | An agent writes code at almost no cost. It spends most effort to find where to edit (P3.6). | Code indexes must measure locality and depth, not only size. |
| A reviewer is independent of the author. | Today all reviewers and auditors run on the same model family (F2.9). | A judge index must measure recall on known defects and family diversity. |

*Synthesis:* an index in this loop must state its uncertainty. It must read data that the measured agent cannot change. It must not feed back into the prompt of that agent.

## 2. Local evidence

All counts are at `main` `e0b8433fe` on 2026-10-21. The programs are in the scratch directory of this session (`scratchpad/qidx/` and `scratchpad/modmetrics/`). The spec gives each formula, so the collector component can reproduce each number.

**F2.1** Wave records exist for waves 82 to 140: `.evolve/waves/wave-<n>.json` holds `number` and `cycle_floor`. A wave owns the cycles from its `cycle_floor` up to the floor of the next wave. The records do not hold outcomes.
*Implication:* the wave of a cycle is derivable, but each outcome must come from the cycle records.

**F2.2** Each retained cycle has `.evolve/runs/cycle-<n>/run.json` (fields `shipped`, `final_verdict`), `ship-binding.json` (`commit_sha`), `triage-decision.json` (`top_n[].id`) and `signals.ndjson`. Run directories exist from cycle 1819 to 2028 (210 directories). In cycle 2024, `run.json` says `final_verdict: PASS`, but its `cycle.sealed` signal says `final verdict FAIL` (`.evolve/runs/cycle-2024/signals.ndjson`, seq 127).
*Implication:* `run.json` `final_verdict` is not a reliable outcome. The ship fact comes from `shipped` or the `ship.landed` signal, and the stop fact from `cycle.sealed`.

**F2.3** Ship rate over waves 82 to 140 is 119 of 186 cycles: 64.0%, Wilson 95% [56.9, 70.5]. The last 10 waves give 28 of 66: 42.4% [31.2, 54.4]. Waves 137 to 140 give 2/5, 1/6, 2/5 and 3/5. One wave has 4 to 16 cycles, so the interval of one wave spans 40 to 70 points.
*Implication:* a change in the ship rate of one wave tells almost nothing. Comparisons must pool waves and state the MDE (§4).

**F2.4** In the wave window, 130 items had at least one try, and 17 items had two or more tries. Of the 17, 13 shipped at last (76.5% [52.7, 90.4]). No item shipped twice, because a ship consumes the item.
*Implication:* production data cannot give pass^k for k ≥ 2. A pass^k index needs a replay harness that runs the same frozen item k times (synthesis).

**F2.5** Of 186 cycles, 30 have no `cycle.sealed` signal (a stop from outside, such as a lease deadline): 16.1% [11.5, 22.1]. One more sealed with `stopped: lane deadline`. Together, 31 of 186 = 16.7% [12.0, 22.7] end by a deadline.

Nine show a system-failure code (`ORCHESTRATOR_SYSTEM_FAILURE`, `LOOP_SYSTEM_FAILURE_HALT` or a `BRIDGE_EXIT_*_TIMEOUT` code): 4.8% [2.6, 8.9]. The ledger has 10 inbox releases for a lease deadline (`kind: inbox-lifecycle`, `action: release`).
*Implication:* about one cycle in six ends for a reason that is not about the item. A ship-rate change must be read next to this rate.

**F2.6** Shipped cycles change a median of 7 files (bootstrap 95% [6, 7]), 2 Go packages ([2, 2]) and 428 lines ([305, 480]). The sample is 119 ships. The source is `git show --numstat <commit_sha>` without `.evolve/` paths.
*Implication:* a locality baseline exists. The tokens that an agent reads before its first edit are not recorded in `llm-calls.ndjson`, so that part of the index has no data.

**F2.7** On 2026-10-21, PRs #1103, #1106 and #1107 each had 100% coverage on the changed functions and a test that did not probe the change. Only the independent reviewers found this, by a revert of the change (operator evidence; `dev/console-notes/reviews/pr-1103.md`, `pr-1106.md` §Q3, `pr-1107.md`). In the regression test-impact soak, the rule "zero misses" held by construction, because `would_skip` was always 0: "43 of 43 regression packages reach an escape hatch" (`dev/console-notes/reviews/pr-1098-r2.md:34-37`).
*Implication:* coverage and a zero-miss rule can both be green and prove nothing. A test index must fail when the change is reverted, and each rule needs a check that it is not vacuous.

**F2.8** The independent review files `dev/console-notes/reviews/pr-*.md` start with `VERDICT:`. Of 98 files, 62 say PASS, 35 FIX and 1 BLOCK. 41 of 98 PRs had at least one HIGH or CRITICAL finding: 41.8% [32.6, 51.7]. Each of these PRs had passed the review of its builder.

The severity is free text, so the count uses a pattern match.
*Implication:* the builder review misses a HIGH finding on about two PRs in five. The independent review is the strongest judge that we have, and its output needs a structured severity field.

**F2.9** In the retained cycles, all 234 audit calls in `llm-calls.ndjson` ran on `claude-tmux`. Build calls ran on `claude-tmux` (295), `agy-tmux` (95) and `agy-claude-tmux` (18).
*Implication:* the judge never comes from a second model family. Self-preference bias is not measurable today (P3.8).

**F2.10** At `e0b8433fe`, 88.8% of 11,296 production functions meet four limits. The limits are 30 lines, cyclomatic complexity 10, nesting depth 3 and 4 parameters. Of 11,256 functions in the coverage profile, 83.5% meet these limits and also have 100% coverage. The scanner is a Go AST walker joined with `go tool cover -func`.
*Implication:* most units are small. The index is a ratchet on changed functions, not a campaign on the whole tree.

**F2.11** Per internal package, the median of non-blank logical lines per exported symbol is 15.2 (p10 6.4, p90 44.2, 316 packages). 99 packages have fewer than 10 lines per exported symbol, for example `internal/ipcenv` (25 lines, 12 symbols).
*Implication:* many packages are shallow: their interface is almost as large as their body (P3.7).

**F2.12** In production Go, 3,550 five-line windows of normalized lines occur in two or more places: 22.7 per 1,000 logical lines (156,436 lines).
*Implication:* a duplication baseline exists for the tree. A rate per added line needs a diff-scoped scanner.

**F2.13** First-parent commits on `main` that are 14 to 30 days old (274 commits) added 47,424 production Go lines. 4,271 of them (9.0%) changed or were removed within 14 days (`git blame --first-parent` at the snapshot 14 days after each commit). For all `.go` and `.md` lines, the rate is 3.8% (10,889 of 286,701).
*Implication:* a churn baseline exists. The method counts a line as churned when a later first-parent commit owns it, so a pure move also counts.

**F2.14** In 30 days, 695 first-parent commits reached `main` and none has a subject that starts with "Revert". The loop repairs forward with a new item.
*Implication:* a revert rate from commit subjects is near zero and says little. Revert and escape need a structured link from the fix to the change that it repairs.

**F2.15** Test code is 3.06 times the size of production code (operator evidence, 2026-10-21).
*Implication:* test size is not the gap. Test value per test is the question (TQ3).

**F2.16** Two counts complete the baselines. First, 49 lines in `agents/*.md` (37, in 106 files) and `skills/*/SKILL.md` (12, in 34 files) match `^\s*[-*0-9.]+\s*\**(If|When|Unless)\b`. The pattern also takes a line that opens with a bold mark. Second, 36 of 186 cycles (19.4% [14.3, 25.6]) end by a deadline or show a system-failure code; 4 cycles are in both counts.
*Implication:* the judgment-surface proxy has a census baseline, and the two PQ4 counts overlap, so their sum is not a rate.

## 3. Current practice

**P3.1** τ-bench defines pass^k as the chance that all k i.i.d. tries of a task succeed. A model with 90% pass@1 falls to about 57% at k = 8. On τ²-bench retail, a score of 81.6% at k = 1 fell to 56.1% at k = 4 Sources: [τ-bench paper](https://arxiv.org/pdf/2406.12045), [Sierra blog](https://sierra.ai/blog/tau-bench-shaping-development-evaluation-agents), [prefactor summary](https://prefactor.tech/learn/agent-benchmarks).

The estimator per task is C(c, k) / C(n, k) for c passes in n tries, averaged over tasks. The search did not show this formula in a primary source. It is the complement of the unbiased pass@k estimator 1 − C(n−c, k) / C(n, k) of Chen and others. Sources: ([benchmarkingagents pass-at-k](https://benchmarkingagents.com/pass-at-k), [HLS-Eval](https://arxiv.org/pdf/2504.12268)).

**P3.2** An aggregate pass^k hides change per task, so a comparison of two configurations must pair the tasks ([arXiv 2604.17849](https://arxiv.org/pdf/2604.17849)). An audit of benchmark noise shows that many reported gains are inside the noise floor ([arXiv 2608.22331](https://arxiv.org/pdf/2608.22331)).

**P3.3** Reward hacks are common and measurable:
- ImpossibleBench gives tasks whose tests cannot pass honestly. GPT-5 "passed" 54% of them by a game of the tests. Hidden tests cut this to near zero. Read-only tests stopped test edits but not special cases in the code ([LessWrong summary](https://www.lesswrong.com/posts/qJYMbrabcQqCZ7iqm/impossiblebench-measuring-reward-hacking-in-llm-coding-1), secondary).
- EvilGenie measures reward hacks in a coding environment with held-out tests and LLM judges ([arXiv 2511.21654](https://arxiv.org/html/2511.21654v2)).
- The Reward Hacking Benchmark reports that RL post-training raised exploit rates from 0.6% to 13.9% ([ResearchGate](https://www.researchgate.net/publication/404477198_Reward_Hacking_Benchmark_Measuring_Exploits_in_LLM_Agents_with_Tool_Use), secondary host).
- SpecBench ([arXiv 2605.21384](https://arxiv.org/pdf/2605.21384)) and BenchJack ([arXiv 2605.12673](https://arxiv.org/pdf/2605.12673)) also measure the gap between the check and the intent.

**P3.4** Meta ACH generates mutants for one concern with an LLM, then generates tests that kill them. It ran over 10,795 classes, with 9,095 mutants and 571 tests. Engineers accepted 73% of the tests. An LLM equivalent-mutant detector reached precision 0.95 and recall 0.96 after preprocessing ([arXiv 2501.12862](https://arxiv.org/html/2501.12862v1)).

**P3.5** Gremlins, a Go mutation tool, reports each mutant as KILLED, LIVED, NOT COVERED, TIMED OUT or NOT VIABLE ([gremlins](https://redirect.github.com/go-gremlins/gremlins)). Stryker counts a timeout as detected and divides by valid mutants ([Stryker mutant states](https://stryker-mutator.io/docs/mutation-testing-elements/mutant-states-and-metrics/)).

**P3.6** Agents spend much of their effort to explore, not to edit ([arXiv 2604.13108](https://arxiv.org/pdf/2604.13108)). Locality and a small blast radius help an agent ([maintainable.software](https://maintainable.software/agentic-engineering-part-2-agentic-codebase-principles/)).

**P3.7** A deep module hides much behavior behind a small interface. AI makes a split cheap, which risks "a fractal of small modules with no center" ([mejba.me](https://www.mejba.me/blog/improve-codebase-architecture-skill-deep-modules), [aihero.dev](https://www.aihero.dev/how-to-make-codebases-ai-agents-love)). Both are practitioner blogs.

**P3.8** For LLM judges, current practice puts deterministic checks first. It calibrates the judge per rubric dimension. It aims for 75% to 90% agreement with people (vendor claim). Self-preference bias argues for a judge from another model family ([Arthur](https://www.arthur.ai/column/can-you-trust-llm-as-judge-calibrating-automated-evals), [LangChain](https://www.langchain.com/resources/llm-as-a-judge)).

**P3.9** GitClear (observational) reports that moved or refactored lines fell from about 25% to under 10% from 2021 to 2024. Duplicated blocks of 5 or more lines rose eight times in 2024. Source: [DevClass summary](https://devclass.com/2025/02/20/ai-is-eroding-code-quality-states-new-in-depth-report/), secondary. The 2026 figures are not verified.

**P3.10** Classic limits: SonarSource sets a default cognitive complexity of 15 ([Sonar community](https://community.sonarsource.com/t/s3776-reason-for-the-current-default-value-of-15/127103)). The `cyclop` linter sets a default cyclomatic limit of 10. A change-detector test fails on each change and finds no defect ([Google Testing Blog](https://testing.googleblog.com/2015/01/testing-on-toilet-change-detector-tests.html)).

**P3.11** Statistical metamorphic testing repeats each relation, because one violation in a random system can be noise ([arXiv 2605.13898](https://arxiv.org/pdf/2605.13898)).

## 4. Statistics for small samples

**P4.1** The Wilson score interval for k successes in n tries, with p̂ = k/n and z = 1.96:

```
centre     = (p̂ + z²/(2n)) / (1 + z²/n)
half-width = z / (1 + z²/n) × sqrt( p̂(1 − p̂)/n + z²/(4n²) )
```

It stays inside [0, 1] and keeps coverage near 95% for small n, where the Wald interval fails ([Brown, Cai and DasGupta](https://projecteuclid.org/download/pdf_1/euclid.ss/1009213286), [metricgate](https://metricgate.com/blogs/wilson-vs-agresti-coull-vs-clopper-pearson/)). Worked example: wave 140 counted as 3 ships in 4 cycles gives 75.0% [30.1, 95.4]. Counted from its 5 run directories, it gives 3/5 = 60.0% [23.1, 88.2].

**P4.2** A bootstrap interval resamples the data with replacement many times and reads the 2.5th and 97.5th percentiles of the statistic. For clustered LLM data, resample whole clusters, such as all cycles of one wave ([Indeed engineering](https://engineering.indeedblog.com/blog/2026/07/bootstrap-confidence-intervals-for-llm-evaluation/)).

**P4.3** Take two proportions with equal arms of size n, a two-sided α = 0.05 and 80% power. Then the MDE is (1.96 + 0.84) × sqrt(2 p̄(1 − p̄) / n). Sources: [arXiv 2305.16459](https://arxiv.org/pdf/2305.16459), [metricgate MDE](https://metricgate.com/blogs/minimum-detectable-effect-explained/).

Worked example with p̄ = 0.42: one wave of 5 cycles per arm gives an MDE of 87 points. 66 cycles per arm give 24 points. A 10-point MDE needs about 382 cycles per arm.
*Implication (synthesis):* a configuration change cannot be judged from one or two waves. A paired replay of the same items is the only fast path (P3.2).

## 5. Adopt or reject

| Practice | Decision | Reason |
|---|---|---|
| pass^k with the C(c, k)/C(n, k) estimator (P3.1) | Adopt, from a replay harness | Production data gives no repeats (F2.4). |
| Paired comparison per task (P3.2) | Adopt | Wave samples are too small for unpaired tests (P4.3). |
| Hidden and read-only acceptance checks (P3.3) | Adopt as an index (held-out share) | It is the counter-measure with the largest measured effect. |
| LLM-made mutants for one concern (P3.4) | Defer | Start with gremlins on changed functions, which costs no LLM call. |
| Mutation efficacy on changed functions (P3.5) | Adopt | It measures fault detection, which coverage did not (F2.7). |
| Locality, depth and duplication (P3.6, P3.7, P3.9) | Adopt in report stage | The baselines exist (F2.6, F2.11, F2.12), but the evidence is from blogs and observation. |
| Judge calibration on seeded defects, cross-family judge (P3.8) | Adopt | Escalation is high (F2.8) and the judge family is single (F2.9). |
| One composite quality score | Reject | A single number invites a game of its easiest term (Goodhart), and hides which layer moved. |
| A vendor metric suite as it is | Reject | Vendor suites assume a human author and give no interval. |
| Cognitive complexity 15 as the limit (P3.10) | Reject for now | `cyclop` 10 matches the scanner that we have. Cognitive complexity needs a new parser. |

## 6. Refinements to adopt

- **R1** Every rate carries a Wilson 95% interval, and every median a cluster bootstrap interval (P4.1, P4.2).
- **R2** A stage flip needs a sample large enough that the MDE is smaller than the effect that the flip claims (P4.3).
- **R3** Indexes read only data that the measured agent cannot write: Git history, the ledger, signals and reviewer files (F2.2, P3.3).
- **R4** The outcome of a cycle comes from `shipped`, `ship.landed` and `cycle.sealed`, never from `run.json` `final_verdict` alone (F2.2).
- **R5** pass^k comes from a replay harness, not from production cycles (F2.4).
- **R6** Each rule that claims "zero misses" must also show that its denominator is not empty (F2.7).
- **R7** Reviews and escapes need structured fields, so that the count does not depend on a pattern match (F2.8, F2.14).
- **R8** The indexes are a panel. No index feeds the prompt of the agent that it measures.

## 7. Strength of the evidence

| Source | Strength | Used for |
|---|---|---|
| Local measurements (F2.x) | Strong for this repository, one snapshot | Baselines |
| τ-bench paper (P3.1) | Peer-reviewed preprint | PQ2 |
| arXiv 2604.x, 2605.x, 2608.x (P3.2, P3.6, P3.11) | Preprints, not reviewed | PQ2, PQ3, CQ2 |
| ImpossibleBench (P3.3) | Secondary summary | TQ4 |
| Reward Hacking Benchmark (P3.3) | Secondary host | TQ4 |
| Meta ACH (P3.4) | Industrial paper, one company | TQ1 |
| GitClear (P3.9) | Observational, vendor, secondary | CQ4, OQ2 |
| Deep-module posts (P3.7) | Practitioner blogs | CQ3 |
| Judge agreement 75% to 90% (P3.8) | Vendor claim | JQ1 |
| Wilson, bootstrap, MDE (§4) | Textbook statistics | All rates |
