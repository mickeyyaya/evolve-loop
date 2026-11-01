# Code review at Anthropic: what the Claude Code team does and what evolve-loop can adopt (research, 2026-10)

Date: 2026-10-27. Method: the article was fetched on 2026-10-27, and each local claim names its file and line. "Synthesis" marks our own inference. This document is research only. It has no plan and no ADR. The operator said: "Research on <url> and generate the report".

## Table of contents

1. Purpose
2. Source
3. Summary of the article
4. Comparison with evolve-loop
5. Recommendations
6. Limits

## 1. Purpose

This document answers one question: which review practices of the Claude Code team can lower the LLM minutes that evolve-loop spends on review? It lists what the article says. It then compares each practice with the repository. It ends with ranked recommendations.

Terms used throughout:

- **Round**: one review of a change, then one fix by the author.
- **Verifier vote**: a separate group of agents that checks a finding before the pipeline posts it.
- **Hand-off finding**: a finding that the lane cannot fix, as ADR-0158 defines it.
- **Floor**: the deterministic checks that run before the audit.

## 2. Source

- Title: "How the Claude Code team does code review".
- Authors: Addy Osmani, Morgan Westlee Lunt and Qing Wang.
- Published: 2026-10-15, in the engineering section of the claude.dev blog.
- Link: https://claude.dev/blog/solving-code-review-at-anthropic/.
- The article has eleven headings. The first is "Rethinking the purpose of code review". The last is "Where this is going".

The fetch tool returned a summary of the page, not the full text. The quotes below are the exact sentences that the tool returned. Section 3 marks any value that came only from the chart of the article.

## 3. Summary of the article

### 3.1 The problem

The authors treat human review time as the scarce resource. They write: "We've said before that Claude writes about 90 percent of Claude Code's own code." They moved most of the reading to machines. Their goal is the least human time, with no rise in reverts or incidents.

Result: "From June to September, merged pull requests to Claude Code's codebase more than doubled while the share that had to be reverted fell by more than 40 percent." In the Anthropic monorepo, "changes landing per day went from about 2,500 in May to more than 30,000 in September this year."

### 3.2 The life of a change

| Step | What the article describes |
|---|---|
| Before the PR | Claude runs tests, builds and linters. A subagent reviews the diff in a fresh context. Claude shows output, not a claim. "A typical CLI pull request may run dozens of checks before anyone looks at it." |
| Required CI | Lint, type checks, cross-platform tests and builds. |
| AI review | Parallel agents hunt for bugs. "Findings get verified by other agents before humans review them." "Then a separate group votes on each finding, and it only gets posted if enough of them agree it's real." |
| Severity | "Each is tagged Important, Nit, or Pre-existing, so a bug the PR didn't introduce doesn't look like the author's fault." |
| Security review | A separate pass. It can block the merge until the author fixes each finding or closes it with a reason. |
| Approval | A classifier sets the risk tier. Low risk needs no person, medium risk needs one, high risk needs two. Sensitive paths (billing, the auto-updater, CI workflows) always need a person. "An agent's approval never counts as a person's, either." |
| Merge | A merge queue re-tests against the latest main. New features go to internal users first. |
| Rollback | A flag turns a feature off without a deploy. An on-call rotation handles incidents. |

Size effect: "Most PRs over 1,000 lines get findings, and most small ones come back clean."

### 3.3 How the reviewer improves

- Past incidents become an eval set. The team scores the reviewer against it each day.
- The team measures noise by the reaction of the author to each comment: fixed, rejected, intentional or ignored.
- `CLAUDE.md` holds the conventions of the project. "`REVIEW.md`, written by code owners, tells the reviewer what matters in their part of the code."
- Review agents "also run nightly over existing code, helping us catch old bugs before they show up somewhere else."

### 3.4 The convergence fixes

The reviewer did not converge at first. "One CLI PR went through more than fifty rounds of review and fixes overnight before its author stopped it." The team ran `git blame` on the repeated flags: "about 59% of it was code that had been there since the reviewer's first pass." The team made four changes:

