# Build Explanation — Cycle 1927

## Build Binding
- Cycle: 1927
- Base SHA: f6aeee9be9ffc625709682d19bd2fca82c4993bb

## Summary
The STE lint now closes a fence or an HTML comment at the end of the blockquote that opened it. A `---` line after a list item is now a rule, so the item keeps its length rule. The docs name the stage word `0`.

## Rationale
An unclosed fence in a blockquote hid the rest of the file from the lint. A list item before `---` became a setext heading, and a heading has no length rule. The decode accepts `0` through `config.GateStage`, so the docs must name it.

## Changed Areas
- `go/internal/stelint/blocks.go` — adds `quoteSkip` and `endQuote`, and limits the setext underline to a paragraph that is not a list item.
- `go/internal/stelint/markdown.go` — adds the `isItem` field that `listItem` sets.
- `go/internal/stelint/check_test.go` — the TDD phase added the tests for the blockquote and quotation rules.
- `go/internal/stelint/edges_test.go` — the TDD phase added three rows for list items before `---`.
- `go/acs/cycle1927/predicates_test.go` — the TDD phase added the four predicates of the cycle.
- `docs/architecture/policy-config.md` — names `0` as a stage word of `docs_floor`.
- `docs/architecture/packages/internal-stelint.md` — records the two new rules, the new tests and the history entry.
- `docs/explain/builds/cycle-1927-01m54kv4vqggbgk5dzw7w3s0b4.md` — this document.

## Design Decisions
The splitter records if the open fence or comment came from a blockquote. A fence outside a blockquote still hides the rest of the file. A flag on the paragraph marks a list item; the kind field alone cannot, because a bullet item has the prose kind. The docs name `0`, and the decode stays unchanged, because other stage dials share `config.GateStage`.

## Verification
`go test -count=1 ./internal/stelint` and `go test -tags acs -count=1 ./acs/cycle1927` pass.

## Compatibility
The public API of the package stays unchanged. The decode of `docs_floor` stays unchanged.

## Limitations
A lazy continuation line after a blockquote still ends the blockquote, as before.
