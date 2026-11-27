---
name: research
description: Use when a request needs a deep, multi-source study of current practice before a decision — open source projects, papers, vendor docs, or an online survey that compares our design with others. It splits the question into research angles, runs a live web research for each angle in parallel, keeps the notes with their links, and then writes the design set through docs-write (research dossier, plan with its execution order, ADR and architecture spec) and one inbox item for each component of the plan.
argument-hint: "<the question, in the words of the person who asked>"
---

# research — from a question to a decided, filed design

> A design decision that rests on remembered practice can be stale or wrong. This skill makes the research live, wide and checkable, and it carries the result to the end: a decision, a design and filed work.
>
> The rules that this skill applies: [skills/docs-write/SKILL.md](../docs-write/SKILL.md) for each document, [skills/docs-review/SKILL.md](../docs-review/SKILL.md) for the review, and [docs/conventions/ste100-writing.md](../../docs/conventions/ste100-writing.md) for the text. This skill does not repeat their templates. It gives the order of the work and the research method.

## When to use it

- The request asks what other projects, papers or vendors do, or asks to "research online", "deep dive" or "survey".
- A plan, an ADR or an architecture spec must decide between designs, and no current research covers the subject.

Do not use it for a single fact. One web search answers a single fact.

## The output

One request makes one design set. The four files link to each other in their headers, as docs-write requires.

| File | Home | What it holds |
|---|---|---|
| Research dossier | `docs/research/<topic>-<yyyy-mm>.md` | The findings (`F`), the outside survey (`P`), the adopt-or-reject table and the refinements (`R`). It does not decide. |
| Plan | `docs/plans/<topic>-<yyyy-mm>.md` | The goals, at least two scored candidate designs, the decisions (`D`), the components, the execution order and the verification. |
| ADR | `docs/architecture/adr/<nnnn>-<decision-as-a-sentence>.md` | The decision, its reasons, its consequences and the rejected options. |
| Architecture spec | `docs/architecture/<topic>.md` | How the part of the system works after the change: the model, the data and the rules. |
| Inbox items | `evolve inbox add --file <item.json>` | One item for each component of the plan, with its acceptance criteria and a link to the plan. |

When the subject needs no new design, stop after the dossier and say why in its last section.

## The procedure

1. **Quote the request.** Write the words of the person who asked, with the date. Each later file cites this quote.
2. **Read what we already have.** Search `docs/research/` and `docs/research-index.md` for the subject. A new dossier builds on an earlier one and links to it. It does not repeat it.
3. **Collect the local evidence.** Read the code and the measurements that the question touches. Cite each one with `path:line`, or with the command and its output and date. Use `evolve code find` and `evolve code callers` to find the code.
4. **Split the question into angles.** An angle is one independent part of the question, for example one family of projects or one discipline. Most requests need three to five angles. Two angles must not cover the same source.
5. **Run the research of each angle in parallel.** Start the web research first, before you write a draft. Give each researcher:
   - the angle, the key questions and the suggested sources;
   - a short paragraph about our system, so that the researcher can judge relevance;
   - the earlier dossiers to build on, with the instruction not to repeat them;
   - the time bound, for example "current state as of the month of the request";
   - one notes file to write.
6. **Keep the notes.** Each notes file has, for each key question, a takeaway, the cited findings, the inferences and the gaps. Each outside fact has its link. A fact from a search summary, not from a fetched page, carries the mark "(search summary)".
7. **Write the dossier.** Use the Research template of docs-write. Turn the notes into numbered findings with their implications. Mark each inference as "synthesis". Write the gaps: what the research looked for and did not find.
8. **Write the plan, the ADR and the spec.** Use the templates of docs-write. Score at least two candidate designs on named criteria. Each decision cites the findings that it comes from. The plan gives the execution order, the reason for the order, and the measure of each goal.
9. **File the work.** Add one inbox item for each component of the plan. An item whose fix surface is a protected file goes to the console. Every other item is lane work.
10. **Index and check.** Add the entry file to `docs/research-index/` and run `evolve docs index generate`. Run `evolve docs ste-lint --strict --changed refs/remotes/origin/main --added`, and check each link and each `path:line`.
11. **Review and ship.** Run the docs-review seat with `evolve agent run docs-review`. Fix each finding, then run the commit gate with `--reviewers run:<run label>`.

## The research method

| Rule | Reason |
|---|---|
| Read the primary source: the repository, the spec, the paper or the official docs. | A blog summary can be old or wrong. |
| Cite a permalink or the commit that you read when you cite source code. | A branch link moves, so a later reader sees different code. |
| Get each number from its source, with its date. Star counts come from the GitHub API on the day of the research. | A remembered number is not evidence. |
| Separate what a source measured from what you infer. | A reader must know which claims a test can disprove. |
| Write what you did not find. | A gap that is not written looks like a covered subject. |
| Prefer two independent sources for a claim that decides a design. | One vendor claim is marketing until a second source agrees. |

## The fan-out

The research of the angles runs in parallel, and each angle writes its own notes file. The console starts each researcher on behalf of the operator. No phase agent starts a researcher, because no LLM in the pipeline starts or directs another (AGENTS.md hard rules). The notes are working files. Write them to `.evolve/evidence/<topic>/` in the worktree, one file for each angle. Git ignores that folder. The dossier and its source list are the record.

## Red flags

| Thought | Correction |
|---|---|
| "I know what these projects do." | Search and read them. The research must be live. |
| "One researcher can cover all the angles." | One context reads fewer sources. Split the question. |
| "The dossier can also decide." | The dossier finds. The plan and the ADR decide. |
| "The plan is done, so the work is done." | Each component needs its inbox item, or nobody builds it. |
| "This number is from a search summary, but it is probably right." | Mark it "(search summary)", or fetch the page. |