1. A round cap: "After three rounds, the reviewer asks the author to step back and reconsider the approach."
2. "Diff-only re-review after the first few passes."
3. "Flag the class of bug, so the author fixes every instance, not one line."
4. "No repeats of findings already posted."

### 3.5 The numbers

| Measure | Before | After | Note |
|---|---|---|---|
| PRs with substantive review comments | 16% | 54% | Quote: "went from 16% to 54%". |
| PRs that pass after one round | 65% | 78% | Chart value. |
| PRs that need five or more rounds | 8.9% | 1.5% | Chart value. |
| Mean reviews to pass | 2.08 | 1.32 | Chart value. |
| Repeat flags on code older than the first pass | n/a | about 59% | From `git blame`. |
| Longest run | n/a | more than 50 rounds | One CLI PR. |

The article does not report the precision of the reviewer, its recall, its cost or its latency. The human-time measures are approvals per PR (median and 95th percentile), human comments and the wait of a PR for a person. Without precision and recall, the 16% to 54% rise does not show that the comments are right. Synthesis: the rise can also come from more noise.

## 4. Comparison with evolve-loop

Each row below was checked in the repository on 2026-10-27, at `origin/main` commit `fb1280de0`. The author dropped each claim that the repository did not confirm.

| Topic | Article | evolve-loop today | Gap |
|---|---|---|---|
| Reviewers | Parallel bug hunters, then a vote | The cycle audit phase and the console reviews, with an attestation | No vote on a finding |
| Severity tags | Important, Nit, Pre-existing | CRITICAL, HIGH, MEDIUM, LOW | No Pre-existing tag |
| Round cap | Three, then reconsider | Never round 4 (ADR-0126) | None. We match. |
| Diff-only re-review | After the first passes | Code-review and console judges only | Not in the audit phase |
| Flag the bug class | One finding for the class | One rejection lists every floor failure | Not for audit findings |
| Cost of noise | 50+ rounds on one PR | Cycle 2035: four audit rounds, 160 LLM minutes | Same lesson |
| Incident eval set | Daily score against past incidents | Failures become lessons and ACS packages | No reviewer score |
| Risk tiers | A classifier sets the tier | Surface tiers, the fence, the gate classifier (PR open) | Near match |
| Agent approval | Never counts as a person's | Operator-only verbs | Match |
| Checks first | Runnable checks before review | The build handoff floor | Match |
| Metrics | Human time, with guardrails | LLM minutes per shipped cycle | No rounds-to-pass |

### 4.1 Reviewers

**F4.1** The audit phase has a persona and a package: `agents/evolve-auditor.md` and `go/internal/phases/audit/`. The auditor returns FAIL for any CRITICAL or HIGH issue (`agents/evolve-auditor.md:151`).

**F4.2** The console reviews run through `evolve agent run` profiles. The shipped profiles include `adversarial-review`, `code-reviewer`, `diff-architecture-reviewer`, `diff-code-reviewer`, `diff-code-simplifier`, `diff-docs-review` and `diff-go-reviewer` (`go/cmd/evolve/testdata/cli_routing_shipped_routes.golden.json:58`, `:545`, `:910`, `:937`, `:962`, `:987`, `:1012`).

**F4.3** The commit gate attests which reviewers ran. The attestation has the field `reviewers_run` (`go/internal/commitgate/attestation.go:13`).

**F4.4** The diff reviewers read changed lines only. The text says "changed lines only; unchanged code only if CRITICAL and adjacent" (`agents/evolve-diff-code-reviewer.md:29`). Synthesis: each diff reviewer has one generic brief, and no brief carries rules for one area of the repository.

**F4.5** The auditor uses the labels HIGH, MEDIUM and LOW (`agents/evolve-auditor.md:166`). A search of the auditor persona, its reference and `go/internal/phases/audit/audit_rules.go` for "pre-existing" returns no match. Synthesis: there is no Pre-existing tag in the audit phase.

