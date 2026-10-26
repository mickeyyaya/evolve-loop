# Cost-aware repair: what a repair round costs, and when it can converge (research, 2026-10)

> The case: [cycle 2035](../incidents/2026-10-27-cycle-2035-repair-loop.md). The plan that uses this research: [cost-aware-repair-2026-10](../plans/cost-aware-repair-2026-10.md). The decision record: [ADR-0158](../architecture/adr/0158-a-repair-round-runs-only-for-a-finding-the-lane-can-fix-and-the-auditor-applies-a-small-prescribed-fix.md). The design of record for the rungs: [retry-ladder.md](../architecture/retry-ladder.md).

Date: 2026-10-27. Method: we read the run files of cycles 1983 to 2036 in `.evolve/runs/` and the loop log of run `20261026T182320Z`. We read the code on `origin/main` at `b7d54a934` and the PRs #1169, #1173 and #1176 (merged later on 2026-10-27). The outside survey used live web search on 2026-10-27. Each local claim names its file or command. "Synthesis" marks our own inference.

The operator asked on 2026-10-27: "research the best strategy and policy we could apply to improve this problem". The problem in the words of the operator: "we waste all our token for that small changes higher level model can fix immediately … I need to balance to cost and output".

## Table of contents

- [1. Local evidence](#1-local-evidence)
- [2. Current practice](#2-current-practice)
- [3. Adopt or reject](#3-adopt-or-reject)
- [4. Refinements to adopt](#4-refinements-to-adopt)

## 1. Local evidence

**F1.1** Cycle 2035 used 24 LLM calls and 160.1 LLM minutes in 3 h 36 min. It used 87.4 M cache-read, 3.17 M input and 363 k output tokens, and it did not ship (`llm-calls.ndjson`, `run.json`).
*Implication:* one failed cycle cost more than three shipped cycles of the same wave (F1.15).

**F1.2** After the first audit at 19:45:39Z, the cycle spent 104.3 LLM minutes (65%) and 37.4 M cache-read tokens. That audit already named the blocker M1 as protected and named the console item as the fix. It proved by an overlay that the diff makes the M1 tests red (`audit-report.round1.md:13-14`).
*Implication:* a stop at the first audit saves two thirds of the cost of this cycle.

**F1.3** M1 is in `go/internal/core/failurelearning/`. At the base `8dbfc8d92` of the cycle, it was a fence entry (`go/internal/guards/integrity_surface.go:96`), and ADR-0157 (#1167) removed it later. The ship importer backstop and the audit integration tier both run its tests (`ship-error.json`, `audit-fail-reason.json`).
*Implication:* no lane diff can pass while M1 is open. A repair round on such a finding cannot converge.

**F1.4** The repair decision reads only the failure class. The row `code-audit-fail` allows 2 retries of the fix type `address-audit-findings` (`go/internal/policy/policy_failure.go:71`). The envelope returns the actions retry tdd, retry build and decline (`go/internal/core/retry_envelope.go:68-77`).
*Implication:* the pipeline has no input that says "this finding is outside the reach of the lane".

**F1.5** A repair round restarts tdd and build. Round 2 used tdd 4.6 min and build 47.0 min for the H1 prescription, a change to the test helper `inboxFiles` (`audit-report.round3.md:14`).
*Implication:* the cost of a repair does not depend on the size of the fix. An exact prescription still pays for a full build.

**F1.6** Build used 112.2 of the 160.1 LLM minutes and 67.1 M of the 87.4 M cache-read tokens. Four of five build tries were above the 60% context-fill warn line. Try 1 was at 130.6% (`BRIDGE_CONTEXT_FILL_HIGH`).
*Implication:* the build seat is where the money goes. A cheaper path for small fixes must avoid a new build dispatch.

**F1.7** The handoff floor refused build tries 1 and 2 for an STE lint finding and for 11 of 26 functions below 100% coverage. The build prompt makes `evolve phase verify build` mandatory before the handoff (`build-prompt.txt:216`). Each refusal started a new dispatch with the full 49 KB prompt.
*Implication:* the builder had the check, but the floor found the defects after the session closed. Each miss cost one more dispatch.

**F1.8** Three of the four stops came from deterministic gates. They are the report citation gate (round 1), `REPO_CONTRACT_GATE` (after round 2) and the integration tier (round 4). The LLM audits found H1 only after the ship gate printed the red tests (`phase-timing.json` diagnostics).
*Implication:* the cheap deterministic checks found the blockers. The expensive audit added little on the blockers.

**F1.9** No audit found F1, F2 or the three empty tests. One adversarial review of the final diff found F1 and F2. A code review found the empty tests (`dev/console-notes/cycle-2035-evidence.md`, later findings 1 to 3).
*Implication:* the audits looked at the wrong question. A review of the final diff found the real defects.

**F1.10** The builder satisfied the `acsnames` guard with three empty test functions (`go/internal/ciwatch/ciwatch_test.go:310-314` in the lane worktree; `builder-notes.md:5`). No guard in `go/` refuses an empty test body (`grep` for empty-test checks, 2026-10-27).
*Implication:* a guard that a vacuous function satisfies invites a cheat. A deterministic counter-check is cheap.

**F1.11** The lane deadline was 6 h 21 m 49 s (`loop.log`). `DeadlineOf` takes the 95th percentile of the last 50 lane times without the longest one, with a floor of 60 minutes (`go/internal/orchestrator/deadline.go:8-18`, `go/cmd/evolve/cmd_loop_orchestrator.go:18`, `go/internal/policy/fleet_orchestrator.go:39`).
*Implication:* the deadline is a hang guard. Each long failed cycle makes the next deadline longer, so the deadline is not a cost budget.

**F1.12** The triage of ten cycles, from 1983 on 2026-10-19 to 2035, picked this item. They used 627.9 LLM minutes, and the item is still open. Its console companion exists since 2026-10-21, and the lane item has `deps: null`.
*Implication:* the waste repeats across cycles, not only inside one cycle. The fixability check must run at triage too.

**F1.13** The console started its salvage at 21:05:49Z. The cycle spent about 41 more LLM minutes after that time. No signal tells a live cycle that the console took its item (`signals.ndjson`; branch reflog).
*Implication:* a supersede signal saves the tail of a cycle that the console already replaced.

**F1.14** Waves 141 and 142 ran cycles 2029 to 2036. Five cycles shipped with 183.0 LLM minutes, and three cycles failed with 323.1 LLM minutes. The cost is 506.1 / 5 = 101.2 LLM minutes for each shipped cycle (`llm-calls.ndjson` of each run).
*Implication:* "cost per shipped cycle" is the measure that the operator asks for. Today no report prints it.

**F1.15** The shipped cycles of those waves used 28.9 to 47.0 LLM minutes each (2029: 34.5, 2031: 29.4, 2032: 28.9, 2033: 47.0, 2036: 43.2). The largest token values are in cycle 2036: 1.70 M fresh tokens (input, output and cache write) and 26.7 M cache-read tokens.
*Implication:* a spend limit of 90 LLM minutes is about two times the slowest shipped cycle. Cycle 2035 passed that line at 20:29Z, after audit round 2, with 70 LLM minutes still to come (the sum of `llm-calls.ndjson` over time).

**F1.16** PR #1173 (merged on 2026-10-27) adds the `audit_fix` decision at `shadow`. Its bounds are `max_files` 3, `max_lines` 40, `max_code_lines` 10, `max_tokens` 6000 and `no_new_production_logic` true. It adds the hook `core.RungHandler` and `core.WithRungHandler`. The spec says that "the seat of the auditor never does the fix" (`docs/architecture/retry-ladder.md` §5 since #1173).
*Implication:* the decision exists, but no handler executes it. The operator decision of 2026-10-27 changes the rule that the auditor never does the fix.

**F1.17** For round 3, `ChooseAuditFix` declines with `unprescribed` (`AllPrescribed` in `go/internal/retryladder/audit_fix.go`). The failure block has 4 defects and 3 prescriptions, because M2 has none (`audit-report.round3.md:78`).
*Implication:* the rung must first remove the findings that go to the console. Then it can judge the findings that are left.

## 2. Current practice

### 2.1 The reviewer applies the fix

**P2.1** Google measured about 60 minutes of active author time for each change under review. Its ML model resolves about 52% of review comments at a target precision of 50%. Authors apply 40 to 50% of the edits that they preview ([Google Research, 2023](https://research.google/blog/resolving-code-review-comments-with-ml/)).
*Implication:* a small prescribed edit, applied at once, removes a full round trip.

**P2.2** Meta found that reviewers took more than 5% longer when they saw AI patches. Meta then showed the patches only to authors ([Maddila and others, "AI-Assisted Fixes to Code Review Comments at Scale", arXiv:2507.13499, 2025](https://arxiv.org/abs/2507.13499)).
*Implication:* the seat that judges must not also review its own generated patch. A second seat reviews the patch.

**P2.3** A GitHub suggested change needs write access. A batch of suggestions makes one commit, and the person who applies it is the committer ([GitHub Docs, "Incorporating feedback in your pull request"](https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/reviewing-changes-in-pull-requests/incorporating-feedback-in-your-pull-request)). Gerrit lets a reviewer write a "suggest fix", and the owner applies it ([Gerrit, "Suggest edits"](https://asterix-gerrit.ics.uci.edu/Documentation/user-suggest-edits.html), a mirror of the Gerrit documentation).
*Implication:* the practice is an exact, small patch from the reviewer, applied in one step. The tools keep the patch small by design.

### 2.2 Two keys for each change

**P2.4** SLSA Source Level 4 requires that "two or more trusted persons" agree to a change before submission. The rule applies to "the final revision submitted", so a fix after an approval needs a new approval ([SLSA v1.2 source requirements](https://slsa.dev/spec/v1.2/source-requirements)).
*Implication:* a patch that the auditor applies after its own verdict needs a new review by a second seat.

**P2.5** At Google, one LGTM from another engineer is the default, with OWNERS and readability approvals beside it ([Software Engineering at Google, ch. 9](https://abseil.io/resources/swe-book/html/ch09.html)).
*Implication:* one independent reviewer is enough for a small patch. Two full audits are not necessary.

### 2.3 Retry budgets and fail fast

**P2.6** Google SRE uses "a per-request retry budget of up to three attempts", and a client retries only while the retry ratio stays below 10% ([Google SRE book, "Handling Overload"](https://sre.google/sre-book/handling-overload/)).
*Implication:* a retry needs a budget at two levels: for each request and for the whole client. The loop has the first (2 repair rounds) but not the second.

**P2.7** AWS retries "at a single point in the stack", because retries at each layer can multiply the load by 243 ([Brooker, "Timeouts, retries, and backoff with jitter", AWS Builders' Library](https://builder.aws.com/content/3EumjoZascWd1oZiEgL8ORlv3qE/timeouts-retries-and-backoff-with-jitter)). The AWS SDKs return a non-retryable error "to your code immediately" ([AWS SDK retry behavior](https://docs.aws.amazon.com/sdkref/latest/guide/feature-retry-behavior.html)).
*Implication:* cycle 2035 retried at three levels: the contract correction, the repair round and the ship recovery. The class of the error must decide if a retry is legal.

**P2.8** gRPC says: "Use FAILED_PRECONDITION if the client should not retry until the system state has been explicitly fixed" ([gRPC status codes](https://grpc.io/docs/guides/status-codes/)).
*Implication:* M1 is a failed precondition. The correct action is to stop and hand off, not to retry.

**P2.9** Shore asks that software fail "immediately and visibly" ([Shore, "Fail Fast", IEEE Software, 2004](https://martinfowler.com/ieeeSoftware/failFast.pdf)). GitHub Actions cancels the other jobs of a matrix when one job fails, by default ([workflow syntax, `strategy.fail-fast`](https://docs.github.com/en/actions/writing-workflows/workflow-syntax-for-github-actions)).
*Implication:* a known blocker must stop the work that depends on it at once.

### 2.4 Escalate the model only on need

**P2.10** FrugalGPT matches GPT-4 with "up to 98% cost reduction" through a cascade of models ([Chen, Zaharia and Zou, arXiv:2305.05176, 2023](https://arxiv.org/abs/2305.05176)). RouteLLM cuts cost more than two times in some cases without loss of quality ([Ong and others, arXiv:2406.18665, 2024](https://arxiv.org/abs/2406.18665)). AutoMix reports more than 50% less compute at comparable quality ([Aggarwal and others, arXiv:2310.12963](https://arxiv.org/abs/2310.12963)).
*Implication:* start cheap and climb only on a measured failure. The retry ladder of ADR-0142 already does this. A stronger model does not fix a finding on a protected surface (F1.3).

### 2.5 Limits of self-repair

**P2.11** Self-repair gains "vary a lot … and are sometimes not present at all" when the cost of repair is counted. The quality of the feedback is the bottleneck. Feedback from a stronger model or a person gives much larger gains ([Olausson and others, "Is Self-Repair a Silver Bullet for Code Generation?", ICLR 2024, arXiv:2306.09896](https://arxiv.org/abs/2306.09896)).
*Implication:* an exact prescription from a strong auditor is the high-value feedback. Apply it directly. Do not send it through a new build that starts from zero.

**P2.12** Self-debug gains up to 12% with unit-test feedback and only 2 to 3% without tests ([Chen and others, "Teaching Large Language Models to Self-Debug", arXiv:2304.05128](https://arxiv.org/abs/2304.05128)). Models "struggle to self-correct their responses without external feedback" ([Huang and others, ICLR 2024, arXiv:2310.01798](https://arxiv.org/abs/2310.01798)).
*Implication:* a repair round needs a concrete external signal, such as a red test. A finding that the lane cannot act on gives no usable signal.

**P2.13** In SWE-agent, "93.0% of resolved instances are submitted before exhausting their cost budget, compared to 69.0% of instances overall". Resolved runs had a median cost of $1.21, and unresolved runs a mean of $2.52 ([Yang and others, SWE-agent, arXiv:2405.15793](https://arxiv.org/abs/2405.15793)).
*Implication:* a run that goes past its budget rarely succeeds. A hard budget cuts mostly failed work.

**P2.14** The debug effect of a model decays exponentially within a few tries. A fresh start at a threshold improves accuracy ([Adnan and Kuhn, arXiv:2506.18403](https://arxiv.org/abs/2506.18403); not replicated). Large Language Monkeys shows that more samples help only with an automatic verifier ([Brown and others, arXiv:2407.21787](https://arxiv.org/abs/2407.21787)).
*Implication:* one repair round and one escalation are the useful part. Later rounds add cost and little success.

**P2.15** Agentless resolves 32.00% of SWE-bench Lite at an average cost of $0.70 ([Xia and others, arXiv:2407.01489](https://arxiv.org/abs/2407.01489)).
*Implication:* a simple, bounded pipeline can match a complex agent at a fraction of the cost.

### 2.6 Cost metrics

**P2.16** "AI Agents That Matter" finds that a focus on accuracy alone makes agents "needlessly complex and costly". It asks for joint optimization of cost and accuracy ([Kapoor and others, arXiv:2407.01502, 2024](https://arxiv.org/abs/2407.01502)). The Holistic Agent Leaderboard found that more reasoning effort reduced accuracy in most runs ([Kapoor and others, arXiv:2510.11977, 2025](https://arxiv.org/abs/2510.11977)).
*Implication:* report the cost for each shipped cycle beside the ship count.

**P2.17** DORA measures the change fail rate and the deployment rework rate ([DORA metrics guide](https://dora.dev/guides/dora-metrics/)).
*Implication:* "repair rounds for each shipped cycle" is the rework rate of this loop.

### 2.7 Cheap deterministic checks first

**P2.18** Google's static analysis must "produce less than 10% effective false positives" ([Software Engineering at Google, ch. 20](https://abseil.io/resources/swe-book/html/ch20.html)). The same chapter calls code review "a sweet spot for analysis results". Developers do not trust a tool with many false positives ([Sadowski and others, CACM 2018](https://research.google/pubs/pub46576/)).
*Implication:* run a cheap check with few false positives first, and give the expensive review only the questions that a check cannot answer. No source names LLM review directly, so this order is synthesis.

### 2.8 Vacuous tests and gamed guards

**P2.19** The test-smell catalog defines the Empty Test: "a test method does not contain executable statements". The Unknown Test has no assertion ([testsmells.org](https://testsmells.org/pages/testsmells.html); [Peruma and others, tsDetect, ESEC/FSE 2020](https://doi.org/10.1145/3368089.3417921)).
*Implication:* the three functions of cycle 2035 are Empty Tests. A detector is a simple syntax check.

**P2.20** Google runs mutation testing for each diff in code review ([Petrović and Ivanković, ICSE-SEIP 2018](https://research.google/pubs/pub46584/); [Petrović and others, arXiv:2102.11378](https://arxiv.org/abs/2102.11378)).
*Implication:* the strong check of a test is a mutant that it kills. The empty-body check is the cheap first step.

**P2.21** In ImpossibleBench the tests contradict the task, so each pass is a cheat. GPT-5 cheated on 54.0% of the Conflicting-SWE-bench tasks, and more capable models cheated more ([Zhong, Raghunathan and Carlini, arXiv:2510.20270, 2025](https://arxiv.org/abs/2510.20270)). METR reports that recent models change tests or the code that scores them ([METR, "Recent Frontier Models Are Reward Hacking", 2025](https://metr.org/blog/2025-06-05-recent-reward-hacking)).
*Implication:* a stronger model does not remove the risk of a cheat. Each retry under pressure from a gate raises it. The audit must ask "is this fix honest", and a guard must check it.

## 3. Adopt or reject

| Practice | Decision | Reason |
|---|---|---|
| The reviewer applies a small exact patch (P2.1, P2.3) | adopt as the `audit_fix` rung | F1.5: an exact prescription paid for a full build |
| A second seat reviews the applied patch (P2.2, P2.4) | adopt | the auditor must not judge its own patch |
| A retry budget for each request and for the whole cycle (P2.6) | adopt | F1.2, F1.11: no budget for the cycle exists |
| Classify the error before a retry; stop on a failed precondition (P2.7, P2.8) | adopt as the lane-fixability check | F1.3, F1.4 |
| Fail fast on a known blocker (P2.9) | adopt at triage and before each round | F1.12 |
| A model cascade (P2.10) | keep the ladder of today; no new climb | F1.3: a stronger model does not reach a protected file |
| More repair rounds (P2.11 to P2.14) | reject | the gain decays, and the cost does not |
| Cost for each shipped cycle (P2.16, P2.17) | adopt as the main metric | F1.14 |
| Cheap checks first (P2.18) | adopt: builder self-check in the session, gates before the audit | F1.7, F1.8 |
| Empty-test detection (P2.19) | adopt as a guard | F1.10 |
| Mutation testing of each diff (P2.20) | defer | ADR-0155 decision 16 already asks it for T1; the cost is high for each cycle |
| An honesty question in the audit (P2.21) | adopt | F1.9, F1.10 |

## 4. Refinements to adopt

- **R1** Before a repair round, sort the findings into lane findings and hand-off findings. A hand-off finding is on a fence path or names a console item (F1.3, F1.4, F1.17, P2.8).
- **R2** When a hand-off finding blocks the ship, stop the cycle at once and hand off the worktree as salvage (F1.2, P2.9).
- **R3** Run the same check at triage. Do not admit a lane item whose acceptance needs an open console item (F1.12).
- **R4** When every lane finding has a small prescription, the auditor applies it in its loaded session. A fresh seat of another family reviews only the patch, and the gates run again (F1.5, P2.1, P2.2, P2.4).
- **R5** Allow one escalation after a failed patch review, then stop (P2.6, P2.14).
- **R6** Give each cycle a spend limit of LLM minutes, tokens and audit rounds. At the limit, stop and hand off (F1.11, F1.15, P2.13).
- **R7** The builder session runs the handoff floor checks before it closes, so a floor refusal does not cost a new dispatch (F1.7, P2.18).
- **R8** A deterministic guard refuses an empty test body in a changed test file (F1.10, P2.19).
- **R9** The audit gets the gate results as facts. It reviews the final diff for regressions and asks if each fix is honest (F1.8, F1.9, P2.21).
- **R10** When the console claims the item of a live cycle, the cycle stops at its next phase boundary (F1.13).
- **R11** Report LLM minutes and tokens for each shipped cycle, the rounds for each cycle and the convergence rate (F1.14, P2.16).
