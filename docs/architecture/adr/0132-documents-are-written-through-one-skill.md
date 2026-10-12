# ADR-0132: Documents are written through one skill that justifies each decision, and the skill is on by default

- **Status:** Accepted (2026-10-12). The skill, this record and the console default are in force. The kernel preload (plan K2) is a lane.
- **Plan:** [docs-write-skill-2026-10.md](../../plans/docs-write-skill-2026-10.md).
- **Research:** [docs-write-skill-2026-10.md](../../research/docs-write-skill-2026-10.md).
- **Skill:** [skills/docs-write/SKILL.md](../../../skills/docs-write/SKILL.md).
- **Relates to:**
  - [ADR-0115](0115-commit-gate-keeps-history-and-records-waivers.md): a docs diff still gets one review pass before the commit gate;
  - [ADR-0099](0099-deliverable-kinds.md): the skill joins the `solution-*` skills in a document cycle;
  - [ste100-writing.md](../../conventions/ste100-writing.md) and [markdown-structure.md](../../conventions/markdown-structure.md): the skill gives their short form; this record changes one rule of the second.
- **Evidence:**
  - Nothing in `skills/` covers the writing of a document; the rules are spread over six sources (research F2.6).
  - The front matter rule has 2 followers in 367 files (F2.3).
  - No outside source covers types, reasoning, completeness and delivery together (research §4).

## Context

The operator asked for documents that a person reads to learn, with the reasoning that justifies each decision. The house already has a reasoning chain in its recent design sets, but no skill describes it. Each author learns it from old files, and the rules for text are in several places.

## Decision

1. **One skill.** `docs-write` holds the rules for every document that a person reads.
2. **Reasoning first.** Each decision cites its findings and its rejected options. A plan scores at least two designs.
3. **One type, one home.** Each document has one type and one folder, as in Diátaxis and `docs/README.md`.
4. **Templates on demand.** The templates are a reference file, not part of the preload.
5. **Checks at the end.** The STE lint on the new text and the link check close each document.
6. **Header block, not front matter.** New documents under `docs/` start with links and a status line. Skills and agent files keep their front matter.
7. **On by default.** The kernel preloads the compact form into each dispatch that writes a document. `AGENTS.md` tells each CLI to load it.

## Consequences

- Good: a reader can follow each decision to its evidence. A new author has one place to learn the house shape.
- Good: the preload is small (1,526 bytes, about 0.4 K tokens) and stays in the cacheable prefix.
- Cost: each code writer dispatch carries 1,526 bytes more of preloaded text.
- Cost: the front matter rule of `markdown-structure.md` no longer applies to new documents.

## Rejected

- **Templates only:** complete, but reasons stay shallow (plan §4, A).
- **Reader types only:** easy to read, but no justification of decisions (plan §4, B).
- **Short rules and checks only:** cheap, but the checks cannot see a weak reason (plan §4, D).
- **Emphatic capitals in the rules:** newer models over-trigger on them (research P3.7).