**F4.6** No document or code that this research read describes a vote of separate agents on each finding before it blocks. ADR-0126 decision 6 comes closest. Each blocking finding passes a falsification check that sees only the diff (`docs/architecture/adr/0126-every-iterative-loop-converges-or-escalates.md:55-56`). That check is one refutation pass, not a vote, and it covers late findings only.

*Implication:* we have the reviewers. We lack the filter between a finding and a block.

### 4.2 Convergence

**F4.7** ADR-0126 sets at most three fix rounds and "No round 4" (`docs/architecture/adr/0126-every-iterative-loop-converges-or-escalates.md:43`). After round 2 the bar rises to HIGH, and lower findings are filed (`:42`). This matches the round cap of the article.

**F4.8** From the second judgment, code-review and console judges verify the earlier findings and the hunks of the fix. A late finding on unchanged code below HIGH is filed, not blocking (`:52-55`). Synthesis: this is a form of diff-only re-review and of the Pre-existing tag, for code-review and console only.

**F4.9** ADR-0126 decision 9 leaves the audit untouched: "CRITICAL is never deferred, and no audit FAIL becomes a PASS" (`:64`). The audit phase therefore has neither diff-only re-review nor a Pre-existing tag.

**F4.10** ADR-0158 adds a fixability stop, the audit-fix rung and a cycle spend limit (`docs/architecture/adr/0158-a-repair-round-runs-only-for-a-finding-the-lane-can-fix-and-the-auditor-applies-a-small-prescribed-fix.md:27-33`, `:34-43`, `:44-49`). A hand-off finding that blocks stops the cycle at once.

*Implication:* the round cap matches the article. The audit phase still re-reviews the whole tree each round.

### 4.3 The cost of noise

**F4.11** Cycle 2035 ran four audit rounds and used 160.1 LLM minutes. It did not ship (`docs/incidents/2026-10-27-cycle-2035-repair-loop.md:13`, `:41`, `:44`, `:46`, `:50`). Round 2 added the new finding M3. Round 3 added the new finding H1. After the first audit, the cycle spent 104.3 minutes, 65% of the total (`:15`).

*Implication:* this is the lesson of the article. A reviewer that finds new items each round does not converge. The article answers with diff-only re-review and a bug-class flag.

### 4.4 Incidents as an eval set

**F4.12** We turn failures into lessons and into regression packages in `go/acs/cycle<N>` and `go/acs/regression`. Both directories exist at this commit.

**F4.13** ADR-0153 defines the index JQ1, seeded-defect recall. Its source is a corpus of "past real defects re-applied to a frozen tree" (Q8). Its baseline reads "no data" (`docs/architecture/llm-quality-indexes.md:59`, `:649`). The corpus idea is the same as the daily eval set of the article.

**F4.14** The JQ1 text has no trigger that reruns the corpus when a reviewer prompt or a reviewer model changes (`docs/architecture/llm-quality-indexes.md:637-660`). Synthesis: the corpus is planned but not wired to those changes.

### 4.5 Risk tiers and approval

**F4.15** The self-healing design defines three tiers: T0 kernel, T1 pipeline code and T2 item code (`docs/architecture/llm-isolation.md:1413-1425`). A deterministic diff classifier (X15) flags a T0 change at the build floor and at ship (`docs/architecture/llm-isolation.md:1436-1438`). Its code is in PR #1210, which is open. It is not in `origin/main`.

**F4.16** ADR-0157 narrows the fence to the files that enforce a guarantee. A lane can work a protected item only as a held proposal (`docs/architecture/adr/0157-the-loop-works-its-own-code-a-narrow-fence-and-held-proposals.md:33-35`).

**F4.17** "Only `evolve landing approve` writes an approval. It is a class D operator verb" (`:36`). The command requires the flag `--operator` (`go/cmd/evolve/cmd_landing.go:17`). This matches "An agent's approval never counts as a person's".

