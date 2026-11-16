# Build Explanation — Cycle 2446

## Build Binding
- Cycle: 2446
- Base SHA: 491d452285425e5f3377aa42679b7900240e5cd6

## Summary
The build adds one research note for the Claude release-notes practice item, one index entry and one model-notes bullet about the text watermark.

## Rationale
The release notes say a non-default temperature, top_p or top_k can remove the text watermark of four models. No persona sets a sampling value, so no persona changes. The fact is a model fact, so it goes in the Claude model notes.

## Changed Areas
- `docs/research/practice-claude-model-release-notes-7ad7926a-202611162241.md` — new note that treats the page as untrusted data, lists four fact rows and gives six no-change persona verdicts.
- `docs/research-index/2026-11-17-01-practice-claude-model-release-notes-7ad7926a.md` — new index entry that links the note.
- `docs/research-index.md` — regenerated with the new entry by the index generator.
- `skills/model-notes-claude/COMPACT.md` — one added bullet that names the four models and the three sampling parameters.

## Design Decisions
The bullet names only the four models that the page lists. It does not name Fable 5.2 or Mythos 5.2, because the page does not list them.

## Verification
The cycle 2446 predicates pass, and the ACS suite reports red=0. The sentence lint reports zero findings for the note and the model notes.

## Compatibility
No persona, no agy note and no code file changes. The existing model-notes bullets stay as they were.

## Limitations
The note reads the watermark claim from the release notes page and cannot test it, because the pipeline reads no watermark.
