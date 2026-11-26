# Persona writing standard

- **Status:** proposed, 2026-11-25. It takes force when the plan step P1 lands.
- **Applies to:** each file in `agents/`, and each phase text that the loop sends to a model.
- **Evidence:** [research note](../research/persona-prompt-standards-2026-11.md). Ids such as P2.12 name its findings.
- **Plan:** [persona-diet-2026-11.md](../plans/persona-diet-2026-11.md).
- **Basis:** [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md).

## Purpose

A persona gives one phase its role, its inputs, its steps and its output. Both families read it: Claude and agy. Current vendor advice is the same for both: plain wording, a short text and a checkable output (P2.1, P2.2, P2.9). This standard turns that advice into rules that a script can check.

## Terms used throughout

- **Persona:** a file in `agents/`.
- **Reference file:** a file next to a persona, named `<persona>-reference.md`, that holds long detail.
- **Closing checklist:** the last section of a persona. It repeats the deliverable path, the format and the hard limits.
- **Guard:** code that enforces a rule: a hook, a gate or a test.

## The rules

### 1. Shape

A persona has these parts, in this order. Each part has a heading.

| # | Part | Content |
|---|---|---|
| 1 | Role | One line. It says what the phase does. It has no character words. |
| 2 | Goal | The outcome in one or two sentences, with the reason. |
| 3 | Inputs | The files and values that the phase reads. List long inputs first. |
| 4 | Steps | Numbered imperative steps. Each step has one action. |
| 5 | Output | The path of the deliverable, the section names and the fields of each finding. |
| 6 | Done | The checks that must pass before the phase hands off. |
| 7 | Closing checklist | The deliverable path, the format and the three to five hard limits. |

**Why:** Gemini drops a limit that appears early in a long prompt (P2.12). Claude answers better when the task comes last (P2.13). The role line is functional, because a strong persona can override a rule on Gemini (P2.7). The task is an imperative, because Gemini reads a descriptive text as a question (P2.27).

**Headless line.** Each persona says once: "No one answers questions. Record the assumption under Assumptions and continue." The reason is that every phase runs unattended (P2.24).

### 2. Wording

1. Write plain imperative lines. Avoid words in capitals for emphasis (`MUST`, `NEVER`, `CRITICAL`, `IMPORTANT`). Keep at most three, only for true invariants, each with its reason (research K1). Use `must` in the lower case, as the STE rule asks.
2. Give each limit its scope and its reason in one line. Example: "Write only inside the worktree, because the ship gate reads that tree."
3. Say what to do instead of what to avoid. Example: "Report only the numbers that a command printed, and derive totals from them" replaces "never invent a number" (P2.5, P2.6). A negative limit with its reason goes in the closing checklist only. Gemini drops a negative limit that appears early (K2).
4. A hard guarantee lives in a guard. The persona names the guard in one line and does not shout. Example: "The ship gate denies a bare `git commit`." (P2.3).
5. Use one term for one thing, and one set of tag names, in all personas (P2.11).
6. Use Markdown headings for the skeleton. Use XML tags only around injected data and examples: `<example>`, and a tag for untrusted data (P2.11, P2.26). The name `<untrusted_data>` is proposed. The lint component (plan C4) fixes the one allowed list of tag names.
7. Give one or two short examples of the output format. Mark them "structure only; the values are placeholders" (K3).
8. Do not add a rule for a model that you did not measure. One-family advice goes to the model notes (rule 8).

**Why:** emphasis makes Claude over-apply a rule (P2.1). Fable 5.2 reads "never" as absolute (P2.4). Gemini can drop a negative limit (P2.12).

### 3. No reasoning-extraction triggers

A request for reasoning in the output can end the model call (P2.21). A persona must not ask for any of these:

- a section for thinking, reasoning or a scratchpad;
- a JSON field named `reasoning`, `thinking`, `trace` or `thoughts`;
- a running log of the thoughts of the model.

Ask for these instead:

- the evidence: a command and its output, or `path:line`;
- the root cause;
- a rationale of one or two sentences;
- a summary of the actions taken.

**Two sites hold such a request today:**

| Site | What it asks | Fix |
|---|---|---|
| `agents/evolve-debugger.md:62` | A JSON field `reasoning` in the decision file, next to `root_cause` | A code change: rename `reasoning` to `rationale` (one or two sentences) in the contract and in the parser (`go/internal/phases/debugger/debugger.go:40`), with a test first. Keep read support for the old name for one release. Then change the persona. |
| `skills/loop/phase3-build.md:222` | A field `reasoning` ("WHY it failed — root cause") in the failed-approach record, next to `error` | Rename the field to `root_cause`, and check each reader of the record. |