*Implication:* in the article, the change sets the tier. In evolve-loop, the surface sets the tier. The idea is the same.

### 4.6 Checks first

**F4.18** The build handoff floor runs deterministic checks before the audit (`go/internal/core/build_handoff_floor.go:21-37`). The mandatory check is the explanation document. The composed floor adds size, coverage, apicover and STE checks, as the fix commit `baf69a77a` records.

**F4.19** Commit `baf69a77a` (PR #1203) made one rejection list every floor failure. Before it, the one permitted correction fixed one gap and failed on the next. That was 5 of the 7 failed cycles of wave 143. This matches "Flag the class of bug". It applies to floor checks, not to audit findings.

### 4.7 Metrics

**F4.20** The baseline is 940 LLM minutes for 14 shipped cycles, which is 67.1 minutes per shipped cycle, for waves 141 to 143 (`dev/console-notes/llm-minutes-baseline-2026-10-27.md:17`). The same note shows a FAIL rate of 7 of 16 (`:19`).

**F4.21** A search of the baseline note for a rounds-to-pass measure and a recall measure found none. ADR-0158 adds the index PQ5, cost per shipped cycle, to the quality panel (`docs/architecture/llm-quality-indexes.md:58`). The article measures human time. We measure LLM minutes. Both are cost measures.

## 5. Recommendations

The order is by expected saving of LLM minutes, highest first. "Small" means one lane can build it from a clear brief. "Design" means a plan is needed first. The operator decides which rows to file as inbox items.

| Rank | Recommendation | Gap that it closes | Size | Route |
|---|---|---|---|---|
| 1 | Re-review only the diff of the fix in audit rounds 2 and later. | F4.9, F4.11: whole-tree re-review finds new items each round. | Design | Console (audit phase) |
| 2 | Run a verifier vote on each finding before it blocks. | F4.6: nothing filters a noisy finding. | Design | Console (audit phase) |
| 3 | Add a Pre-existing tag. A Pre-existing finding is filed and does not block. | F4.5, F4.9: old-code findings can block an audit. | Small | Console (audit phase) |
| 4 | Flag the bug class: one audit finding names every instance. | F4.19: the fix covers floor checks only. | Small | Console (auditor persona) |
| 5 | Track rounds-to-pass per cycle in the signal center. | F4.21: no measure of review convergence. | Small | Lane |
| 6 | Build the Q8 corpus of past real bugs for JQ1. Rescore it on each reviewer prompt or model change. | F4.13, F4.14: JQ1 has no data and no trigger on a prompt or model change. | Design | Lane |
| 7 | Add REVIEW.md-style rules for each area: what failure looks like, a cap on nits, file:line evidence. | F4.4: reviewers share one generic brief. | Design | Lane |

Reasons for the order:

- Ranks 1 to 3 attack the cost that cycle 2035 shows (F4.11). Rank 1 stops new items each round. Rank 2 removes false items. Rank 3 removes old-code items.
- Rank 4 is small, and its saving is smaller. The fix for floor checks already removed most of the waste.
- Ranks 5 and 6 save no minutes directly. They show if ranks 1 to 4 work. Rank 5 comes first because it needs no design.
- Rank 7 is last. The article gives no number for it. Synthesis: its saving is lower than the others.

What changes our mind: if rank 5 shows a mean of 1.5 rounds or fewer to pass the audit, ranks 1 and 2 save little. Then build rank 3 and stop.

## 6. Limits

- The article is a vendor account of its own practice. It gives no precision, recall, cost or latency. We cannot say that its changes give the same gain here.
- The numbers 65% to 78%, 8.9% to 1.5% and 2.08 to 1.32 come from a chart that the fetch tool described. The exact article text was not available.
- The article covers human reviewers and Anthropic's own repository. Our reviewers are agents, and our authors are agents.
- The gate classifier is in an open PR. This document states its design, not its behavior.
- The ranking is an estimate. No local number yet gives the saving of each item.
