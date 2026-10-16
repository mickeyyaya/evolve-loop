---
score_cap:
  - criterion: "The markdown-structure convention body states no unscoped frontmatter or TLDR mandate"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1907_001 ./acs/cycle1907"
  - criterion: "Frontmatter and TLDR rules are scoped to skill and agent files, and docs/ documents start with links and a status line"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1907_00[23]' ./acs/cycle1907"
  - criterion: "The ADR-0132 link and the table-of-contents rule over 100 lines are kept"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1907_00[24]' ./acs/cycle1907"
  - criterion: "The convention passes ste-lint in strict mode"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run TestC1907_005 ./acs/cycle1907"
---

# Eval: Align the markdown-structure convention body with its ADR-0132 amendment

> Pins the fix for docs/conventions/markdown-structure.md. Its 2026-10-12 banner (line 8) says a new document under docs/ uses no YAML front matter and no TLDR block. Its body (line 17 and the Frontmatter, TLDR and Full Template sections) still said both are mandatory for every file. Source incident: cycle 1907 inbox item docs-convention-frontmatter-drift.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| contradiction-removed | No unscoped frontmatter or TLDR mandate in the body | 7/10 | `TestC1907_001` |
| rule-scoped | Skills and agents keep front matter; docs/ uses links and a status line | 6/10 | `TestC1907_002`, `TestC1907_003` |
| preservation | ADR-0132 link and TOC-over-100-lines rule kept | 5/10 | `TestC1907_002`, `TestC1907_004` |
| ste | Strict ste-lint reports 0 findings | 4/10 | `TestC1907_005` |
