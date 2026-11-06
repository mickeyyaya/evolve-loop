# Pipeline blockers of wave 155: what blocked the fixes, and what current practice says about signals (research, 2026-11)

> The plan that uses this research: [pipeline-blockers-2026-11](../plans/pipeline-blockers-2026-11.md). The design: [signal-accuracy](../architecture/signal-accuracy.md). The decision record: [ADR-0161](../architecture/adr/0161-a-pipeline-blocker-fix-meets-six-test-rows-and-a-signal-states-its-cause-and-next-step.md). The incident: [2026-11-05 wave 155](../incidents/2026-11-05-wave-155-false-signals-and-the-logout.md).

Date: 2026-11-05. Method: code reads at `origin/main` (`09bb331d4`) and the run files of wave 155 under `.evolve/`. I also read seven architecture review reports and the staged plan draft, ran a live web search and fetched three guides.

Each local claim names its file or command. "Synthesis" marks our own inference. This file does not decide. The plan decides.

## Table of contents

- [Terms used throughout](#terms-used-throughout)
- [1. Local evidence](#1-local-evidence)
- [2. Outside practice on error signals](#2-outside-practice-on-error-signals)
- [3. Adopt or reject](#3-adopt-or-reject)
- [4. Refinements to adopt](#4-refinements-to-adopt)
- [Gaps in this research](#gaps-in-this-research)
- [Sources](#sources)

## Terms used throughout

- **Blocker:** a defect that stops the pipeline, or that makes a signal of the pipeline false. The ids `B1` to `B11` name the eleven blockers of wave 155.
- **Block:** one review verdict FAIL on the staged diff of one fix. A fix that has two FAIL verdicts is "blocked twice".
- **Signal:** a message that the pipeline gives to an operator or an agent: a log line, a reason file, a status line or a digest.
- **Class:** the label that a signal gives to a failure, for example `infra-error` or `gate-block`.
- **Six rows:** the test plan of one fix: red proof, journey, preservation, wiring, mutation and signal. The [plan](../plans/pipeline-blockers-2026-11.md#2-facts) lists them.

## 1. Local evidence

The source of the numbers is the console measurement of 2026-11-04 and 2026-11-05. The scratch root is `S=/private/tmp/claude-501/-Users-danleemh-ai-claude-evolve-loop/f0748469-0221-4c12-b329-91782f1f5e57/scratchpad`. The runtime root is `R=/Users/danleemh/ai/claude/evolve-loop/runtime`.

### 1.1 The outcome of wave 155

**F1.1** Wave 155 ran from run `20261104T141300Z` (`R/.evolve/waves/wave-155.json`). `evolve wave status` prints "outcome: 0 of 11 cycles shipped". The ledger holds two ship entries: cycle 2159 (`c9a7ef419`, 2026-11-04T15:27:21Z) and cycle 2158 (`09bb331d4`, 2026-11-04T16:47:34Z) (`R/.evolve/ledger.jsonl`).

*Implication:* the status line is a false signal. It reports zero for a wave that landed two ships.

**F1.2** `zeroShipHalt` starts at `max(b.zeroShip.seen, batchStartCycle) + 1` (`go/cmd/evolve/cmd_loop_zeroship.go:19`). A cycle that the run resumed has a number below that start, so the halt does not count its ship. The cycle floor of wave 155 is 2160 (`wave-155.json`), and both ships (cycles 2158 and 2159) are below it. A probe halted with rc=4 on one zero-ship wave after a resumed ship (`S/round9/b5.json`). Wave 154 also printed "0 of 9" with three real ships (staged plan draft, B5; I did not repeat this count).

*Implication:* a wave whose ships are all resumed cycles can stop the loop with no real cause.

**F1.3** Cycles 2165 to 2171 show "quota pauses 1" and 2.3 to 78.7 LLM minutes to a verdict (`evolve wave status`, 2026-11-05). The cycles 2167 to 2171 stopped in the scout phase after 2.3 to 3.9 minutes.

*Implication:* `wave status` printed five scout stops and one quota pause for each cycle. Their cause is unknown (plan Q4, and see F1.12).

### 1.2 The eleven blockers

**F1.4 (B1)** The census classed the item of cycle 2161 as `console_route` at 14:23:41Z. `LOOP_ORCH_LANE_LAUNCHED` started it at 14:23:42Z (`R/.evolve/runs/cycle-2161/retrospective-report.md`, D1). At `09bb331d4`, `pausedPlan.order` added each ready paused scope to the head of the plan with no route check. The B1 fix `bb69b6092` changed this, and it landed in batch seven (`5df3fdf77`).. The fix is commit `bb69b6092` (2026-11-04) on PR #1283.

*Implication:* a source of lanes that the route filter does not cover starts an item that the pipeline refused.

**F1.5 (B2)** `committedset.Committed` uses the lane pin and subtracts only `deferred` (`go/internal/committedset/committedset.go:18-35`). The host claim floor refuses a routed item, so `checkInboxClaim` reports `missing_effect` (`go/internal/deliverable/effects.go:64`). The message says "defer or drop it". The retrospective reproduced this: the `dropped` and `escalated` forms keep the item in `committed`, and `deferred` leaves it in `unanswered` (cycle 2161 retrospective, Reproduction).

*Implication:* each answer that the pipeline prescribes fails the gate, so a correction cannot succeed.

**F1.6 (B3)** The fresh path calls `provenBlocker` (`go/internal/core/build_blocker_exit.go:118`), which runs `NewBlockerProver().Prove` (line 127). The resume path calls `resumedBlockerRoute` (`resume.go:434`). That function returns at once when `review.ProvenBlocker` is empty (`build_blocker_exit.go:167-169`).

In cycle 2157 the floor told the builder twice more to fix a protected file. The builder then wrote the file through a Bash heredoc (cycle 2157 retrospective, D1).

*Implication:* one claim has two readers, and only one reader proves it.

**F1.7 (B4)** `verifyNoControlPlaneEdits` reads `git diff --name-only --no-renames HEAD` and the untracked files (`go/internal/phases/ship/integrity.go:34-40`). In a test through the ship verb, the unstaged, staged and committed forms of a protected file all landed on main. The log line was `no control-plane (integrity-surface) paths in cycle diff`. The check read the main checkout, not the lane worktree. The build handoff floor caught all three forms (`S/round9/b4.json`).

*Implication:* two checks that judge one fact read two different trees.

**F1.8 (B4, audit)** The console audited the landed commits on 2026-11-05. Of 282 cycle commits since 2026-09-01, 6 files in 4 commits (2026-09-14 and 2026-09-15) were protected at their commit. The build floor check landed on 2026-09-26 (`b9553b9bc`). From then, 0 files.

The method matched path fragments of `integrity_surface.go` at the parent of each commit. This approximates `IsProtectedSurface`. The reason why the 6 files shipped was not checked. I did not repeat this audit.

*Implication:* the ship check gap did not show in landed history after the floor check. The history is weak proof, because the method is an approximation.

**F1.9 (B5, B6, B7)**
- The triage cell of cycle 2161 printed `command not found: evolve` (exit 127) for its correction commands. It then wrote "No inbox-claim effect is owed" with no verifier output (cycle 2161 retrospective, D3).
- `PATH` is in the base list of launcher names (`go/internal/bridge/hermetic_env.go:28`).
- Two review panes of this period printed the same error: B3 and B9 (`S/rq3/b3-architecture-reviewer.json`, `S/rq3/b9-architecture-reviewer.json`).
- The boundary of wave 155 stopped because a quarantine move deleted a tracked file. The console recorded the move by hand in `3c0090689` (2026-11-04). The fix is in cycle 2153 (staged plan draft, B7; I did not read cycle 2153).


*Implication:* a cell that cannot run the verifier states a false "done". A missing step shows as a silent pass.

**F1.10 (B8)** `classifyPreClass` returns the first rule whose words occur in the lowercase text (`go/internal/core/failure_digest.go:61-70`). The first rule holds `quota`, `bridge` and `teardown` (line 54). `AssembleFailureDigest` joins all reasons into that text (line 75). The reason of cycle 2157 holds the test name `TestResumeKeepsLadderRung_AQuotaPausedResumeCheckpointsTheRungOfItsCorrectionLoo…`. Its digest says `infra-error` (`R/.evolve/runs/cycle-2157/audit-fail-reason.json`, `failure-digest.json`).

The retrospective states that the quota evidence belongs to an earlier pause. The builder did not stop for quota.

*Implication:* a test name changed the class. The class sent the operator to the wrong cause.

**F1.11 (B8, B9)**
- The same reason lists `--- FAIL: TestF` and `FAIL	p`. The retrospective names `TestF` as floor noise (D3). By synthesis, the names `TestF` and `p` look like the output of a fixture in a nested run.
- `ISOLATION_I2_VIOLATION` appears 1,511 times in the log of wave 155 at the end of the log (2026-11-05 01:16). The console counted 530 earlier. Each launch writes 53 lines, one for each filtered launcher variable (cycle 2154: 53 lines; `R/.evolve/logs/20261104T141300Z/loop.log`). Each line is WARN with `action=refused` and `stage=enforce` (`go/internal/bridge/hermetic_env.go:164-186`, severity at line 173).


*Implication:* a routine filter fills the log with WARN lines, and a real signal is hard to see.

**F1.12 (B10)** Near 01:00 on 2026-11-05 the claude CLI was logged out. Lanes reported `quota-pause` at 56 percent weekly usage. Reviews returned an empty report and an empty verdict. The operator ran /login near 07:10 (inbox item `logout-is-reported-as-a-quota-wall`, `R/.evolve/inbox/2026-11-04T23-12-03Z-logout-is-reported-as-a-quota-wall.json`).

The commit gate refused each empty review, so nothing landed unreviewed. A search of non-test Go code for `logged out`, `not logged` and `/login` finds no match (`git grep -i` at `origin/main`, 2026-11-05). `quota-pause` is a stop reason in four files of `go/cmd/evolve`, for example `cmd_cycle_quota_deferral.go:28`.

*Implication:* by synthesis, the pipeline has no class for a logout, so a logout can only read as quota or as an empty result. I did not trace the exact code path that wrote `quota-pause`.

**F1.13 (B11)** `evolve integration record-pr` accepts an attestation only when its tree equals the digest of the whole PR diff (`AttestedBinding`, `go/internal/integration/consolepr.go:39`). The commit gate attests one staged tree. No flow gave a matching file for #1282 to #1285 (inbox item `train-record-pr-has-no-reachable-attestation`). I did not read `consolepr.go` beyond the signature.

*Implication:* the console train cannot land a normally reviewed PR, so every console fix waits for a manual path.

### 1.3 What the blocked reviews show

**F1.14** Seven fixes got a FAIL from the architecture reviewer on 2026-11-04 and 2026-11-05. The run envelopes give these counts (`S/rq3/<b>-architecture-reviewer.json`, field `findings`).

| Fix | Run label | CRITICAL | HIGH | Times blocked |
|---|---|---:|---:|---:|
| B2 | `agent-architecture-reviewer-20261105T000429Z-8d31` | 1 | 3 | 2 |
| B3 | `agent-architecture-reviewer-20261104T235342Z-b3bc` | 1 | 2 | 2 |
| B4 | `agent-architecture-reviewer-20261104T231210Z-8b3a` | 2 | 1 | 1 |
| B5 | `agent-architecture-reviewer-20261104T231817Z-c845` | 1 | 8 | 2 |
| B6 | `agent-architecture-reviewer-20261104T233252Z-a8ed` | 2 | 4 | 1 |
| B8 | `agent-architecture-reviewer-20261104T233628Z-ab46` | 2 | 3 | 2 |
| B9 | `agent-architecture-reviewer-20261104T234350Z-7cbd` | 2 | 6 | 1 |
| **Total** | | **11** | **27** | |

The column "Times blocked" comes from the brief of the console, not from the run files. B1 is committed on #1283.

*Implication:* four fixes are two blocks from the threshold in the words of the operator, "over 3 times" (see F1.16).

**F1.15** I grouped the 38 findings by the title of each finding. This is synthesis. A finding goes in the first group that fits.

| Group | Findings | Examples |
|---|---:|---|
| The signal text states a false cause, no cause, or a next step that fails | 12 | B2 C1 and H1; B3 H2 and H3; B4 C1; B8 C1 and H3; B9 C1, H3, H4 and H8; B5 H2 |
| A test cannot fail (surviving mutant) or does not use a production shape | 14 | B2 H2 and H3; B5 H4 to H7; B6 H3 to H6; B8 H2; B9 C2, H6 and H7; B4 H3 |
| One fact has two readers or two counts | 5 | B4 C2; B5 C1, H3 and H9; B8 C2 |
| A fix is wired to one of several entries | 4 | B3 C1; B6 C1 and C2; B5 H6 |
| Other | 3 | B5 H8 (dead code); B8 H1; B9 H5 |

I read the title of each finding and the full text of about half of them. The grouping is a judgment, and a different reader can move a few items.

*Implication:* about one finding in three (12 of 38) is a failure of the rule "the signal needs to be accurate". The review finds these late, after the build.

**F1.16** Examples of the first group, with the true cause:
- B2: the message for a routed item names `surface ""`, and the true cause `route:console-only` is not in the text (B2 CRITICAL 1).
- B3: a missing or corrupt snapshot gives the text "the lane changed the predicates" when no edit happened (B3 HIGH 2).
- B4: the message tells a lane to run `evolve ship --class manual`. The probe gave "no staged changes (clean exit)" and shipped nothing (B4 CRITICAL 1).
- B9: a lost pipeline variable opens a false isolation incident, because the consumer in `selfheal` excludes only `source=launcher` (B9 CRITICAL 1).


*Implication:* each message was true in the author's mind and false for the reader. A test that reads the whole text, with equality, catches this.

**F1.17** Examples of the groups "two readers" and "one of several entries":
- B5: the halt keeps its own tally (`seen`, `resumed`, `n <= floor`), and the status reads each cycle again. Two probes gave `waves=[0 0] halted=true` where `wave status` counts a ship (B5 CRITICAL 1).
- B8: the fix adds a fourth parser of `go test -json`, and the repo-contract floor still lists fixture failures (B8 CRITICAL 2).
- B3: a resume that starts at an earlier phase has no predicate base, so a true blocker never routes (B3 CRITICAL 1).
- B6: the fix reaches a tmux cell at `enforce` and not at `shadow` or `off`, and a test claims all three stages (B6 CRITICAL 1).


*Implication:* a fix that names one entry or one reader leaves the same fault on the others. The design must list every entry before the build.

### 1.4 What the rules say today

**F1.18** The operating policy already holds these rules. The console fixes a pipeline defect at once, in an isolated worktree, with TDD, dual review, a wiring proof and a CI watch (`docs/operations/operating-policy.md` §1.1). It holds the wiring proof (§3.3) and "fail loudly" (§3.7).

The convergence policy sets a ladder. Round 3 is rung 2, "change the strategy", with a fresh-context fixer or a new plan (`docs/architecture/convergence-policy.md` §3).

*Implication:* the block count exists as the round counter. The rule that the strategy change of a pipeline blocker is a redesign at the top tier does not exist.

**F1.19** No rule requires a design check before the TDD step. No rule requires that a signal states a cause and a next step. The only text near it is "fail loudly: every degraded path WARNs with specifics" (§3.7).

*Implication:* the operator rules of 2026-11-04 and 2026-11-05 add rules. They do not repeat rules.

**F1.20** The console built evidence with scratch commands: a JSON print of each report, and a line count of `loop.log`. Class A verbs exist for `signals tail`, `wave status`, `agent list` and `dossier show` (`evolve agent capabilities --json`, 98 verbs, 2026-11-05).

No verb counts signals by a field. No verb shows an agent run by its label. No verb shows the reason, class and route of a cycle in one place. This is synthesis from `evolve signals --help`, `evolve agent --help` and `evolve dossier --help`.

*Implication:* the operator question "Why do we have bash script?" has a concrete answer. Two verbs are missing from the debug family of ADR-0162.

## 2. Outside practice on error signals

**P2.1** [Write Actionable Error Messages](https://www.joshka.net/practice/patterns/write-actionable-error-messages/) (Josh McKinney, Software Practices, no date; copyright 2026; a practitioner guide, so a weak source). A fetch agent read the full page, and I rely on its report.

The page lists these elements:
- name the attempted operation and the affected item;
- say what failed or what state changed;
- "explain why only when the program actually knows why";
- distinguish blocked from skipped and degraded;
- give a safe next step, and keep correlation ids and raw diagnostics.

It warns against dead-end phrases such as "failed" and "invalid". It does not discuss runbooks by name or classification by text.

*Implication:* the rule "state the cause only when the program knows it" matches F1.10 and F1.12 exactly.

**P2.2** [Google Chat: write actionable error messages](https://developers.google.cn/chat/design/errors?hl=en) (Google, no date). A fetch agent read the full page, and I rely on its report. A message states what went wrong, how the user can go on toward the goal, and how to get help. It names a `/help` command or a link to documentation. Its examples say if the action happened.

*Implication:* the structure "problem, fix, help" is the common core. The help item is the diagnostic handle.

**P2.3** [Google tech writing: error messages](https://developers.google.cn/tech-writing/error-messages) (Google, no date). A fetch agent read the full page, and I rely on its report. It is a short course page with two questions: "What went wrong?" and "How does the user fix that problem?".

It lists the faults "unactionable", "vague", "imprecise", "inaccurate", "unclear cause" and "unknown next steps". It points to a next unit on general rules, which nobody read. It gives no concrete checklist.

*Implication:* the page names "inaccurate" as a fault. It gives no method to prevent it, so it adds little beyond P2.1 and P2.2.

**P2.4** [Glukhov on dead letter queues](https://glukhov.org/app-architecture/integration-patterns/dead-letter-queues/) (practitioner blog, a weak source). The brief of the console states that a dependency outage can make every valid message fail. I did not read the page beyond that summary.

*Implication:* by synthesis, this matches B10. A logout is an outage of a dependency, and it made every valid review fail. The cause is the dependency, not the message. The signal must say so.

**P2.5** Gap search 1, on classification by string matching. A live search on 2026-11-05 found only practitioner material, and no standard. I read the search summaries only, not the pages. Each source is an issue thread, a review or an audit write-up.

- A [Gitaly merge request](https://gitlab.com/gitlab-org/gitaly/-/merge_requests/742) says that matching error text is an anti-pattern and often fragile.
- A [brittle error detection entry](https://kenrinzero.github.io/ai-code-audit-taxonomy/taxonomy/patterns/brittle-error-detection/) says that a reworded message makes the match fail silently.
- A [codec issue](https://github.com/void-layer/codec/issues/43) proposes an error-code map, so that consumers branch on the code and not on `.message`.

The sources agree: give an error a typed identity and branch on it.

*Implication:* the fix of B8 (the class comes from a field that the writer sets) is the mainstream practice. The failure of B8 is the known anti-pattern, in the form of a word list over free text.

**P2.6** Gap search 2, on runbook links. The search found vendor and blog guides only. I read the summaries only. Both sources are weak, and the search did not find the Google SRE book chapter.

- A [OneUptime guide](https://oneuptime.com/blog/post/2026-01-30-alert-runbook-links/markdown) puts a `runbook_url` on each alert, so that an alert answers "What do I do next?".
- A [GitHub template of SRE runbooks](https://github.com/Inblade/sre-runbooks) deletes or demotes a page that has no runbook link and no clear action. It checks runbook structure in CI and does not fetch the URL.

*Implication:* a handle that points to a document is a common idea. It has no strong source here. We use it as a small element (the evidence handle), not as a rule that needs a runbook for each code.

## 3. Adopt or reject

| Practice | Decision | Reason |
|---|---|---|
| Name the operation, the item, the cause, the impact and a safe next step (P2.1, P2.2) | Adopt | The six elements cover the 12 findings of F1.15, group 1. |
| State a cause only when the program knows it (P2.1) | Adopt | F1.10, F1.12: a guessed cause sent the operator to the wrong place. |
| Keep a diagnostic handle on each signal (P2.1, P2.2) | Adopt | An agent that debugs needs the file or the command, not a guess. |
| Branch on a typed code, not on message text (P2.5) | Adopt | F1.10. The mainstream practice is the fix. |
| A runbook link for each code (P2.6) | Reject | The sources are weak, and the catalog `signal-codes.md` already names the meaning of each code. |
| A new tool for message linting | Reject | The test that reads the whole text with equality gives the same proof with no new tool. |
| Treat a dependency outage as the cause of all failures (P2.4) | Adopt in part | A logout gets its own class. A general outage detector is outside this set. |

## 4. Refinements to adopt

**R1** A signal has six parts: operation, item, cause or "unknown", evidence handle, impact, next step. Source: P2.1, P2.2, F1.15.

**R2** The writer of a failure sets the class from a host fact. The class is never a function of text that an agent, a test or a user can fill. Source: F1.10, P2.5.

**R3** Each fact has one reader: one count of ships, one parser of `go test -json`, one rule for which tree a check reads. Source: F1.2, F1.7, F1.17.

**R4** The design of a fix lists each entry that reads the fact, and the wiring row has one test for each entry. Source: F1.6, F1.17.

**R5** The test of a signal reads the whole text with equality. A check on fragments cannot catch a missing cause. Source: F1.16.

**R6** A logout has its own cause. When evidence names neither a logout nor a quota wall, the cause is "unknown" and not quota. Source: F1.12, P2.1, P2.4.

**R7** A task with more than three blocks gets a redesign at the top tier, from the architecture side. The design check reads it before the build. Source: F1.14, F1.18.

**Note on R7.** I read only search summaries of these sources on 2026-11-05. They support bounded retries, a breaker and a hand-off to a person (P3.1, P3.2). The summaries agree on no number. One practitioner post, which is not a standard, argues that a limit stops cost and that the work itself must change (P3.3). So the limit of three comes from the operator, not from outside practice.

**R8** A step that the console does by hand twice becomes a tested verb. Source: F1.20.

## Gaps in this research

- I did not run the code or the tests. The hand-run evidence comes from the console files of 2026-11-04 and 05.
- I did not repeat the audit of landed commits (F1.8), the count of wave 154 (F1.2) or the count of "times blocked" (F1.14).
- I did not trace the code path that turned the logout into `quota-pause` (F1.12). The exact text of a logged-out claude pane is not captured in the files that I read.
- I did not read cycle 2153, `consolepr.go` beyond its signature, or the full text of each review report. For the reports I read the part that follows the heading "Findings", up to about 2,200 characters for each, and the title of each finding.
- Sources P2.4, P2.5 and P2.6 are weak or summary-only, as marked.

## Sources

**Local files**
- `docs/operations/operating-policy.md`, `docs/architecture/convergence-policy.md`, `docs/architecture/signal-center-design.md`
- `R/.evolve/runs/cycle-2157/` and `R/.evolve/runs/cycle-2161/` (retrospective reports, `audit-fail-reason.json`, `failure-digest.json`)
- `R/.evolve/logs/20261104T141300Z/loop.log`, `R/.evolve/ledger.jsonl`, `R/.evolve/waves/wave-155.json`
- `S/round9/b4.json`, `S/round9/b5.json`, `S/rq3/<b2|b3|b4|b5|b6|b8|b9>-architecture-reviewer.json`

**Outside**
- [Write Actionable Error Messages](https://www.joshka.net/practice/patterns/write-actionable-error-messages/)
- [Google Chat: write actionable error messages](https://developers.google.cn/chat/design/errors?hl=en)
- [Google tech writing: error messages](https://developers.google.cn/tech-writing/error-messages)
- [Glukhov on dead letter queues](https://glukhov.org/app-architecture/integration-patterns/dead-letter-queues/) (summary only)
- [Gitaly: do not match text in gRPC errors](https://gitlab.com/gitlab-org/gitaly/-/merge_requests/742) (summary only)
- [Brittle error detection](https://kenrinzero.github.io/ai-code-audit-taxonomy/taxonomy/patterns/brittle-error-detection/) (summary only)
- [codec issue 43](https://github.com/void-layer/codec/issues/43) (summary only)
- [Alert runbook links](https://oneuptime.com/blog/post/2026-01-30-alert-runbook-links/markdown) (summary only; vendor blog)
- [SRE runbooks template](https://github.com/Inblade/sre-runbooks) (summary only)
- P3.1 [Error handling, retries and escalation paths](https://open-exam-prep.com/study-guides/github-ai-developer/tool-use-mcp/error-retries-escalation) (search summary only; study guide)
- P3.2 [Preventing the agent death spiral](https://dev.to/debashish_ghosal/preventing-the-agent-death-spiral-why-your-ai-agents-need-a-circuit-breaker-9h7) (search summary only; blog)
- P3.3 [AI agent failure loops](https://dev.to/gshevchenko/ai-agent-failure-loops-when-persistence-becomes-a-quality-bug-4mmg) (search summary only; blog)
