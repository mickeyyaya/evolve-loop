# A review for changes that touch only documents (research, 2026-10)

- **Status:** complete, 2026-10-20.
- **The skill:** [skills/docs-review/SKILL.md](../../skills/docs-review/SKILL.md). **The gate:** [internal-commitgate.md](../architecture/packages/internal-commitgate.md). **Related:** [docs-write-skill-2026-10](docs-write-skill-2026-10.md) (ADR-0132).
- **Method:**
  - a read of the commit gate and of the review skills under `skills/`;
  - the reviews of the wave-integration design set on 2026-10-20;
  - live web searches on 2026-10-20 (section 3).

## Table of contents

- [1. The request](#1-the-request)
- [2. Local evidence](#2-local-evidence)
- [3. Current practice](#3-current-practice)
- [4. Approaches and the winner](#4-approaches-and-the-winner)
- [5. Limits](#5-limits)

## 1. The request

The operator asked on 2026-10-20: "why do we run code review against doc diff? It should be "doc review", not code review". Then: "build the doc review skill and use it directly, research online to learn from them".

## 2. Local evidence

**F2.1** The commit gate asks every change for a simplify capability and a review capability. Each synonym is a code reviewer: `code-simplifier`, `code-reviewer` or a `<lang>-reviewer` (`go/internal/commitgate/reviewers.go:8-9`). Its only waiver is for a Go change that is proven comment-only (`go/internal/commitgate/comment_only.go:29`).
*Implication:* a change of Markdown only must run code reviewers to commit.

**F2.2** On 2026-10-20 the docs-only commit of the wave-integration design set was refused: "missing required review capability: simplify review".
*Implication:* the console ran a simplifier and a code reviewer on prose to satisfy the gate.

**F2.3** Of those two passes, the code reviewer found one false claim and two real gaps. The simplifier found two real inconsistencies, one false finding, and four comments on duplication. An architecture review of the same set found 9 HIGH design holes. The best findings checked the claims of the text against the code, and the decisions against each other.
*Implication:* a docs review has its own lenses: claims against the code, the reasoning chain, and the consistency of a design set. Style is the job of the lint.

**F2.4** `docs-write` already holds the house rules, the types and the checks of a document (`skills/docs-write/SKILL.md`). `evolve docs ste-lint` checks the measurable STE rules.
*Implication:* a docs review does not repeat the house rules. It applies them, and it runs the lint first.

## 3. Current practice

The searches ran on 2026-10-20. "acc." marks a page with no date of its own.

**M1** The Diátaxis theory of quality splits functional quality (accuracy, completeness, consistency, usefulness, precision) from deep quality (fit to the need of the reader). Each property can fail on its own. Its own users say that the structure of Diátaxis does not prove accuracy.
Sources: [Diátaxis](https://www.diataxis.fr/quality/), acc.; [a Diátaxis review skill](https://tessl.io/registry/skills/github/oxidecomputer/claude-plugins/diataxis/review), acc.
*Implication:* the review checks the type and the fit of a document, and checks accuracy as a separate lens.

**M2** Fuchsia gives a small docs change a normal technical review. A large change of exposition or of organization needs the approval of the docs team.
Source: [Fuchsia docs OWNERS](https://fuchsia.googlesource.com/fuchsia/+/main/docs/OWNERS), acc.
*Implication:* the depth of the review follows the size and the type of the change.

**M3** Review guides split three roles. A peer checks structure and style. A subject expert checks the facts and the examples. An editor checks the language.

An expert who rewrites the style works outside the role.
Sources: [Helply](https://helply.com/blog/documentation-review-process), acc.; [River](https://rivereditor.com/blogs/add-technical-writing-comments-no-rewrite), acc.
*Implication:* one lens for each role, and the facts lens does not edit the style.

**M4** Docs-as-code guides check each referenced symbol, flag and config key against the source, and run each code sample. When a document contradicts the code, the document changes.
Sources: [docs-guard](https://skillselion.com/skills/amelnagdy/guard-skills/docs-guard), 2026; [ln-600 docs auditor](https://playbooks.com/skills/levnikolaevich/claude-code-skills/ln-600-docs-auditor), 2026-01.
*Implication:* the claims lens opens each cited `path:line` and each named symbol.

**M5** An ADR review asks seven questions:

1. Does the problem merit a record?
2. Are the options real, and is one missing?
3. Are the drivers complete?
4. When drivers conflict, are they ranked?
5. Does the choice solve the problem, with a sound rationale?
6. Are the pros and cons fair?
7. Is the decision traceable, with a review date?

The same source names review anti-patterns, such as "Pass Through" and "Copy Edit".
Source: [Zimmermann, ADR review](https://ozimmer.ch/practices/2023/04/05/ADRReview.html), 2023-04-05.
*Implication:* the reasoning lens asks these questions. A review that only copy-edits fails.

**M6** An ADR audit checks for drift from the code, contradictions, stale decisions, missing supersession notes and missing cross-references.
Source: [adr-review skill](https://tessl.io/registry/skills/github/chattocorp/chatto/adr-review), acc.
*Implication:* the consistency lens checks the amendment notes and the cross-links of a design set.

**M7** Prose linters (Vale) and link checkers (lychee) run in CI on the changed files only, before a person reviews.
Sources: [Datadog](https://datadoghq.com/blog/engineering/how-we-use-vale-to-improve-our-documentation-editing-process), acc.; [lychee](https://lychee.cli.rs/overview/), acc.; [SAP](https://architecture.learning.sap.com/docs/community/automated-checks/external-links), acc.
*Implication:* the deterministic checks run first, so the LLM spends no turn on what a tool finds.

## 4. Approaches and the winner

| Approach | Summary |
|---|---|
| A. Accept the code reviewers for docs | The gate stays as it is. A code reviewer and a simplifier read prose. |
| B. A docs waiver | A docs-only change needs no review, as a comment-only change needs none. |
| C. A docs-review skill and capability | A docs-only change needs the capability "docs review". The skill runs the deterministic checks, then four lenses. |

Each criterion scores 1 (poor) to 5 (good).

| Criterion | A | B | C |
|---|---|---|---|
| Finds the defects of a design (F2.3) | 2 | 1 | 5 |
| Cost for each docs commit | 2 | 5 | 4 |
| Matches current practice (M1 to M7) | 1 | 1 | 5 |
| Change size | 5 | 4 | 3 |
| **Total (of 20)** | 10 | 11 | **17** |

**C wins.** A spends two code-review passes on prose and misses design holes (F2.3). B removes the review that found the 9 HIGH holes. C takes from B the deterministic checks before any LLM turn (M7), and from A the diff scope of the existing review skills.

The decisions:

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | A change whose paths all end in `.md` needs the capability "docs review", satisfied by `docs-review`. It needs no simplify and no code review. | F2.1, F2.3 | A docs waiver: it removes the lens that found the design holes. |
| D2 | A change with any other path keeps the simplify and review capabilities. `docs-review` does not satisfy them. | F2.1 | One review for all: a docs review does not check code. |
| D3 | The skill runs `ste-lint`, the link check and the citation check before any lens. A failed check stops the review. | M7, F2.4 | Lenses first: an LLM turn finds what a tool finds. |
| D4 | The skill has four lenses: type and fit, claims against the code, the reasoning chain, and the consistency of the set. The depth follows the type and the size of the change. | M1 to M6, F2.3 | One general pass: it copy-edits and passes through (M5). |
| D5 | A finding names `path:line`, the evidence and a fix. A claim that the reviewer did not check is not a finding. | M4, F2.3 | Free comments: the false findings of F2.3. |

## 5. Limits

- Most sources are vendor pages and community skills (M3, M4, M6). M1, M2 and M5 are primary.
- The skill checks the claims that a document cites. It cannot find a claim that the document leaves out.
- A Markdown file outside `docs/`, such as a skill, is also docs-only for the gate (D1). A skill file is an instruction to an agent, so the lens of claims applies to it as well.
