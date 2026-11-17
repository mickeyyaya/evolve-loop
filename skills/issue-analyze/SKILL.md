---
name: issue-analyze
description: Use when you must find the root cause of an issue, alert or mistake before a fix — a failed test, a gate refusal, a WARN or INCIDENT signal, a leak, a wrong claim. It traces the causal chain from the evidence, names each contributing cause and the control that did not catch the issue (the escape), and gives the instance fix, the class fix and the red-first regression test. issue-record calls it; it also works alone.
---

# issue-analyze — the causal chain, the escape and the class fix

> TLDR: no fix before the cause. Trace back from the symptom to the first cause, one link at a time, with evidence for each link. Then ask which control let the issue through. The instance fix removes the cause; the class fix repairs the control.
>
> Sources: [docs/research/issue-record-skills-2026-11.md](../../docs/research/issue-record-skills-2026-11.md) (P2.6 and P2.7). The principle "root cause or nothing" is in [fable](../fable/SKILL.md) §2. When the superpowers plugin is installed, its `systematic-debugging` skill and its `root-cause-tracing.md` give the debug loop; this skill adds the escape and the output shape.

## Inputs

- The evidence of [issue-record](../issue-record/SKILL.md) step 1: the commands, the exit codes, the signal lines, the gate text and the run paths.
- Read access to the code. Track code with `evolve code` when the index is fresh.

## The procedure

| Step | Action | Exit criterion |
|---|---|---|
| 1. State the symptom | Write what failed, where and when, in one sentence with its evidence. | One sentence, one source. |
| 2. Reproduce | Run the smallest command that shows the symptom. Record the command and its output. | The symptom recurs, or you write why it cannot. |
| 3. Trace back | From the symptom, ask "what made this happen?" Read the code at each link and cite `path:line`. Stop at a cause that is a design choice or an input from outside. | A chain of numbered links, each with a source. |
| 4. Find the other causes | Ask which other conditions were necessary. An issue often needs two or more causes together. | Each contributing cause is in the list. |
| 5. Find the escape | Ask which control did not catch the issue: a test, a guard, a gate, CI, a review. Then ask why. | The escape names a control and the reason. |
| 6. Choose the fixes | The instance fix removes the first cause. The class fix repairs the escape. | Two fixes, or one with the reason. |
| 7. Plan the test | Name the regression test through the real entry point. Prove it red without the fix. | A test name and its red output. |

Five Whys is a fast first probe for step 3. Do not stop at its fifth answer. An issue that needs two causes together has two chains, and Five Whys follows only one.

## The output

Give this block to issue-record. docs-write puts it into the §Root cause and §What changed sections of the record.

```markdown
**Symptom.** <one sentence, with its source>
**Chain.**
1. <the symptom> ← 2. <what caused it, path:line> ← 3. ... ← n. <the first cause>
**Contributing causes.** <each other necessary condition, with its source>
**Escape.** <the control that did not catch it, and why>
**Instance fix.** <the change, and its PR or item>
**Class fix.** <the change to the control, and its PR or item>
**Regression test.** <path::TestName>, red without the fix: <the output line>
```

## Worked example: tests leaked ledgers under go/ (2026-11-16)

- **Symptom.** The composed-tree `make test` of cycle 2419 failed with four `LEAK` paths under `go/internal/phases/triage/.evolve` (signal `ORCHESTRATOR_COMPOSED_GATE_DECLINED`, 00:50:27Z).
- **Chain.** 1. `evolve treewalk check` found new files under `go/` ← 2. a test wrote `.evolve/ledger.jsonl` in its package directory ← 3. `inboxmover` built the ledger path from `ProjectRoot`, which the test left empty, so the path was relative ← 4. `resolveOpts` filled `InboxDir` from `ProjectRoot`, but never `ProjectRoot` from `InboxDir`.
- **Contributing causes.** Three test files in three packages named only the inbox. Each lane composed the whole tree, so each lane met the leak.
- **Escape.** CI runs `make test-integration`, which has no treewalk guard. Main stayed green, so no person saw the leak until the lanes failed.
- **Instance fix.** PR #1490: `planeRootOf` derives the root from `<root>/.evolve/inbox`.
- **Class fix.** The inbox item `ci-test-integration-treewalk-parity` adds the treewalk guard to `make test-integration`.
- **Regression test.** `go/internal/inboxmover/resolve_root_test.go::TestClaimPending_AnInboxOnlyCallerWritesTheClaimRowToThePlaneLedger`.

## Red flags

| Thought | Correction |
|---|---|
| "I see the bug, so I skip the trace." | A fix without the chain fixes a symptom. Trace first. |
| "The person or the agent made an error." | Name the design that let the error happen. A record is blameless. |
| "There is one root cause." | Look for the other conditions that were necessary. |
| "The fix works, so the escape does not matter." | The escape lets the next issue of the class through. |
| "No root cause exists; it is the environment." | Write what you checked. Most such cases are an incomplete trace. |
