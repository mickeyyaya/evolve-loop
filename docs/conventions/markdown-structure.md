---
name: markdown-structure
description: Canonical schema for all markdown files in the evolve-loop project — frontmatter, TLDR, TOC, tables, voice
metadata:
  type: convention
---

> **Amendment (2026-10-12, [ADR-0132](../architecture/adr/0132-documents-are-written-through-one-skill.md)).** A new document under `docs/` does not use the YAML front matter or the `## TLDR` block. It starts with links to its design set and a status line, and a table of contents over 100 lines. On 2026-10-12, only 2 of 367 documents used the front matter, and no tool read it. Skills and agent files keep their front matter, because the CLIs read it to load them. The [docs-write](../../skills/docs-write/SKILL.md) skill has the templates.
>
> **Canonical markdown schema.** Every reference, architecture, skill and agent doc in this project follows this schema. Tools: `legacy/scripts/utility/lint-markdown-structure.sh` (WARN linter), `legacy/scripts/utility/extract-tldr.sh` (TLDR extractor).

## TLDR

**Synopsis:** Skill and agent files start with a YAML frontmatter block and a TLDR section. A document under `docs/` starts with links to its design set and a status line. All files use a TOC over 100 lines, tables for data and imperative headings.

**Key points:**
- Skill and agent files require frontmatter, with the `name`, `description` and `metadata.type` fields.
- Skill and agent files require a TLDR section, with the sub-structure synopsis + key points + non-goals.
- A document under `docs/` has no frontmatter and no TLDR. It starts with links to its design set and a status line.
- A file of more than 100 lines requires a TOC.
- Tables are preferred over prose lists for structured data.
- Use the imperative voice in H2 headings (for example, "Run the linter", not "Running the linter").
- Use one-line comments only. Do not use multi-paragraph docstrings.
- Inter-doc links use the `[[name]]` slug pattern.

**Non-goals:** to enforce this schema retroactively on legacy files this cycle; to gate ship on lint failures; to reformat prose body paragraphs.

## Table of Contents

1. [Frontmatter Block](#frontmatter-block)
2. [TLDR Section](#tldr-section)
3. [Table of Contents Rule](#table-of-contents-rule)
4. [Tables Over Prose](#tables-over-prose)
5. [Voice and Headings](#voice-and-headings)
6. [Comment Style](#comment-style)
7. [Inter-doc Links](#inter-doc-links)
8. [Full Template](#full-template)
9. [Linting](#linting)

---

## Frontmatter Block

A skill file or an agent file must start with a YAML frontmatter block, because the CLIs read it to load the file. A new document under `docs/` has no frontmatter ([ADR-0132](../architecture/adr/0132-documents-are-written-through-one-skill.md)).

```yaml
---
name: <kebab-case-slug>
description: <one-line summary — used by lint and discovery tools>
metadata:
  type: <convention | architecture | agent | skill | reference | incident | guide>
---
```

**Required fields:**

| Field | Format | Example |
|-------|--------|---------|
| `name` | kebab-case, no spaces | `incremental-intent` |
| `description` | ≤120 chars, one sentence | `Delta-mode intent resolution for multi-cycle batches` |
| `metadata.type` | enum (see above) | `architecture` |

Optional fields:

| Field | When to add |
|-------|-------------|
| `metadata.cycle` | Incident docs — record the cycle number |
| `metadata.date` | Incident docs — ISO date |
| `metadata.severity` | Incident docs — LOW/MEDIUM/HIGH/CRITICAL |

---

## TLDR Section

A skill file or an agent file requires a TLDR. Put it immediately after the blockquote header (if present) or after the frontmatter. A new document under `docs/` has no TLDR. It starts with links to its design set and a status line.

```markdown
## TLDR

**Synopsis:** <one sentence summary of the file's purpose>

**Key points:**
- <key point 1>
- <key point 2>
- <key point 3>

**Non-goals:** <what this file does NOT cover, one line>
```

The TLDR is the first thing readers and tools see. Keep the synopsis under 25 words. Key points are 2–5 bullets. The Non-goals line prevents scope creep.

---

## Table of Contents Rule

Add a TOC when the file exceeds 100 lines. Format:

```markdown
## Table of Contents

1. [Section Name](#anchor)
2. [Another Section](#another-anchor)
```

Anchors are lowercase. Hyphens replace spaces, and special chars are dropped. Do not add a TOC for files under 100 lines. It adds noise without benefit.

---

## Tables Over Prose

Use tables for any structured comparison, parameter list, enum description or mapping. Prefer this form:

```markdown
| Key | Value | Notes |
|-----|-------|-------|
| ... | ...   | ...   |
```

Prefer it to prose like "The key can be X, Y, or Z. X means this. Y means that."

Exception: a narrative explanation that needs sentences to show causality belongs in prose paragraphs, not in tables.

---

## Voice and Headings

Use imperative voice in H2 and H3 headings:

| Use | Avoid |
|-----|-------|
| `## Run the linter` | `## Running the linter` |
| `## Configure the gate` | `## Gate configuration` |
| `## Add a challenged premise` | `## Adding challenged premises` |

Body prose can use any clear voice. Headings must be imperative.

---

## Comment Style

Use one-line comments only. Do not use multi-paragraph docstrings or comment blocks.

```markdown
<!-- one-line note here -->
```

Use anchors for tool-targeted sections:

```markdown
<!-- ANCHOR:acceptance_criteria -->
```

Do not add comments that explain WHAT the section does. The heading already does that.

---

## Inter-doc Links

To refer to related documents in body text, use the `[[name]]` slug pattern:

```markdown
See [[incremental-intent]] for the delta-mode design.
```

The `name` field in the frontmatter of the referenced file is the canonical slug. Unresolved links (to a file that does not exist yet) are permitted. They mark future work. Lint tools report them as INFO, not WARN.

For actual hyperlinks, use standard markdown: `[text](path/to/file.md)`.

---

## Full Template

Use this template for a document under `docs/`:

```markdown
# <Title>

> Design set: [research](<path>) · [plan](<path>) · [decision](<path>) · [spec](<path>)
> Status: <draft | accepted | superseded> (<YYYY-MM-DD>)

## Table of Contents    ← omit if file < 100 lines

1. [Section](#section)

## Section

<content>
```

Use this template for a skill file or an agent file:

```markdown
---
name: <slug>
description: <one-line summary>
metadata:
  type: <type>
---

> **<one-line blockquote header summarizing the file's role in the system>**

## TLDR

**Synopsis:** <one sentence>

**Key points:**
- <point>
- <point>

**Non-goals:** <one line>

## Table of Contents    ← omit if file < 100 lines

1. [Section](#section)

---

## Section

<content>

---

## References

- `path/to/file.sh` — <one-line description>
- [[related-doc]] — <one-line description>
```

---

## Linting

Run the WARN-only linter against a directory or file list:

```bash
bash legacy/scripts/utility/lint-markdown-structure.sh <path> [<path>...]
```

Extract the TLDR of any compliant file:

```bash
bash legacy/scripts/utility/extract-tldr.sh <path/to/file.md>
```

Exit codes: 0 = found, 1 = not found.

The linter always exits 0 (WARN-only). This cycle adds no gate integration.

---

## References

- `legacy/scripts/utility/lint-markdown-structure.sh` — WARN-only linter
- `legacy/scripts/utility/extract-tldr.sh` — TLDR extractor
- [[feedback_skill_file_structure]] — the operator memory entry that motivated this convention
