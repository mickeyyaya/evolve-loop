---
name: docs-review
description: Use when a change touches only documents (every changed path ends in .md) — research dossiers, plans, ADRs, architecture specs, conventions, guides, skills and READMEs. It runs the deterministic checks first, then four review lenses (type and fit, claims against the code, the reasoning chain, the consistency of a design set), and reports findings with evidence. It is the "docs review" capability of the commit gate, in place of a code simplifier and a code reviewer.
---

# docs-review — a review for a change of documents

> A code reviewer reads prose for bugs that prose does not have. It misses the defects that prose does have. These are a false claim about the code, a decision with no reason, and two files of one design that disagree. This skill reviews a document as a document.
>
> Sources: [docs/research/docs-review-skill-2026-10.md](../../docs/research/docs-review-skill-2026-10.md). The rules that it applies: [skills/docs-write/SKILL.md](../docs-write/SKILL.md), [docs/conventions/ste100-writing.md](../../docs/conventions/ste100-writing.md) and [docs/conventions/markdown-structure.md](../../docs/conventions/markdown-structure.md).

## Scope

- The unit of review is the change: `git diff --cached`, or the files that the caller names. Read a file outside the change only when a changed line cites it.
- Use this skill when every changed path ends in `.md`. A change that has any other file needs the code review chain. This skill does not review code.
- The caller declares what already ran. Do not run a declared check again.
- Do not edit the files. Report the findings. The author applies them.

## Step 1. The deterministic checks (stop on a failure)

Run these before any lens. A tool finds these defects faster than a reader.

1. `evolve docs ste-lint --strict --changed refs/remotes/origin/main --added`. The result must be 0 findings.
2. The link check: each relative `[text](path#anchor)` resolves to a file, and each anchor to a heading of it.
3. The citation check: each cited `path:line` exists, and `grep` finds each named symbol in the tree.

If a check fails, report it as a HIGH finding and stop. The author fixes it first.

## Step 2. Choose the depth

| Change | Lenses |
|---|---|
| A typo, a link or one sentence | 1 and 2 on the changed lines |
| A section of a spec, a guide or a skill | 1, 2 and 4 |
| A research dossier, a plan or an ADR | all four |
| A design set (research, plan, ADR, spec) | all four, over the whole set |

## Step 3. The four lenses

Each lens has one job. Do not copy-edit in a lens: style is the job of the lint.

### Lens 1. Type and fit

- The document has one type and one home (the table of `docs-write`). A how-to does not hide in a design, and a decision does not hide in a research dossier.
- The document answers the question of its type for its reader. A new reader gets the context, the terms and one worked example before the rules.
- The header links the other files of its design set. The index (`docs/research-index.md`) lists a new dossier, plan or ADR.

### Lens 2. Claims against the code

Each claim about this repository must be true now. Open the source of each claim that a changed line makes.

- A `path:line`: the line holds what the text says.
- A named symbol, command, flag, config key or signal code: it exists with that name. A new name that the document proposes is marked as proposed.
- A number: it has a date and a method, or it says "estimate".
- A claim about behavior: read the function. Do not trust a comment or another document.
- An outside claim: it has a link and a date, or "acc.".

A claim that you did not check is not a finding. Say "not checked" in the report.

### Lens 3. The reasoning chain

For each finding and each decision, ask the questions of an ADR review:

1. Does the problem merit the decision?
2. Are the options real, and is a valid option missing? A plan scores at least three approaches.
3. Does each decision cite its evidence, and do the cited ids exist?
4. Does the chosen option solve the problem? Is its rationale sound?
5. Are the pros and cons of each option fair? Is each rejected option given a reason?
6. Is the decision traceable to a component and to a test? Does it say what changes the decision?
7. Are the limits stated: what the design does not do, and what a test cannot prove?

Look for a hole in the design. Examples: a state with no owner, a failure with no path, a measure that is true by construction. Also a rule that contradicts a standing rule.

### Lens 4. The consistency of the set

- The ids (`F`, `M`, `D`, `I`, `Q`) match across the files, and each cited id exists.
- The names of keys, codes, branches and stages are the same in each file.
- A number is the same in each file.
- Each document that the change amends has an "Amended by" note, and the amended text no longer contradicts the new one.
- One fact has one home. When two files repeat one table, one file links to the other.

## Step 4. The report

Report the findings first, most severe first. Each finding has this form:

```
[HIGH|MEDIUM|LOW] <path:line> — <the defect, in one sentence>
  Evidence: <the command, the path:line or the link that proves it>
  Fix: <the change, in one sentence>
```

- HIGH: a false claim, a broken link or citation, a design hole, or a contradiction between files.
- MEDIUM: a missing reason, a missing option, an unclear owner, or a repeated fact.
- LOW: a fit or structure point that does not change a decision.

End with the counts and the checks that ran: `docs-review: 2 HIGH, 1 MEDIUM, 0 LOW; ste-lint 0, links 0 broken, citations 14/14`.

Then the commit gate takes the capability: `evolve commit-gate run --reviewers docs-review`. Attest it only when this review ran.

## Anti-patterns

| Anti-pattern | Correction |
|---|---|
| Pass through: "looks good" with no check | Run the checks and each lens of the depth. Report the counts. |
| Copy edit: only wording and style | The lint owns the style. Find the false claims and the holes. |
| A finding from memory | Open the source. A claim that you did not check is "not checked". |
| A review of code in a docs change | This skill does not review code. A change with code needs the code review chain. |
| Rewriting the document | Report the fix. The author writes it. |
