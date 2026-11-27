---
name: research
description: Use when a request needs a deep, multi-source study of current practice before a decision — open source projects, papers, vendor docs, or an online survey that compares our design with others. It runs an LLM-steered block loop. Seats read through diverse lenses, report their claims at each barrier, and stop on a rule that the lead applies by the band (code checks it once the `evolve research` verbs land). It then writes the design set through docs-write (research dossier, plan, ADR and architecture spec) and one inbox item for each component.
argument-hint: "<the question, in the words of the person who asked>"
---

# research — from a question to a decided, filed design

> A design decision that rests on remembered practice can be stale or wrong. This skill makes the research live, wide and checkable. It also states why the research stopped.
>
> The rules that this skill applies: [skills/docs-write/SKILL.md](../docs-write/SKILL.md) for each document, [skills/docs-review/SKILL.md](../docs-review/SKILL.md) for the review, and [docs/conventions/ste100-writing.md](../../docs/conventions/ste100-writing.md) for the text. The spec of the loop: [docs/architecture/research-loop.md](../../docs/architecture/research-loop.md). The decision: [ADR-0171](../../docs/architecture/adr/0171-research-is-an-llm-steered-loop-inside-a-code-envelope-gated-in-depth-blocks-by-evidence-coverage.md).
>
> The operating rules for the lead and the seats: [references/loop.md](references/loop.md). The templates of the ledger and the barrier report: [references/ledger.md](references/ledger.md).

## When to use it

- The request asks what other projects, papers or vendors do, or asks to "research online", "deep dive" or "survey".
- A plan, an ADR or an architecture spec must decide between designs, and no current research covers the subject.
- A writer of an ADR, a spec or a plan marked a missing fact with `NEEDS-EVIDENCE`.

Do not use it for a single fact. One web search answers a single fact.

## The output

One request makes one design set. The four files link to each other in their headers, as docs-write requires.

| File | Home | What it holds |
|---|---|---|
| Research dossier | `docs/research/<topic>-<yyyy-mm>.md` | The findings (`F`), the outside survey (`P`), the adopt-or-reject table, the refinements (`R`), the gaps, and one Decision, Rationale and Alternatives entry for each decision. It does not decide. |
| Plan | `docs/plans/<topic>-<yyyy-mm>.md` | The goals, at least three scored candidate designs, the decisions (`D`), the components, the execution order and the verification. |
| ADR | `docs/architecture/adr/<nnnn>-<decision-as-a-sentence>.md` | The decision, its reasons, its consequences and the rejected options. Each option and driver cites a finding id. |
| Architecture spec | `docs/architecture/<topic>.md` | How the part of the system works after the change. |
| Inbox items | `evolve inbox add --file <item.json>` | One item for each component of the plan, with its acceptance criteria and a link to the plan. |

When the subject needs no new design, stop after the dossier and say why in its last section.

## The procedure

1. **Quote the request.** Write the words of the person who asked, with the date.
2. **Read what we already have.** Search `docs/research/` and `docs/research-index.md`. Build on an earlier dossier and link to it.
3. **Collect the local evidence.** Cite each fact with `path:line`, or with the command, its output and its date.
4. **Plan the loop.** Split the question into decisions and claims. Set the tier. Choose the angles and the lenses. See [references/loop.md](references/loop.md).
5. **Run the blocks.** The console starts the seats. A seat works alone for up to three levels. Each barrier returns a report, and the lead chooses STOP, DEEPEN, WIDEN or STOP-INCOMPLETE.
6. **Report at each barrier.** Give the main session the fixed short form: block, band, decisions, choice and remaining budget.
7. **Write the dossier.** Use the Research template. Turn the ledger into numbered findings. Write the gaps and the stop reason in the method line.
8. **Write the plan, the ADR and the spec.** Score at least three designs. Cite finding ids. Mark each missing fact `NEEDS-EVIDENCE: <claim>`.
9. **Close the markers.** Run a narrow one-block loop for each marker. A document with an open marker does not ship.
10. **File the work.** Add one inbox item for each component of the plan. An item whose fix surface is a protected file goes to the console.
11. **Index and check.** Add the entry file to `docs/research-index/` and run `evolve docs index generate`. Run `evolve docs ste-lint --strict --changed refs/remotes/origin/main --added`. Check each link and each `path:line`.
12. **Review and ship.** Run the docs-review seat with `evolve agent run docs-review`. Fix each finding, then run the commit gate with `--reviewers run:<run label>`.

## The research method

| Rule | Reason |
|---|---|
| Read the primary source: the repository, the spec, the paper or the official docs. | A blog summary can be old or wrong. |
| Cite a permalink or the commit that you read when you cite source code. | A branch link moves, so a later reader sees different code. |
| Get each number from its source, with its date. Star counts come from the GitHub API on the day of the research. | A remembered number is not evidence. |
| Label each claim: measured, maker claim, search summary or synthesis. | A reader must know which claims a test can disprove. |
| Write what you did not find. | A gap that is not written looks like a covered subject. |
| A claim that decides a design needs two independent sources, and one is not the vendor. | One vendor claim is marketing until a second source agrees. |

## The fan-out

The console starts each seat on behalf of the operator. No phase agent starts a seat, because no LLM in the pipeline starts or directs another (AGENTS.md hard rules). The lead is the console. Write the notes to `.evolve/evidence/<topic>/`, one file for each seat and block. Git ignores that folder. The dossier and its source list are the record.

## Red flags

| Thought | Correction |
|---|---|
| "I know what these projects do." | Search and read them. The research must be live. |
| "One researcher can cover all the angles." | One context reads fewer sources. Split the question. |
| "The seats can compare notes to save time." | Debate converges on the majority. Keep the seats alone until the barrier. |
| "I feel sure, so I will stop." | Confidence is not a gate. Read the band and the ledger. |
| "The band is LOW, but the budget is nearly spent." | Choose STOP-INCOMPLETE and write the gaps. Do not claim a complete run. |
| "The dossier can also decide." | The dossier finds. The plan and the ADR decide. |
| "The plan is done, so the work is done." | Each component needs its inbox item, or nobody builds it. |
| "This number is from a search summary, but it is probably right." | Mark it "(search summary)", or fetch the page. |
