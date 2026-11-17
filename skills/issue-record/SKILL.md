---
name: issue-record
description: Use when an issue, alert or mistake is detected — a failed test or CI red, a gate refusal, a WARN or INCIDENT signal, a self-heal incident, a wrong or retracted claim, a reverted change, or an operator correction. It captures the evidence and the key logs first, hands the cause to issue-analyze and the text to docs-write, and adds the record to docs/incidents/ and its regression index.
---

# issue-record — one record for each issue, alert or mistake

> TLDR: capture the evidence before you change anything. Then find the cause with [issue-analyze](../issue-analyze/SKILL.md), fix it with a red-first test, write the record with [docs-write](../docs-write/SKILL.md) (§Incident template), and add the index row.
>
> The operator asked on 2026-11-16: "I intend to create a detailed document to record all the mistakes and issues encountered during development, including the incident reports, key logs, results, and the corresponding fixes, as everything needs to be documented for future learning." The design and its reasons: [docs/plans/issue-record-skill-2026-11.md](../../docs/plans/issue-record-skill-2026-11.md). The research: [docs/research/issue-record-skills-2026-11.md](../../docs/research/issue-record-skills-2026-11.md).

## When to use

Start this skill when one of these events occurs. The console and each loop agent can start it.

| Trigger | Where you see it |
|---|---|
| A test fails, or CI on a branch or on main is red | `go test` output; `evolve ci classify` |
| A gate refuses: a commit gate, a ship gate, a composed-tree gate, a ratchet | the refusal text and its code |
| A WARN or INCIDENT signal | `evolve events watch --channel errors,selfheal.incident`; `.evolve/runs/cycle-<N>/signals.ndjson` |
| A self-heal incident opens or reopens | `evolve selfheal incidents`; `HEALING_INCIDENT_OPENED` |
| A claim was wrong, or an answer was retracted | your own output, or a reviewer finding |
| A change was reverted | `git log`; a revert PR |
| The operator corrects you | the message of the operator |

Do not use it for a cycle FAIL that the loop already handles. [retro](../retro/SKILL.md) and the retrospective agent own the lessons of a cycle. Link their report from the record when the issue goes past one cycle.

## Choose the level

A mistake is **small** only when all five tests are true:

1. One action fixed it, for example a correct command or a rebuilt binary.
2. No code, test or document changed.
3. It cost less than 15 minutes.
4. It did not reach main, a lane, CI or the operator.
5. It is the first row of its cause in the session log.

A small mistake gets one row in `docs/incidents/<yyyy-mm-dd>-session-log.md`. Each other issue gets its own record at `docs/incidents/<yyyy-mm-dd>-<slug>.md`. A second row of one cause promotes it to a record. The reason for each limit is in the plan, decision D4.

The session log has one table:

```markdown
# <yyyy-mm-dd>: session log of small mistakes

| Time (UTC) | Mistake | Evidence | Fix | Cause |
|---|---|---|---|---|
```

## The procedure

| Step | Action | Exit criterion |
|---|---|---|
| 1. Capture | Before a fix, save the evidence. Keep the failed command, its exit code, the signal lines and the gate text. Keep the run paths, the commits and the PR numbers. | Each fact has a path, a command or a time. |
| 2. Trim and redact | Keep 20 lines or fewer for each log block. Remove each token, key, password, account name, email address and person name. | No secret in the text. |
| 3. Analyze | Run [issue-analyze](../issue-analyze/SKILL.md) on the evidence. | A causal chain, the escape and the class fix. |
| 4. Fix | Write the regression test first and prove it red without the fix. Then fix the root cause through its owner: a PR, a lane item or an inbox item. | The test is red without the fix and green with it. |
| 5. Write | Write the record with [docs-write](../docs-write/SKILL.md) and its §Incident template. Put the key logs in fenced blocks under the timeline. | The five sections exist. |
| 6. Index | Add one row for each failure mode to [REGRESSION-COVERAGE-INDEX.md](../../docs/incidents/REGRESSION-COVERAGE-INDEX.md). Change the summary counts in the same edit. | The row names the pinning test, or ❌ and the item that tracks it. |
| 7. Link | Name the self-heal incident id, the signal code, the PR and the inbox item in the record. | Each id is in the record. |
| 8. Check | Run `evolve docs ste-lint --strict --changed refs/remotes/origin/main --added` and resolve each relative link. Then run the [docs-review](../docs-review/SKILL.md) pass. | 0 findings. |

The record holds these parts. The template of docs-write gives the section order.

- **What happened:** the impact and the timeline, with the time of each event in UTC.
- **Key logs:** the signal lines and the gate text, verbatim but trimmed.
- **Results:** the measured effect: cycles lost, minutes, red runs, ships blocked.
- **Root cause:** the causal chain from issue-analyze.
- **Fix:** each change with its PR or commit, and its regression test.
- **Lesson:** each lesson with the rule or the guard that it became.

## The alert link

A WARN or INCIDENT event on the lossless channels `errors` and `selfheal.incident` starts this skill. The console watch shows the event:

```bash
evolve events watch --channel errors,selfheal.incident --filter 'severity>=WARN' --since new
```

Start the skill for each new code and cycle pair. Do not start it again for a repeat of one code in one cycle. Add the new evidence to the open record instead. The later verb `evolve issue stub` (plan component K4) writes the header and the timeline from the signal. Until it exists, step 1 is by hand.

## Worked example: tests leaked ledgers under go/ (2026-11-16)

The record: [2026-11-16-tests-leak-evolve-ledgers-under-go.md](../../docs/incidents/2026-11-16-tests-leak-evolve-ledgers-under-go.md).

1. **Trigger.** Cycle 2419 emitted `ORCHESTRATOR_COMPOSED_GATE_DECLINED` (WARN) at 00:50:27Z. The ship and re-audit loop of the cycle ran for about 3 hours.
2. **Capture.** The signal field `tail_test` held the key log:

   ```text
   LEAK internal/phases/triage/.evolve
   LEAK internal/phases/triage/.evolve/ledger.jsonl
   evolve treewalk check: 4 path(s) appeared under . while the tests ran
   make: *** [test] Error 1
   ```

3. **Level.** The issue reached the lanes and cost hours, so it gets its own record.
4. **Analyze.** `inboxmover` resolved the ledger from an empty `ProjectRoot`. A test that named only the inbox wrote `.evolve/ledger.jsonl` in its package directory. The escape: CI runs `make test-integration`, which has no treewalk guard. Main stayed green.
5. **Fix.** PR #1490 derives the root from `<root>/.evolve/inbox` (`planeRootOf`). The red-first test is `TestClaimPending_AnInboxOnlyCallerWritesTheClaimRowToThePlaneLedger`. The class fix is the inbox item `ci-test-integration-treewalk-parity`.
6. **Index.** Two rows: the leak (✅) and the CI escape (❌ until the item lands).

## Red flags

| Thought | Correction |
|---|---|
| "I will fix it first and write it up later." | The evidence goes away with the fix. Capture first. |
| "It is too small to record." | Apply the five tests. A small mistake still gets a row. |
| "The signal says WARN, so it can wait." | The issue of 2026-11-16 was a WARN that cost hours. |
| "The log is long, so I paste all of it." | Trim to the lines that prove the claim, and redact. |
| "The cause is the test that leaked." | Also ask which gate did not catch it. That answer is the class fix. |
| "I will add the index row later." | A fix without its row is incomplete. |
