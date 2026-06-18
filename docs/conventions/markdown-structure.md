---
name: markdown-structure
description: Canonical schema for all markdown files in the evolve-loop project — frontmatter, TLDR, TOC, tables, voice
metadata:
  type: convention
---

> **Canonical markdown schema** — every reference, architecture, skill, and agent doc in this project follows this schema. Tools: `legacy/scripts/utility/lint-markdown-structure.sh` (WARN linter), `legacy/scripts/utility/extract-tldr.sh` (TLDR extractor).

## TLDR

**Synopsis:** All markdown files must follow this schema: YAML frontmatter block, TLDR section with synopsis + key points + non-goals, TOC for files over 100 lines, tables over prose lists, and imperative voice in headings.

**Key points:**
- Frontmatter is mandatory: `name`, `description`, `metadata.type` fields required
- TLDR section required in every file; synopsis + key points + non-goals sub-structure
- TOC required for files exceeding 100 lines
- Tables preferred over prose lists for structured data
- Imperative voice in H2 headings (e.g., "Run the linter" not "Running the linter")
- One-line comments only; no multi-paragraph docstrings
- Inter-doc links use `[[name]]` slug pattern

**Non-goals:** Enforcing this schema retroactively on legacy files this cycle; gating ship on lint failures; reformatting prose body paragraphs.

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

Every markdown file must start with a YAML frontmatter block:

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

Place immediately after the blockquote header (if present) or after frontmatter. Required in every file.

```markdown
## TLDR

**Synopsis:** <one sentence summary of the file's purpose>

**Key points:**
- <key point 1>
- <key point 2>
- <key point 3>

**Non-goals:** <what this file does NOT cover, one line>
```

The TLDR is the first thing readers and tools see. Keep synopsis under 25 words. Key points are 2–5 bullets. Non-goals prevents scope creep.

---

## Table of Contents Rule

Add a TOC when the file exceeds 100 lines. Format:

```markdown
## Table of Contents

1. [Section Name](#anchor)
2. [Another Section](#another-anchor)
```

Anchors are lowercase, spaces replaced by hyphens, special chars dropped. Do not add a TOC for files under 100 lines — it adds noise without benefit.

---

## Tables Over Prose

Use tables for any structured comparison, parameter list, enum description, or mapping. Prefer:

```markdown
| Key | Value | Notes |
|-----|-------|-------|
| ... | ...   | ...   |
```

Over prose like "The key can be X, Y, or Z. X means this. Y means that."

Exception: narrative explanation that requires sentences to convey causality belongs in prose paragraphs, not tables.

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

One-line comments only. No multi-paragraph docstrings or comment blocks.

```markdown
<!-- one-line note here -->
```

Use anchors for tool-targeted sections:

```markdown
<!-- ANCHOR:acceptance_criteria -->
```

Do not add comments that explain WHAT the section does — the heading already does that.

---

## Inter-doc Links

Reference related documents using the `[[name]]` slug pattern in body text:

```markdown
See [[incremental-intent]] for the delta-mode design.
```

The `name` field in the referenced file's frontmatter is the canonical slug. Unresolved links (pointing to a file not yet created) are allowed — they mark future work. Lint tools report them as INFO, not WARN.

For actual hyperlinks use standard markdown: `[text](path/to/file.md)`.

---

## Full Template

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

The linter always exits 0 (WARN-only). No gate integration this cycle.

---

## References

- `legacy/scripts/utility/lint-markdown-structure.sh` — WARN-only linter
- `legacy/scripts/utility/extract-tldr.sh` — TLDR extractor
- [[feedback_skill_file_structure]] — operator memory entry that motivated this convention
