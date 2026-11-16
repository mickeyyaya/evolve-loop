---
score_cap:
  - criterion: "The agy notes record the docs index URL of the item as data, and mark it as read-at-link"
    max_if_missing: 6
    evidence: "grep -q 'https://aistudio.google.com/llms.txt' skills/model-notes-agy/COMPACT.md"
  - criterion: "The agy notes record the coding-agent skills guide URL of the item"
    max_if_missing: 6
    evidence: "grep -q 'https://aistudio.google.com/docs/coding-agents.md' skills/model-notes-agy/COMPACT.md"
  - criterion: "The new note cites the prompting-strategies source and states that no persona changes under ADR-0133"
    max_if_missing: 7
    evidence: "grep -E 'prompting-strategies' skills/model-notes-agy/COMPACT.md | grep -q 'No persona changes under ADR-0133'"
  - criterion: "The new note keeps the Docs MCP server unused and gives the CLI reason"
    max_if_missing: 6
    evidence: "grep -E 'prompting-strategies' skills/model-notes-agy/COMPACT.md | grep -qi 'MCP stays unused'"
  - criterion: "The agy notes pass the sentence lint and keep the earlier bullets"
    max_if_missing: 5
    evidence: "go/evolve docs ste-lint --strict skills/model-notes-agy/COMPACT.md && test \"$(grep -c '^- ' skills/model-notes-agy/COMPACT.md)\" -ge 15"
---

# Eval: Gemini prompting guide note for item 6a71a881 (cycle 2445)

> Pins the document deliverable for the practice-watch item `practice-gemini-prompting-guide-6a71a881-202611162241`. The changed text of the source is three header links (the docs index, the Docs MCP server, the coding-agent skills guide) and no prompting guidance. The note reads those lines as data, records the two CLI-reachable URLs, leaves the MCP server unused, and states that no persona changes under ADR-0133. Source incident: the notes of cycles 2286 and 2407, which this note follows.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| data-not-instructions | Index URL and skills-guide URL recorded as data | 6/10 | `grep` of both URLs in COMPACT.md |
| adr-0133-verdict | Prompting-strategies source cited with the no-persona-change verdict | 7/10 | `grep` pipe on the note line |
| cli-not-mcp | Docs MCP server stays unused, with the reason | 6/10 | `grep` pipe on the note line |
| lint-and-preserved | Zero lint findings, earlier bullets kept | 5/10 | `evolve docs ste-lint --strict` |
