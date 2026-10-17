# Build Explanation — Cycle 1924

## Build Binding
- Cycle: 1924
- Base SHA: 956797753730feac37256293089fcb10a0a58730

## Summary
Sixteen rows of the research index have their dropped facts back. The ADR-0115 row now says that the added-comment rule does not apply to `docs/`.

## Rationale
Cycle 1900 (commit 1791f3295) shortened these rows to pass a strict STE lint and deleted facts (audit finding M1). Its ADR-0115 row also seemed to exempt `docs/` from the whole commit gate. The restore takes the text from the parent of commit 1791f3295 and splits it into sentences of 25 words or fewer.

## Changed Areas
- `docs/research-index.md` — restores the counts, names, cycle numbers, refinement ids and rules in 16 rows. Each row describes its document again. The ADR-0115 row names the added-comment rule as the rule that `docs/` is exempt from.
- `.evolve/evals/research-index-ste-restore-adr115.md` — the eval of the TDD phase that sets the score caps; the build did not change it.
- `go/acs/cycle1924/predicates_test.go` — the ACS predicates of the TDD phase. They check the facts, the ADR-0115 text, the lint and the row set. The build did not change them.

## Design Decisions
Each old row becomes a set of short sentences, and no fact moves to a different row. The rejected option was a verbatim restore of the old rows. Those rows have sentences of more than 40 words, and the strict lint refuses them. A second rejected option was to move the detail into the linked documents. The documents already hold the detail, and the index row then no longer tells a reader which document to open.

## Verification
`go test -tags acs -count=1 ./acs/cycle1924/` passes all four predicates. `evolve docs ste-lint --strict` reports 0 findings on `docs/research-index.md`.

## Compatibility
The change is documentation only. No code, gate or test behavior changes. Every row that origin/main links stays in the index.

## Limitations
The restore covers the 16 rows that the predicates name. The other four rows that cycle 1900 rewrote kept their facts, so this build does not change them.
