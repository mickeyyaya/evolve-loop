---
score_cap:
  - criterion: "The three F2 mutants are killed by named test cases"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^(TestCheck_SentenceSplitting|TestCheck_AnEmptyFirstBlockquoteLineDoesNotDecideTheQuotation|TestCheck_EachBlockquoteDecidesItsOwnQuotation)$' ./internal/stelint/"
  - criterion: "A blockquote that opens a fence or an HTML comment and does not close it does not hide the text after the blockquote"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestCheck_ABlockquoteEndClosesItsFenceAndComment$' ./internal/stelint/"
  - criterion: "A 30-word list item followed by a --- line gives one STE-SENTENCE finding"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestCheck_RuleLinesAndSetextUnderlines$' ./internal/stelint/"
  - criterion: "The docs_floor docs and the docs_floor decode agree on the stage words"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1927_004_TheDocsAndTheDecodeAgreeOnTheStageWords$' ./acs/cycle1927"
---

# Eval: STE lint review follow-ups (F2, F3, F4, F7)

> Pins the four items that the round-2 check of the STE lint filed after the S0 landing (inbox `ste-lint-review-followups`, 2026-10-08). F2: three surviving mutants (quotation ending with `?`/`!`, an empty first `>` line, the blockquote state reset). F3: an unclosed fence or HTML comment in a blockquote hid the rest of the file. F4: a `---` line after a list item made the item a setext heading, so it lost the length rule. F7: the decode accepts stage `0`, but the docs named only `off`, `shadow` and `enforce`. Source incident: cycle 1927 lane.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| mutant-kill | F2 mutants killed by named tests | 5/10 | `go test -run TestCheck_SentenceSplitting\|AnEmptyFirst…\|EachBlockquote… ./internal/stelint/` |
| blockquote-exit | fence/comment state cleared at blockquote end | 7/10 | `go test -run TestCheck_ABlockquoteEndClosesItsFenceAndComment ./internal/stelint/` |
| setext-list | list item before `---` keeps its length rule | 7/10 | `go test -run TestCheck_RuleLinesAndSetextUnderlines ./internal/stelint/` |
| stage-words | docs and decode agree | 5/10 | `go test -tags acs -run TestC1927_004 ./acs/cycle1927` |
