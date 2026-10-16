# Build Explanation — Cycle 1907

## Build Binding
- Cycle: 1907
- Base SHA: 2652b4323740c6090c8c2b25c0936e5e3f23f4b0

## Summary
The markdown-structure convention body now agrees with its ADR-0132 banner. Frontmatter and TLDR rules apply to skill and agent files only. A document under `docs/` starts with links to its design set and a status line.

## Rationale
The banner at line 8 revoked the frontmatter and TLDR rule for `docs/`, but the body still stated it for every file. An edit of the body is the smallest change that removes the contradiction. The banner and the ADR stay the authority.

## Changed Areas
- `docs/conventions/markdown-structure.md` — scopes the TLDR key points and the two schema sections to skill and agent files. The Full Template section gets a `docs/` template with a status line.

## Design Decisions
The file keeps its own frontmatter, because the banner applies to new documents only. The Table of Contents Rule stays unchanged at 100 lines. The skill and agent template stays as the second template.

## Verification
`go test -tags acs -count=1 -v ./acs/cycle1907` passes 5 of 5 predicates. Strict ste-lint reports 0 findings on the file.

## Compatibility
The legacy WARN linter and the TLDR extractor keep their behavior. Skill and agent files keep the same schema.

## Limitations
This change does not remove the frontmatter from existing documents under `docs/`.
