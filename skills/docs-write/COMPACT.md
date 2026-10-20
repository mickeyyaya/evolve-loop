---
name: docs-write
description: Preloaded form of docs-write — the rules for any document that this dispatch writes.
---

# docs-write (preloaded)

When this dispatch writes or changes a document, follow these rules. The full skill, with the templates, is `skills/docs-write/SKILL.md`.

- **One type, one home:**
  - research → `docs/research/`; plan → `docs/plans/`; decision → `docs/architecture/adr/`;
  - spec of record → `docs/architecture/`; rule → `docs/conventions/`; task → `docs/operations/` or `docs/guides/`;
  - the four files of a design set link to each other.
- **Every claim has a source:**
  - `path:line`, a command with its output, a dated number, or a link;
  - your own inference says "synthesis".
- **Every decision shows its reasoning:**
  - fact → implication → decision → consequence, with ids `F`, `R` and `D`;
  - the rejected options, and the fact that rules out each one;
  - a plan scores at least two candidate designs.
- **Teach first:** quote the request, give the terms and one worked example, and put the answer first in each section.
- **ASD-STE100:**
  - an instruction has 20 words or fewer, a description 25 or fewer;
  - active voice, no "-ing" form, "must" or "can" only;
  - six sentences or fewer in a paragraph; a table for data with two or more attributes.
- **Index it:** add one entry file for a design set to `docs/research-index/`, then run `evolve docs index generate`. Do not edit the generated region of `docs/research-index.md`.
- **Check it:**
  - `evolve docs ste-lint --strict --changed refs/remotes/origin/main --added` reports 0;
  - each relative link resolves.
