---
name: docs-write
description: Use when you write or change any document for people to read — research dossiers, plans, ADRs, architecture specs, conventions, operations guides, incident records, READMEs and design write-ups under docs/. It gives the type of each document, its home, its template, the reasoning chain that justifies each decision, the ASD-STE100 house rules and the checks to run.
---

# docs-write — documents that a person reads to learn and to decide

> A document in this repository has two readers. A person reads it to understand the system and to judge a decision. An agent reads it to do work. Write for the person first. If a person can follow the reasoning and check each claim, an agent can too.
>
> Sources of this skill: [docs/research/docs-write-skill-2026-10.md](../../docs/research/docs-write-skill-2026-10.md). House rules: [docs/conventions/ste100-writing.md](../../docs/conventions/ste100-writing.md) and [docs/conventions/markdown-structure.md](../../docs/conventions/markdown-structure.md).

## The five laws

1. **One document has one type and one home.** Choose the type before you write (table below). Do not mix a how-to into a design, or a decision into a research dossier.
2. **Every claim has a source.** A local claim cites `path:line`, a command and its output, or a measured number with its date. An outside claim cites a link. Your own inference says "synthesis".
3. **Every decision shows its reasoning.** Write the chain: fact → implication → decision → consequence. Show the alternatives that you rejected and why. A decision without its reasons is an order, not a design.
4. **Teach before you specify.** Give the reader the context, the terms and one worked example before the rules. A new reader must finish with the mental model, not only the list.
5. **The text is checkable.** Run `evolve docs ste-lint --strict --changed <base> --added` and the link check before you hand off. A document that fails its checks is not done.

## Choose the type and the home

| Type | Question it answers | Home | Template |
|---|---|---|---|
| Research dossier | What did we find, and what does current practice say? | `docs/research/<topic>-<yyyy-mm>.md` | references/templates.md §Research |
| Plan | What will we build, in which order, and how do we prove it? | `docs/plans/<topic>-<yyyy-mm>.md` | references/templates.md §Plan |
| Decision record (ADR) | What did we decide, and why? | `docs/architecture/adr/<nnnn>-<decision-as-a-sentence>.md` | references/templates.md §ADR |
| Architecture spec | How does this part of the system work? (the spec of record) | `docs/architecture/<topic>.md` | references/templates.md §Architecture |
| Convention | Which rule must every author follow? | `docs/conventions/<rule>.md` | references/templates.md §Convention |
| How-to or operations guide | How do I do this task? | `docs/operations/` or `docs/guides/` | references/templates.md §How-to |
| Concept primer | What is this, for a new reader? | `docs/concepts/` | references/templates.md §Concept |
| Incident record | What broke, why, and what changed? | `docs/incidents/<date>-<topic>.md` | references/templates.md §Incident |

This table follows Diátaxis: tutorials, how-to guides, reference and explanation each serve one need. A design set for one change has four linked files: research, plan, ADR and architecture spec. Each file links to the other three in its header.

## Write the reasoning chain

Each finding and each decision uses this chain. It is the part that a reader learns from.

```
**F2.3** <a fact, with its source: path:line, a command, a number and its date, or a link>.
*Implication:* <what the fact means for this design, in one sentence>.

| Id | Decision | Because (findings) | Rejected options and why |
|---|---|---|---|
| D4 | <the decision> | F2.3, F3.1 | <option>: <the fact that rules it out> |
```

- Number findings (`F<section>.<n>`), refinements (`R<n>`) and decisions (`D<n>`). A later file cites them by id, so the chain stays visible across files.
- Give at least two candidate designs for a plan. Score them on named criteria in a table, and say why the winner wins.
- Write the consequences of each decision, the good and the bad. Write the condition that makes you change the decision.
- Write the limits: what the design does not do, and what a test cannot prove.
- Mark each open question. Do not hide an assumption inside a sentence.

references/reasoning.md has the patterns: a decision table, a scored comparison, a worked example, a "what changes our mind" line, and a cost line.

## Write for a human reader

- Open with the request or the problem in the words of the person who asked. Quote them.
- Give a short "Terms used throughout" list when the document uses five or more special terms.
- Put the answer first in each section. Then give the detail.
- Use one worked example with real values from this repository. An example teaches faster than a rule.
- Use a table for data with two or more attributes. Use a mermaid diagram when the flow has branches or more than four steps.
- Keep each paragraph on one topic, with six sentences or fewer.

## The house rules (ASD-STE100)

The full rules are in docs/conventions/ste100-writing.md. The short form:

- An instruction has 20 words or fewer. A description has 25 words or fewer.
- Use the active voice, the simple tenses and the imperative. Do not use the "-ing" form, except in a technical name.
- Write "must" for a rule and "can" for a possibility. Do not write "should", "may", "might", "could" or "would".
- Use one word for one meaning. Do not use a synonym for variety.
- A paragraph has six sentences or fewer. A heading is short and is not a sentence.
- Put code names, paths and commands in backticks. Each counts as one word.

## The procedure

1. Read the request. Find the type and the home of each document that the request needs.
2. Read the documents that already cover the subject. Link to them. Do not repeat them.
3. Collect the evidence: local measurements, code reads with `path:line`, and outside sources. Write the findings first.
4. Write the reasoning chain, then the decisions, then the design or the steps.
5. Index the document, and add a pointer in the file that owns the subject:
   - add one entry file to `docs/research-index/`, named `<yyyy-mm-dd>-<nn>-<slug>.md`, that starts with a `## ` heading line;
   - run `evolve docs index generate`. Do not edit the generated region of `docs/research-index.md` by hand;
   - write the links of the entry relative to `docs/`, because the entry renders into `docs/research-index.md`.
6. Run the checks:
   - `evolve docs ste-lint --strict --changed refs/remotes/origin/main --added`
   - each relative link resolves (`[text](path)` targets exist)
   - each `path:line` that you cite points at the code that you describe
7. Run one review pass with the [docs-review](../docs-review/SKILL.md) skill, then the commit gate with `--reviewers docs-review`.

## Red flags

| Thought | Correction |
|---|---|
| "The reader knows the context." | A reader in six months does not. Write the context. |
| "The decision is obvious." | Then the reason is short. Write it. |
| "I will list the options later." | A decision without its options cannot be judged. |
| "This number is roughly right." | Measure it, or say "estimate" and give the method. |
| "I will stack this docs PR on my open PR." | Open each PR from `origin/main`. A squash merge rewrites the commits of the first PR, so the stacked PR conflicts. |
| "One more section on a different topic fits here." | It belongs in its own document of its own type. |
| "The agent needs exact steps, so the why can go." | The why is what lets an agent apply the rule to a new case. |