The debugger parser reads `Reasoning` (`json:"reasoning"`) at line 40, and line 39 already holds `RootCause` (`json:"root_cause"`). So the debugger fix is a rename to a new name, not a merge into `root_cause`. A persona-only edit breaks the contract. The fix is one component of the plan.

### 4. Verification

1. Name the exact check command. Example: `go test -count=1 ./internal/prompts/...` from `go/`.
2. Require the output of the command as evidence in the report: "N/N PASS" and the last lines.
3. Do not write a generic line such as "double-check your work" or "verify before you finish". Opus 5 over-verifies on such a line (P2.17, E1). Opus 5.5 is expected to do the same, but this is unverified (plan open question 2).
4. Judging belongs to a separate phase with a fresh context. The builder does not grade its own work (P2.18).
5. A report states a result only if a command of this session printed it (P2.20).

### 5. Reviewer personas

- Report each finding with a severity and a confidence. Do not filter in the first pass.
- Say "skip style" when style is not the job of the reviewer.
- A later step filters by a concrete threshold. The persona does not use "important" as a threshold.
- A found defect counts, also when it looks small.

**Why:** a prompt that says "report only high-severity issues" makes the model report less (P2.19).

### 6. Thinking and depth

Depth comes from the effort setting of the route (P2.16). A persona has no "think step by step", "think carefully", "be thorough" or "do not think" line. A phase that runs at `xhigh` gives a stop criterion: "stop when the root cause has a failing test".

Put no token count, no budget count-down and no time stamp in a persona (P2.9).

### 7. Length and structure

| Item | Target |
|---|---|
| A persona | About 200 lines |
| A reference file | One link deep from the persona. A table of contents when it has more than 100 lines. |
| A rule that a guard enforces | One line that names the guard |
| A history note, a cycle number or a date | None. The incident record holds them. |

- Move schemas, long rubrics and long examples to the reference file.
- Do not describe what the model can find by reading the code (P2.10).
- Keep one set of tag names. The persona ids that the loop reads (for example `P-NEW-xx`) stay only when a script reads them.

### 8. What goes to the model notes

A rule for one family goes to `skills/model-notes-claude/COMPACT.md` or `skills/model-notes-agy/COMPACT.md`. It does not go to the persona. Examples:

- a rule that helps one model and hurts another (E1 to E15 in the research note);
- the plan of agy for rule files, flags and artifacts;
- a fact that holds for one CLI version.

Keep each notes file short. Add a line only when the fact is new for that family.

### 9. How the standard is checked

A review reads rules 1 to 8. The checks below can run as a lint. They are candidates. This document does not build them.

| Candidate rule | Check | Class |
|---|---|---|
| L1 | Count of words in capitals for emphasis is at most 3 per persona (rule 2.1) | Script |
| L2 | No JSON field named `reasoning`, `thinking`, `trace` or `thoughts`, and no heading for a scratchpad | Script |
| L3 | The last heading is the closing checklist, and it holds the deliverable path | Script |
| L4 | The line count is at most the budget of the persona | Script |
| L5 | No line holds "double-check", "think step by step" or "be thorough" | Script |
| L6 | Each reference link resolves, and no link is more than one level deep | Script |
| L7 | One tag vocabulary: each XML tag is in an allowed list | Script |
| L8 | Each limit gives a reason | Review only |
| L9 | A rule says what to do, not only what to avoid | Review only |

The scripts L1 to L7 are the content of the lint component in the plan. L8 and L9 stay with the reviewers of the [docs-review](../../skills/docs-review/SKILL.md) skill.

## Closing checklist of a persona

The example below shows the shape. The values are placeholders (structure only).

<example>
## Before you hand off

- The report is at `<workspace>/<name>-report.md` with the sections Changes, Tests and Verdict.
- The report holds the output of `go test -count=1 ./<package>/...` as N/N PASS.
- You changed only the files that the input names, because the pipeline restores each other file.
- You added no code comment, because the commit gate rejects it.
- You made no commit and no push, because the console ships.
</example>

## What this standard does not do

- It does not set the model or the effort of a phase. The route holds both: `evolve cli-routing explain <agent>`.
- It does not replace a guard. A rule that must hold every time stays in a guard.
- It does not prove quality. Each change to a persona needs the before and after measures of ADR-0133 decision 5.
