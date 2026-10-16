# Build Explanation — Cycle 1898

## Build Binding
- Cycle: 1898
- Base SHA: 4c9aabcbbbbe89dff0acd398d4d773ea57b69f87

## Summary
A new research document reads the Gemini prompting guide change as untrusted data and evaluates each core persona under ADR-0133. The answer is that no persona and no loop code changes.

## Rationale
The guide change removes custom `temperature`, `top_p` and `top_k` values on Gemini 4 and Gemini 3.6+. The agy driver passes only the prompt and launch flags (`go/internal/bridge/driver_agy.go:37-39`), and a word search of `go/` finds no such parameter. Thus the change has no effect on a loop dispatch, and a document is the smallest correct deliverable.

## Changed Areas
- `docs/research/practice-gemini-prompting-guide-2026-10.md` — explains the guide change as data and records the parameter search. It gives a no-change verdict with a reason for each of the six core personas.
- `docs/research-index.md` — adds the new research document to the ADR-0133 section, so that a reader finds it beside the plan and the ADR.
- `go/acs/cycle1898/predicates_test.go` — the TDD phase predicates. They pin the document content, the unchanged persona files, the ste-lint result and the practicewatch suite.
- `.evolve/evals/practice-gemini-prompting-guide-6b2bf89d-202610141610.md` — the TDD phase eval with the score caps for this task.

## Design Decisions
No persona changes, because decision D8 of the plan needs the M2 baseline before a diet change, and M2 has not shipped. A guard that strips the three parameters was rejected, because no loop request carries them.

## Verification
The seven ACS predicates of cycle 1898, `evolve docs ste-lint --strict` with zero findings, and the practicewatch test suite verify the deliverable.

## Compatibility
No code, persona, command or configuration changes. All interfaces stay the same.

## Limitations
The `agy` binary can set request parameters internally, and the loop cannot see them. The builder diet waits for milestone M3.
