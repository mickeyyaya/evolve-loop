# Build Explanation — Cycle 2527

## Build Binding
- Cycle: 2527
- Base SHA: d0e4d03a25459979053ab273c4fbf2a64454b44f

## Summary
One research note explains the Claude prompt guide snapshot after `2026-11-15T11:10:47Z` as untrusted data. It gives each core persona a no-change verdict under ADR-0133.

## Rationale
The snapshot diff quotes only two search shortcut key cap symbols from the HTML shell. The live guide repeats model families and rules already in `skills/model-notes-claude/COMPACT.md`. A review note with no persona or model-notes edit is the smallest correct change.

## Changed Areas
- `docs/research/practice-claude-prompting-guide-2c5f9a4d-202611190048.md` — the note with five fact rows, six persona verdicts, decisions D1 and D2, and the rejected option.
- `docs/research-index/2026-11-19-02-practice-claude-prompting-guide-2c5f9a4d.md` — the index entry file that links the note.
- `docs/research-index.md` — the generated research index updated with `evolve docs index generate`.

## Design Decisions
`skills/model-notes-claude/COMPACT.md` and all `agents/` persona files stay unchanged. The snapshot diff holds UI key cap chrome, and the live guide adds no new model rule.

## Verification
All ten predicates in `go/acs/cycle2527` pass. Strict `ste-lint` reports zero findings on the new note and `COMPACT.md`, and the native ACS suite passes with zero red predicates.

## Compatibility
No persona under `agents/`, no model notes file, and no Go source file changes.

## Limitations
The inbox snapshot quotes only the `⌘` and `K` key cap lines from the page shell. The note reads the live guide at its link on 2026-11-19 and treats all guide text as untrusted data.
