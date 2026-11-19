# Build Explanation — Cycle 2526

## Build Binding
- Cycle: 2526
- Base SHA: 50a0c02d5556a2e829b40fce171901e1b8bbe068

## Summary
One research note explains the Claude release notes snapshot after `2026-11-17T18:47:38Z` as untrusted data. It gives each core persona a no-change verdict under ADR-0133.

## Rationale
The live release notes page has no entry newer than `2026-11-17`. The quoted lines name a Compliance API security posture endpoint and a cut-off AWS host fragment. Neither item changes prompt behavior, so a note with no persona or model-notes edit is the smallest correct change.

## Changed Areas
- `docs/research/practice-claude-model-release-notes-security-posture-aws-7177779e-202611190048.md` — the note with six fact rows, six persona verdicts, the rejected option, and decisions D1 and D2.
- `docs/research-index/2026-11-19-01-practice-claude-model-release-notes-7177779e.md` — the index entry file linking the note.
- `docs/research-index.md` — the generated research index updated with `evolve docs index generate`.

## Design Decisions
`skills/model-notes-claude/COMPACT.md` and all `agents/` persona files stay unchanged. The Compliance API endpoint and AWS platform infrastructure are administrative features, not model-behavior rules.

## Verification
All ten predicates in `go/acs/cycle2526` pass. Strict `ste-lint` reports zero findings on the new note, and the native ACS suite passes with zero red predicates.

## Compatibility
No persona under `agents/`, no model notes file, and no Go source file changes.

## Limitations
The inbox snapshot quotes UI shell glyphs, a 2026-11-05 Compliance API endpoint, and a cut-off 2026-05-11 AWS entry. The note treats the cut-off fragment as incomplete untrusted data and not as a rule.
