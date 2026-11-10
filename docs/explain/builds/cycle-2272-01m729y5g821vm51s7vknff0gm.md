# Build Explanation — Cycle 2272

## Build Binding
- Cycle: 2272
- Base SHA: 389887d3dedfaff4f8d43acfef79a744fd984292

## Summary
The agy model notes gain one bullet about the response prefix. A research note records the review of the Gemini prompting guide under ADR-0133.

## Rationale
The guide describes a completion strategy: the prompt ends with the start of the wanted format. The agy notes are the one home of agy practice, so one bullet there reaches no other family.

## Changed Areas
- `skills/model-notes-agy/COMPACT.md` — adds one bullet that tells an agy seat to write the deliverable skeleton as a response prefix.
- `docs/research/practice-gemini-prompting-guide-completion-prefix-2026-11.md` — new note that explains the guide lines as untrusted data and gives a verdict for each core persona.
- `docs/research-index/2026-11-10-01-practice-gemini-prompting-guide.md` — adds the link to the new note.
- `docs/research-index.md` — adds the same row for the new note at line 661 of the generated index.
- `.evolve/evals/agy-model-notes-completion-prefix.md` — eval file that pins the four score caps of the task.

## Design Decisions
No persona changes, because the guide gives no rule that can be measured. A blanket few-shot rule is rejected, because it adds length for all prompts.

## Verification
All five predicates of cycle 2272 pass (5/5 PASS with `go test -tags acs -count=1 -v ./acs/cycle2272`).

## Compatibility
The four old bullets of the notes stay unchanged. The notes keep 9 lines, below the ceiling of 12.

## Limitations
The effect of the bullet is not measured in this cycle.
