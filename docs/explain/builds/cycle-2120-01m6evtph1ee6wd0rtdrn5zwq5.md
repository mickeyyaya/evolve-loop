# Build Explanation — Cycle 2120

## Build Binding
- Cycle: 2120
- Base SHA: 2003532014179809b458f854db8597d209c04684

## Summary
The build adds a research note on the `agy changelog` releases 1.4.3 to 1.4.12 and a solution set that proposes no persona change.

## Rationale
ADR-0133 requires a 10-cycle baseline (milestone M2) before a persona trim.
A note with a verdict for each persona is the smallest change that answers the inbox item.

## Changed Areas
- `docs/research/practice-agy-cli-changelog-2026-11.md` — explains the changelog lines as untrusted data and gives one ADR-0133 verdict for each of the six core personas.
- `docs/research-index/2026-10-12-03-the-loop-watches-cli-practice-and-trims-its.md` — adds the entry row for the new note and keeps the October rows.
- `docs/research-index.md` — regenerated from the entry files so the index check stays in sync.
- `solutions/practice-agy-cli-changelog-command-772be890-202610270728/proposal.md` — answers the request and lists the persona verdicts.
- `solutions/practice-agy-cli-changelog-command-772be890-202610270728/recommendation.md` — compares three options and names Option 1.
- `solutions/practice-agy-cli-changelog-command-772be890-202610270728/assumptions-and-evidence.md` — holds A1 to A4 and E1 to E7 that the other files cite.
- `solutions/practice-agy-cli-changelog-command-772be890-202610270728/options/1-no-persona-change-record-verdict.md` — the recommended strategy.
- `solutions/practice-agy-cli-changelog-command-772be890-202610270728/options/2-edit-personas-now.md` — the rejected strategy that edits the builder now.
- `solutions/practice-agy-cli-changelog-command-772be890-202610270728/options/3-deterministic-guard-and-follow-up-item.md` — the runner-up strategy that changes bridge code.

## Design Decisions
The note covers ten releases, not six, because the live command lists 1.4.9 to 1.4.12 after the inbox item was filed.

## Verification
`evolve solution check` reports OK, and the ste-lint run reports 0 findings.

## Compatibility
No file under `agents/` or `go/` changes.

## Limitations
The note cannot size the token saving of a builder trim, because the M2 baseline does not exist.
