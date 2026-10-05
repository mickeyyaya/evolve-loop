# ADR-0111 — Code carries no comments; what they carried moves to tests and docs

- **Status:** Accepted (2026-09-30). The target and the tooling landed in PR #746; round 12 in #749. The commit-gate refusal and the loop-wide rule are in PR #753, and the history archive is in its own PR (both landing at the wave-54 boundary).
- **Supersedes** the earlier target of "machine-read comments plus exported-identifier docs". Exported docs are no longer required.
- **Followed by:** [ADR-0115](0115-commit-gate-keeps-history-and-records-waivers.md), which makes the commit gate enforce decision 7 for every console change.
- **Related:** [the code-comments convention](../../conventions/code-comments.md) (the rule's one home); [the comment reduction plan](../../plans/comment-reduction-2026-09.md); [round 12 findings](../../reports/comment-round-12-findings-2026-09-30.md); AGENTS.md invariant 10.

## Context

Comments in this codebase had drifted into three kinds of content. First, narration of what the code does, which goes stale whenever the code changes and which the code itself could say through names and structure. Second, history: cycle numbers, incident retellings, dates and F-ids, which belong in the incident record, the ADR and the CHANGELOG. Third, rules the code depends on ("classification must run before Ship commits the binary") that no test pinned, so nothing stopped a change from breaking them.

A reduction workstream had cut the comment count round by round since 2026-09-26, but nothing stopped new comments arriving. Loop lanes and console sessions kept adding them, and a comment-only addition could even ride the commit gate's review waiver for proven comment-only changes.

The operator set three rules on 2026-09-30:

1. "Continue to remove comments until there are zero comments in the code."
2. "Make sure when we removed comments, code is lean enough to explain itself without comments."
3. "Add system level policy for evo loop skill and project to set the rule that it is forbidden to write comments; code should explain itself." And: "The history track must be recorded and stored."

## Decision

1. **The target is zero comments** beyond what a tool reads: directives such as `//go:build` and `//nolint`, markers a gate reads, generated-file headers, and one short package doc per package.
2. **What a comment said moves to where it belongs:**
   - what the code does, into the code itself (names, extracted functions, named constants);
   - a rule the code depends on, into a test named for the rule, and then the comment goes;
   - design knowledge, into the package's design notes under `docs/architecture/packages/`;
   - history, into the comment history archive under `docs/history/code-comments/`.
3. **Every removal round has two commits.** The first removes comments and is proven comment-only by `commentaudit verify`. The second, a reviewed refactor, makes the code say what the deleted comments said.
4. **New comments are refused at commit.** The commit gate refuses a commit that adds a comment, using the same rule the build floor uses (`commentaudit.AddedAcrossDiff`), before any review waiver applies. A moved comment is not an added one.
5. **The rule is stated in every place an agent reads.** AGENTS.md invariant 10, the loop skill, the build, tdd, refactor and minimalism skills, and the personas that write code. The convention states the rule once, and every other document points to it.
6. **Loop lanes follow in stages.** The build floor's `comment_floor` counts added comments in `shadow`. It moves to `enforce` only after the TDD phase has its own comment floor, because the build floor also scans the predicate files TDD writes and the builder may not edit (inbox `tdd-comment-floor-then-enforce`, then `comment-floor-enforce`).
7. **History is recorded before it is removed.** `commentaudit history` appends each removed history-carrying comment group, as it was, with its file, line and the code below it, to a per-package page under `docs/history/code-comments/`. Every round runs it before its comment-only commit.

## Alternatives considered

| Alternative | Why it was not chosen |
|---|---|
| Keep exported-identifier docs (godoc style) | Most restated the identifier's name. The operator set the target at zero, and names and package design notes carry the intent. |
| Allow "why" comments | A "why" that matters is a rule, and a rule belongs in a test that fails when it breaks. A comment cannot fail. |
| Enforce at review only | Reviews are advisory, and a comment-only addition could ride the proven-comment-only review waiver. The gate closes that hole. |
| Enforce `comment_floor` in the loop immediately | Would stall lanes on comments in TDD-owned predicate files the builder may not edit (found by the architecture review). |
| Delete history comments without recording them | Loses the only link from a line of code to the incident that shaped it. The archive keeps it, outside the code. |

## Consequences

- A commit that adds a comment is refused with the offending lines named. Reworded comments count as added, which is consistent with the zero target.
- Rules that lived only in comments become tests. Round 12 alone surfaced nine.
- Tests that hash raw source or read comment text block the workstream. They move to code shapes (ASTs) or are retired.
- Closely reading every comment doubles as a code review. Round 12 found 17 production defects, two of them security holes.
- The rule reaches loop lanes fully only after the TDD comment floor lands. Until then, lanes' added comments are counted, not refused.

## Evidence

- Rounds 1–12 merged, round 12 as PR #749. The [plan's progress table](../../plans/comment-reduction-2026-09.md) lists every batch. Each round was proven comment-only by `commentaudit verify`, and from round 12 on each was followed by a reviewed self-explanation commit.
- The commit-gate refusal (#753): two architecture-review rounds (Approve), Go review PASS. Mutation sweeps killed 13 of 13 mutants, plus a fault-text mutant.
- The history archive: three architecture-review rounds. Its backfill records the removals from 3ce14dd0 (2026-09-26) to main after round 12.
