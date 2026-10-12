# Plan: one skill writes every document that a person reads, and it is on by default

- **Status:** accepted, 2026-10-12. K0, K1, K3 and K4 are in this change. K2 (the kernel preload) is done in the lane `feat/docs-write-preload`.
- **Decision:** [ADR-0132](../architecture/adr/0132-documents-are-written-through-one-skill.md). **Research:** [docs-write-skill-2026-10.md](../research/docs-write-skill-2026-10.md). **Skill:** [skills/docs-write/SKILL.md](../../skills/docs-write/SKILL.md).

## Table of contents

1. [Request](#1-request)
2. [Facts](#2-facts)
3. [Goals and non-goals](#3-goals-and-non-goals)
4. [Candidate approaches](#4-candidate-approaches)
5. [The winner and what it takes from the others](#5-the-winner-and-what-it-takes-from-the-others)
6. [Decisions](#6-decisions)
7. [Components](#7-components)
8. [Order](#8-order)
9. [Verification](#9-verification)
10. [Risks](#10-risks)
11. [Open questions](#11-open-questions)

## 1. Request

The operator wrote these on 2026-10-12 (research §1 has the full text):

- "create a skill for document writing for researching and planning by strictly following the rules and policies under docs/ ... The most important is the doc is written for human to read and learn from it ... include the detail logics and reasoning to justify the decisions. Then set this skill as default enabled skill for documentation writing."
- "make sure you also document in detail for the document skill research and planning"
- "propose at least 3 approaches and pick the winner and take the advantages from other approaches to build a new one"

## 2. Facts

| Fact | Research |
|---|---|
| The STE house rules apply to every author and every file in `docs/`. A lint checks them. | F2.1 |
| 845 of 1,492 doc files have old lint findings. A new text can be clean while its file is not. | F2.2 |
| The front matter rule has 2 followers in 367 files, and no tool enforces it. | F2.3 |
| The house design set (research, plan, ADR, spec, with F, R and D ids) exists, but no skill describes it. | F2.5, F2.6 |
| The kernel preloads skills by a compiled rule table; the preload of a skill is its `COMPACT.md`. | F2.7 |
| No single outside source covers types, reasoning, completeness and delivery together. | research §4 |

## 3. Goals and non-goals

**Goals:**

- G1: a reader of any new design document can follow each decision back to its evidence and its rejected options.
- G2: a new reader learns the system from the document, not only the rules.
- G3: every new document passes the STE lint on its new text and the link check.
- G4: every author gets the skill by default: the console, each CLI, and each loop dispatch that writes a document.

**Non-goals:**

- a rewrite of the old documents (the [STE rewrite plan](ste100-docs-rewrite-2026-10.md) owns that);
- a new structure linter;
- a change to the four homes of [docs/README.md](../README.md).

## 4. Candidate approaches

**A. Templates first.** Like The Good Docs Project and the KEP: one complete template for each type, which the author fills in.
- Advantage: each document is complete and has the same shape, so a reader knows where to look.
- Disadvantage: authors fill boxes. The reasons can stay shallow, and the template does not teach why a section exists.

**B. Reader types first.** Like Diátaxis: the skill classifies each document by the need of the reader and keeps the types apart.
- Advantage: each document serves one reader, so it is easy to find and easy to learn from.
- Disadvantage: it says almost nothing about evidence or about the justification of a decision.

**C. Reasoning first.** Like Google design docs, Rust RFCs and MADR. Each document has context, goals and non-goals, options, the decision and its consequences. Each claim has its evidence.
- Advantage: the decisions are justified and checkable. This is the center of the request.
- Disadvantage: heavy for a how-to or a concept primer. It does not say where a document lives.

**D. Short rules and checks first.** Like the current guidance for capable models. A very short skill states the outcome, and deterministic checks (the lint, the link check) catch the rest.
- Advantage: a small preload, and the model uses its own judgment.
- Disadvantage: the checks measure form, not reasoning. A clean text can still hide a weak decision.

Each criterion has a score from 1 (bad) to 5 (good). The criteria come from the goals.

| Criterion (goal) | A. Templates | B. Reader types | C. Reasoning | D. Rules and checks |
|---|---|---|---|---|
| Justified decisions (G1) | 3 | 1 | 5 | 2 |
| A reader learns (G2) | 3 | 5 | 4 | 2 |
| Checkable and house-compliant (G3) | 4 | 2 | 3 | 5 |
| Cheap to load by default (G4) | 2 | 4 | 3 | 5 |
| **Total** | **12** | **12** | **15** | **14** |

C wins. It has the highest score on G1, and the operator named G1 "the most important". D is second, because it is cheap and checkable, but it cannot see a weak reason.

## 5. The winner and what it takes from the others

The skill is C, the reasoning chain, with the strongest part of each other approach added:

| From | What the skill takes | Where it is in the skill |
|---|---|---|
| C (the winner) | the chain fact → implication → decision → consequence; F, R and D ids; rejected options; at least two scored candidates in a plan | SKILL.md "Write the reasoning chain", references/reasoning.md |
| B | one type and one home for each document; a how-to keeps its explanation in another file | SKILL.md "Choose the type and the home" |
| A | a complete template for each type, read on demand, not preloaded | references/templates.md |
| D | a short preload (`COMPACT.md`, under 4 KB) and two deterministic checks at the end | COMPACT.md, SKILL.md "The procedure" step 6 |

Synthesis: the result keeps the score of C on G1. It raises G2 with the types of B, and G3 with the checks of D and the templates of A. It raises G4 with the compact preload of D.

## 6. Decisions

| Id | Decision | Because |
|---|---|---|
| D1 | One skill, `docs-write`, in `skills/docs-write/`, with `SKILL.md`, `COMPACT.md` and `references/`. | F2.6, R1 |
| D2 | The name follows the `<object>-<action>` rule of [skill-naming.md](../conventions/skill-naming.md). | house rule |
| D3 | New documents under `docs/` use the header block (links and status), a table of contents over 100 lines, and a terms list. They do not use YAML front matter. Skills and agent files keep theirs, because the CLIs read it. [markdown-structure.md](../conventions/markdown-structure.md) records the change. | F2.3, R7 |
| D4 | The skill uses plain imperatives with no emphatic capitals. | R9, P3.7 |
| D5 | The kernel preloads `COMPACT.md` into the scout, build and audit dispatches of a document cycle, next to the `solution-*` skills. | F2.7, R8 |
| D6 | The kernel preloads `COMPACT.md` into each source-writing dispatch of a code cycle, after `engineering-craft`, because each landing updates the docs of its surface. | F2.7, R8 |
| D7 | `docs-write` joins the integrity surface, because the kernel preloads it. | F2.7 |
| D8 | `AGENTS.md` and `CLAUDE.md` say: load `docs-write` before you write a document. Every CLI reads `AGENTS.md`. | F2.8, R8 |
| D9 | The skill judges only the new text with `ste-lint --added`. Until that mode lands (T10 of the test-system plan), the author lints the changed files and fixes the findings in the new text. | F2.2 |

**Cost of D5 and D6.** The compact body is 1,526 bytes, so about 0.4 K tokens. It goes into the cacheable prefix of each document-writing dispatch. A code writer then carries 10,795 bytes of preloaded skill bodies below deep, against 9,269 today. At deep and top it carries 13,084, against 11,558 ([skill-overlays.md](../architecture/skill-overlays.md) §Design decisions).

**What changes our mind.** We revisit D6 if the preload raises the cost of a code cycle by more than 2% in the token telemetry. We also revisit it if a sampled review finds worse docs from builders with the preload.

## 7. Components

| Id | Component | Red test first | Owner |
|---|---|---|---|
| K0 | This research, plan and ADR, and the index entry | `evolve docs ste-lint --strict` on each file; the link check | console |
| K1 | The skill: `SKILL.md`, `COMPACT.md`, `references/templates.md`, `references/reasoning.md`; the `plugin.json` entry; the generated command stub (`evolve skills generate`) | `evolve skills check` passes; the stub has no drift | console |
| K2 | The kernel preload: two rules in `compiledDefaultOverlays` (D5, D6), the integrity surface entry (D7), the `TestCompiledDefaultOverlaySkills` pin, and `skill-overlays.md` | A document-cycle build resolves `solution-build` and `docs-write`; a code writer resolves `engineering-craft`, `code-review-simplify` and `docs-write`; the preloaded block is the compact body, under 4 KB | Sonnet lane |
| K3 | The console default: one line in `AGENTS.md` and in `CLAUDE.md` (D8) | review | console |
| K4 | `markdown-structure.md` records D3 | review | console |

## 8. Order

1. K0, K1, K3 and K4 in one pull request. They are text and do not change the kernel.
2. K2 as its own lane, because it changes the prompt of every writer dispatch and needs red tests.

## 9. Verification

| Goal | Measure |
|---|---|
| G1 | A review of the next three design sets: each decision cites findings and rejected options. |
| G2 | A reader who did not write the set can state the main decision and its reason after one read (a review question). |
| G3 | `ste-lint --strict --added` reports 0 on each new document; each link resolves. |
| G4 | K2 tests pass; the session log of a document cycle shows `skill-overlays=[... docs-write]`. |

## 10. Risks

| Risk | Control |
|---|---|
| Authors fill the templates and skip the reasoning. | The reasoning chain is the first section of the skill, and the review checks for rejected options. |
| The preload grows the prompt of each writer. | A compact body under 4 KB, pinned by a test (K2). |
| Two homes for the rules (the skill and the conventions). | The skill gives the short form and links to the conventions. The conventions stay the source. |
| The change to the front matter rule is a change of a convention. | ADR-0132 records it, and the convention file points to it. |

## 11. Open questions

1. Must an older design set (for example the ADR-0131 set) move to the skill shape? This plan says no; a touched file follows the skill for its new text only.
