# Templates for each document type

> Each template lists the sections in order. A section that does not apply stays, with one line that says why. The empty section tells the reader that you considered it. Sources for each shape: [docs/research/docs-write-skill-2026-10.md](../../../docs/research/docs-write-skill-2026-10.md) §2.

## Research

The research dossier holds the evidence and current practice. It does not decide. The plan decides.

```markdown
# <Subject>: <what the research covers> (research, <yyyy-mm>)

> The plan that uses this research: [<plan>](../plans/<plan>.md). The design: [<spec>](../architecture/<spec>.md). The decision record: [ADR-<nnnn>](../architecture/adr/<nnnn>-<slug>.md).

Date: <yyyy-mm-dd>. Method: <what you measured, read and searched>. Each local claim names its command or file. "Synthesis" marks our own inference.

## Table of contents
## 1. Local evidence          (F1.1, F1.2 ... each with *Implication:*)
## 2. <topic of the outside survey>   (P2.1 ... each with its link)
## 3. Adopt or reject         (a table: practice | decision | reason)
## 4. Refinements to adopt    (R1, R2 ... each cites the findings it comes from)
```

## Plan

The plan turns findings into decisions and work. A reader of the plan must be able to say why each component exists.

```markdown
# Plan: <the outcome, as a sentence>

- **Status:** <proposed | accepted | built>, <date>.
- **Design:** [...]. **Decision:** [ADR-<nnnn>](...). **Research:** [...].

## 1. Request          (the words of the person who asked, quoted, with the date)
## 2. Facts            (a table: fact | research id)
## 3. Goals and non-goals
## 4. Candidate designs (at least two; a table of criteria with scores; why the winner wins)
## 5. Decisions        (a table: id | decision | research id; each one short)
## 6. Components       (a table: id | component | red test first | lane or owner)
## 7. Order            (numbered, with the reason for the order)
## 8. Verification     (before and after numbers; how each goal is measured)
## 9. Risks            (a table: risk | control)
## 10. Open questions
```

## ADR

The ADR is the short, stable record of one decision. A reader who reads only the ADR must know what was decided, why, and what it costs.

```markdown
# ADR-<nnnn>: <the decision, as a sentence>

- **Status:** <Proposed | Accepted | Superseded by ADR-x> (<date>).
- **Plan:** [...]. **Research:** [...]. **Design:** [...].
- **Relates to:** (each linked ADR with one line on how)
- **Evidence:** (the three to five facts that force the decision, with research ids)

## Context        (the forces, in a short paragraph)
## Decision       (numbered, one rule per item, 20 words or fewer each)
## Consequences   (good and bad)
## Rejected       (each option with the fact that rules it out)
```

## Architecture

The architecture spec is the spec of record. It explains how the part works and why it works that way.

```markdown
# <Subject>: <the idea in one line>

> [ADR-<nnnn>](...) · plan: [...] · research: [...].
> This file is the spec of record for <what>. <Which other file holds what>.
>
> **Terms used throughout:** (each term with one line)

## Table of contents
## 1. Goals          (a table: goal | measure | target; the order of the goals)
## 2..n. <one section for each part>, each in the shape:
###   Issue    (what goes wrong today, with evidence)
###   Gap      (why the current design cannot fix it)
###   Solution (the design, with a diagram when the flow has branches)
## Guards         (a table: rule | guard | state)
## Limits         (what the design does not do)
```

## Convention

```markdown
# Convention: <the rule in a few words>

**Status:** active · **Introduced:** <date or cycle> · **Scope:** <files or roles>

## The rule      (the rule, then the one example that shows it)
## Why           (the incident or the evidence that made the rule)
## What this convention does not do
## Regression guard   (the test or the check that enforces it)
```

## How-to

A how-to serves a reader who knows the system and has one task. It has no long explanation. It links to the explanation.

```markdown
# How to <task>

Before you start: (the conditions, as a list)

1. <one instruction, 20 words or fewer>
2. ...

Check the result: (the command and the output that proves success)
If it fails: (the common causes and the fix for each)
```

## Concept

A concept primer serves a new reader. It builds the mental model before any rule.

```markdown
# <Concept>

## The idea in one paragraph
## A worked example     (real values from this repository)
## How it fits the rest (a diagram, and links to the architecture specs)
## Where to go next
```

## Incident

```markdown
# <date>: <what broke, as a sentence>

## Summary            (impact, duration, root cause, fix, in five lines)
## Timeline           (a table: time | event | evidence)
## Root cause         (the chain from the first cause to the symptom)
## What changed       (each fix with its pull request and its regression test)
## What we learned    (each lesson with the rule or the guard it became)
```
