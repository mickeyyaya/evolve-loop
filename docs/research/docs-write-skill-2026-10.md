# A skill for documents that people read to learn and to decide (research, 2026-10)

> The plan that uses this research: [docs-write-skill-2026-10.md](../plans/docs-write-skill-2026-10.md). The decision record: [ADR-0132](../architecture/adr/0132-documents-are-written-through-one-skill.md). The skill: [skills/docs-write/SKILL.md](../../skills/docs-write/SKILL.md).

Date: 2026-10-12. Method: a read of the house rules under `docs/` and of the skill-overlay code, and counts over the document tree. Also a survey of the most used open projects and guides for software design documents. Each local claim names its file or command. "Synthesis" marks our own inference.

## Table of contents

1. [The request](#1-the-request)
2. [Local evidence](#2-local-evidence)
3. [Current practice](#3-current-practice)
4. [What each source teaches](#4-what-each-source-teaches)
5. [Adopt or reject](#5-adopt-or-reject)
6. [Refinements to adopt](#6-refinements-to-adopt)

## 1. The request

The operator wrote this on 2026-10-12:

> "create a skill for document writing for researching and planning by strictly following the rules and policies under docs/. Research for the highest rate github project for documentation writing for project and software designing. The most important is the doc is written for human to read and learn from it. It's not only for AI to do the work but also include the detail logics and reasoning to justify the decisions. Then set this skill as default enabled skill for documentation writing."

Two more requests followed: "make sure you also document in detail for the document skill research and planning", and "propose at least 3 approaches and pick the winner and take the advantages from other approaches to build a new one".

Three requirements follow. The skill must obey the house rules. The documents must teach a person and justify each decision. The skill must be on by default for each author of a document.

## 2. Local evidence

**F2.1** The house rules for text are in [ste100-writing.md](../conventions/ste100-writing.md) (ASD-STE100, Issue 9). They apply to every file in `docs/` and to every author: people, the console and every loop phase. `evolve docs ste-lint` checks the measurable rules.
*Implication:* the skill must carry these rules in short form and must end with the lint.

**F2.2** On 2026-10-12, `evolve docs ste-lint` found 20,808 findings in 845 of 1,492 files. The rules exist, but most old text does not follow them.
*Implication:* the skill cannot ask an author to fix a whole old file. It must judge only the new text. The `--added` mode of the lint (plan T10 of [test-system-efficiency-2026-10.md](../plans/test-system-efficiency-2026-10.md)) does this.

**F2.3** [markdown-structure.md](../conventions/markdown-structure.md) requires YAML front matter and a `## TLDR` section in every file. A count on 2026-10-12 over six folders (`docs/plans`, `docs/research`, `docs/architecture`, its `adr/`, `docs/conventions`, `docs/operations`) found front matter in 2 of 367 files. It found a TLDR in the same 2 files, and no tool enforces the rule.
*Implication:* two patterns conflict. The recent design sets use a header block with links, a status line and a table of contents. The skill must choose one and say why (plan D3).

**F2.4** [docs/README.md](../README.md) gives the home of each kind of document by the question that it answers: `reference/`, `architecture/`, `research/`, and the task and event folders.
*Implication:* the skill can map each document type to one home with this principle.

**F2.5** The recent design sets have one shape. Examples are the agent messaging set (ADR-0129) and the test-system set (ADR-0131). Each set has four linked files: research, plan, ADR and architecture spec. Findings have ids `F<section>.<n>`, refinements `R<n>` and decisions `D<n>`.
*Implication:* the house already has a reasoning chain. No skill writes it down, so each author learns it from old files.

**F2.6** The rules for documents are spread over the conventions, `docs/README.md`, ADR-0115 (one review pass for a docs diff) and operator memory notes. Nothing in `skills/` covers the writing of a document. `solution-build` covers the strategies of a document cycle (ADR-0099), not the shape of a design document.
*Implication:* an author has to find six sources. One skill gives one home to the rules.

**F2.7** The kernel preloads skills into phase prompts by a compiled rule table, `compiledDefaultOverlays` (`go/internal/policy/overlays.go:62`). A rule selects by phase, tier, the write axis and `deliverable_kind`. The materializer prefers a skill's `COMPACT.md` (`skilloverlay.readSkillBody`). `code-review-simplify` keeps its preload under 4 KB, and a test pins it.
*Implication:* "default enabled" in the loop means one more rule in that table, with a compact body. A compiled-default skill also needs an entry in the integrity surface (`go/internal/guards/integrity_surface.go`), because a tampered copy changes every dispatch that loads it.

**F2.8** In Claude Code, a skill loads when its description matches the task. `CLAUDE.md` and `AGENTS.md` load in each session.
*Implication:* "default enabled" for the console means an exact description and one line in `AGENTS.md`, which every CLI reads.

## 3. Current practice

**P3.1 Diátaxis.** Diátaxis divides documentation into four types by the need of the reader: tutorials, how-to guides, reference and explanation. Each type has its own form, and the types must stay apart. A common failure is an explanation inside a how-to guide ([Canonical](https://canonical.com/blog/diataxis-a-new-foundation-for-canonical-documentation), [Medium summary](https://medium.com/@techwritershub/how-to-structure-documentation-using-the-diataxis-framework-70d4a5a61db7)).

**P3.2 Design docs at Google.** A design doc has context and scope, goals and non-goals, the design, cross-cutting concerns and the alternatives considered. A non-goal is a possible goal that the authors choose not to have. The alternatives section makes the authors think about other designs, an existing system, or doing nothing ([industrialempathy.com](https://www.industrialempathy.com/posts/design-docs-at-google/)).

**P3.3 Rust RFCs.** The template asks for the motivation, the rationale and alternatives, and the impact of no change. It also asks for prior art (good and bad experiences of others), unresolved questions and future possibilities ([rust-lang/rfcs 0000-template.md](https://github.com/rust-lang/rfcs/blob/master/0000-template.md), [RFC 2333 prior art](https://rust-lang.github.io/rfcs/2333-prior-art.html)).

**P3.4 Kubernetes KEPs.** A KEP is the standard design document for a significant change. It has motivation, proposal and design details, and it passes several reviews, including production readiness ([kubernetes.dev](https://kubernetes.dev/blog/2025/01/21/sig-architecture-enhancements/)).

**P3.5 Architecture decision records.** `joelparkerhenderson/architecture-decision-record` has about 13,600 stars, the most among ADR collections ([ecosyste.ms](https://sponsors.ecosyste.ms/accounts/joelparkerhenderson), [star-history](https://star-history.com/joelparkerhenderson/architecture-decision-record)). MADR (`adr/madr`, about 2,100 stars) adds decision drivers and the options with their pros and cons ([ecosyste.ms](https://awesome.ecosyste.ms/projects/github.com%2Fadr%2Fmadr)). A 2023 study of GitHub projects found that most use the Nygard template: status, context, decision, consequences ([JKU study](https://research.jku.at/en/publications/using-architecture-decision-records-in-open-source-projects-an-ms/)).

**P3.6 The Good Docs Project.** A community gives free Markdown templates for many document types, with a guide for each ([The Content Wrangler](https://www.thecontentwrangler.com/p/the-good-docs-project-helps-open), [JetBrains Writerside](https://blog.jetbrains.com/writerside/2024/09/new-in-writerside-the-good-docs-project-templates-right-in-your-editor/)).

**P3.7 Skills for capable models.** A skill loads in three layers: the name and description always, the body on a match, the references on demand. The description decides when the skill loads ([skill guides summary](https://medium.com/@quanap5/claude-code-progressive-disclosure-insights-from-my-learning-5244bc9864aa)). Anthropic's current prompting guide says that newer models follow instructions more literally. Emphatic words such as "CRITICAL" can cause over-triggering ([platform.claude.com](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices)).

Secondary reports say that Anthropic removed more than 80% of the Claude Code system prompt for its newest models. They report no loss on coding evaluations. We did not verify this against the primary post ([techstrong.ai](https://techstrong.ai/agentic-ai/anthropic-cut-80-of-claude-codes-system-prompt-heres-why-that-matters-for-your-agents/)).

## 4. What each source teaches

| Source | Strength | Weakness for this request |
|---|---|---|
| Diátaxis (P3.1) | One type for each need of the reader; a document is easy to find and to read. | It says little about how to justify a decision. |
| Google design docs (P3.2) | Goals and non-goals; alternatives considered; the reader comes up to speed. | No evidence rules; no checks. |
| Rust RFC and KEP (P3.3, P3.4) | Rationale, prior art, unresolved questions, the cost of no change. | Long; made for a public proposal, not for each change. |
| ADR, Nygard and MADR (P3.5) | A short, stable record; decision drivers; pros and cons of each option. | One decision only; it does not teach the system. |
| The Good Docs Project (P3.6) | A complete template for each type. | Templates can become form filling with shallow reasons. |
| Skill practice for capable models (P3.7) | A short body; detail on demand; calm wording. | A short body alone does not make good reasoning. |

Synthesis: no one source covers the three requirements. Diátaxis gives the types. The design doc, RFC and ADR give the reasoning. The templates give completeness. The skill practice gives the delivery.

## 5. Adopt or reject

| Practice | Decision | Reason |
|---|---|---|
| The four Diátaxis needs, mapped to the homes of F2.4 | Adopt | One type and one home for each document (F2.4). |
| Goals and non-goals; alternatives considered (P3.2) | Adopt | The operator asks for reasons that justify each decision. |
| Rationale, prior art, unresolved questions (P3.3) | Adopt, in the research and plan templates | They make the reasoning complete. |
| The ADR shape with drivers and rejected options (P3.5) | Adopt | It is already the house shape (F2.5). |
| A template for each type (P3.6) | Adopt, in a reference file read on demand | Completeness without a long preload. |
| YAML front matter in each file (F2.3) | Reject for docs; keep the purpose | 2 of 367 files have it, no tool reads it, and the header block gives the summary first. |
| A short body with detail on demand (P3.7) | Adopt | The preload stays under 4 KB (F2.7). |
| Emphatic words such as CRITICAL or MUST in capitals | Reject | They cause over-triggering in newer models (P3.7). The rules use plain imperatives. |
| A separate doc linter for structure | Reject for now | The STE lint and the link check cover the measurable rules. A review covers the rest. |

## 6. Refinements to adopt

| Id | Refinement | From |
|---|---|---|
| R1 | One skill, `docs-write`, holds the rules for every document that a person reads. | F2.6 |
| R2 | The skill maps each type to one home and one template. | F2.4, P3.1, P3.6 |
| R3 | Each finding has its implication. Each decision cites its findings and its rejected options. | F2.5, P3.2, P3.5 |
| R4 | A plan compares at least two candidate designs with scores on named criteria. | P3.2, P3.3 |
| R5 | A document teaches first: the request, the terms, a worked example, the answer first in each section. | the request, P3.1 |
| R6 | The skill ends with the STE lint on the added text and the link check. | F2.1, F2.2 |
| R7 | The header block, the status line and the table of contents replace the YAML front matter. The convention file records the change. | F2.3 |
| R8 | The kernel preloads `COMPACT.md` into each dispatch that writes a document. The console loads the skill by its description and by one line in `AGENTS.md`. | F2.7, F2.8 |
| R9 | The skill uses calm, plain imperatives. It has no emphatic capitals. | P3.7 |
