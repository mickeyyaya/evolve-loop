---
score_cap:
  - criterion: "The note cites the Gemini prompting guide link, the snapshot it follows, the inbox item and ADR-0133"
    max_if_missing: 6
    evidence: "test -f docs/research/practice-gemini-prompting-guide-2026-11.md && grep -q -F 'https://ai.google.dev/gemini-api/docs/prompting-strategies' docs/research/practice-gemini-prompting-guide-2026-11.md && grep -q -F '2026-10-14T16:10:54Z' docs/research/practice-gemini-prompting-guide-2026-11.md && grep -q -F 'practice-gemini-prompting-guide-b79996aa-202611020008' docs/research/practice-gemini-prompting-guide-2026-11.md && grep -q -F 'ADR-0133' docs/research/practice-gemini-prompting-guide-2026-11.md"
  - criterion: "The note explains the item lines (API CLI) as untrusted data and records that the source was read at its link"
    max_if_missing: 6
    evidence: "test -f docs/research/practice-gemini-prompting-guide-2026-11.md && grep -q -F 'API CLI' docs/research/practice-gemini-prompting-guide-2026-11.md && grep -q -i 'untrusted' docs/research/practice-gemini-prompting-guide-2026-11.md && grep -q -F 'HTTP' docs/research/practice-gemini-prompting-guide-2026-11.md"
  - criterion: "The note states that no prompt or persona changes and why, or gives a full ADR-0133 proposal, and ties it to the baseline rule"
    max_if_missing: 7
    evidence: "test -f docs/research/practice-gemini-prompting-guide-2026-11.md && grep -q -E '^Decision D[0-9]+:' docs/research/practice-gemini-prompting-guide-2026-11.md && grep -q -F 'Rejected option' docs/research/practice-gemini-prompting-guide-2026-11.md && grep -q -i 'baseline' docs/research/practice-gemini-prompting-guide-2026-11.md"
  - criterion: "No persona or skill file changes in this cycle"
    max_if_missing: 7
    evidence: "git diff --quiet origin/main -- agents skills && test -z \"$(git ls-files --others --exclude-standard agents skills)\""
  - criterion: "The note holds no placeholder text"
    max_if_missing: 5
    evidence: "test -f docs/research/practice-gemini-prompting-guide-2026-11.md && ! grep -q -i -E 'TBD|TODO|lorem ipsum' docs/research/practice-gemini-prompting-guide-2026-11.md"
  - criterion: "The note and the research index pass the docs-write sentence lint with zero findings"
    max_if_missing: 6
    evidence: "go/bin/evolve docs ste-lint --strict --project-root . docs/research/practice-gemini-prompting-guide-2026-11.md docs/research-index.md"
  - criterion: "The research index links the note"
    max_if_missing: 5
    evidence: "grep -q -F '(research/practice-gemini-prompting-guide-2026-11.md)' docs/research-index.md"
---

# Eval: Gemini prompting guide note, snapshot of 2026-10-14 (cycle 2264)

> Pins the document deliverable for the practice-watch item on the Gemini prompting guide (inbox item practice-gemini-prompting-guide-b79996aa-202611020008). The page changed after the snapshot of 2026-10-14T16:10:54Z, and the item shows only the two words "API CLI". The note (docs/research/practice-gemini-prompting-guide-2026-11.md) reads the page at its link as untrusted data and states whether any prompt or persona changes under ADR-0133. Source incident: the earlier note docs/research/practice-gemini-prompting-guide-2026-10.md, and cycle 2125, which released this item for a capability failure.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| source-cited | Link, snapshot marker, item id and ADR-0133 present | 6/10 | `grep -F` on the note |
| data-not-instructions | Item lines named, page text called untrusted, HTTP outcome recorded | 6/10 | `grep -F` on the note |
| persona-verdict | Decision line, rejected option and baseline tie | 7/10 | `grep -E` on the note |
| no-persona-edit | agents/ and skills/ unchanged | 7/10 | `git diff --quiet origin/main -- agents skills` |
| no-placeholder | No TBD, TODO or lorem ipsum | 5/10 | `grep -i -E` on the note |
| ste-lint | Zero sentence-lint findings | 6/10 | `evolve docs ste-lint --strict` |
| indexed | Research index links the note | 5/10 | `grep -F` on the index |
