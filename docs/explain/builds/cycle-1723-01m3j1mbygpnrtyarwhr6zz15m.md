# Build Explanation — Cycle 1723

## Build Binding
- Cycle: 1723
- Base SHA: c8bc27bbe07a5b12fb43608fa3c7e569f1a4eba1

## Summary
`commentaudit comments -base <ref> [dir ...]` lists every whole-line comment that a diff adds, as `path: text`. It leaves out machine-read directives and the package doc of a file that is new at base. It does not count a comment moved within its file, and it exits 1 when it lists any line. It gives the zero-comment rule for new code a deterministic count, so reviewers no longer have to count by reading the diff.

## Rationale
The package already had the two pieces this needs. The `directive` regex behind `Directives` knows the machine-read comments, and `AddedNarrative` already did a move-aware multiset diff. The smallest change generalizes that diff: `AddedNarrative` and the new `AddedComments` share `subtract` and `commentLines`, and differ only in which lines they keep. `check` and `comments` also shared their whole CLI shape, so both now run through one `listAdded` function. A second copy of that function was the rejected alternative.

## Changed Areas
- `go/internal/commentaudit/stats.go` — adds `AddedComments` and the `packageDoc` helper. `AddedNarrative` now uses the shared `subtract`/`commentLines` multiset diff, so the two cannot drift apart on the moved-line rule.
- `go/internal/commentaudit/cli.go` — adds the `comments` dispatch case and folds `check` into the shared `listAdded`. The usage line names `comments`.
- `go/internal/commentaudit/stats_test.go` — `TestAddedComments` is table-driven. It covers plain, block, directive, moved, second-copy, removed, new-file package doc, existing-file package doc and unparseable new-file cases.
- `go/internal/commentaudit/cli_test.go` — `TestMain_Comments` drives `Main` through the listing, the all-clear summary and the usage error.
- `docs/conventions/code-comments.md` — the Enforcement section documents the `comments` subcommand with its landed flags.
- `go/acs/cycle1723/predicates_test.go` — TDD-phase acceptance predicates that build and run the real binary against temp git repos. The builder did not modify them.
- `.evolve/evals/commentaudit-reports-every-added-comment.md` — TDD-phase eval with score caps for this task. The builder did not modify it.
- `docs/explain/builds/cycle-1723-01m3j1mbygpnrtyarwhr6zz15m.md` — this explanation document.

## Design Decisions
- The package-doc exemption applies only when the base side is absent (`before == nil`). The doc lines are added to the held multiset, so they cancel one matching added line each. No line-number bookkeeping is needed. The whole doc group is spared because the convention allows a one-to-three-line package doc.
- `packageDoc` parses with `parser.PackageClauseOnly`, so a syntax error later in the file does not cost the exemption. When even the package clause fails to parse, the file has no package doc and every comment is listed. That errs toward over-reporting, never toward hiding a comment.
- Directives are filtered by the same `directive` regex that `Directives` uses, on both sides of the diff. The two cannot diverge.

## Verification
- `go test -count=1 -tags acs ./acs/cycle1723/` — 8/8 PASS against the real binary.
- `go test -count=1 ./internal/commentaudit/... ./cmd/commentaudit/...` — PASS. `go vet` and `gofmt -l` are clean.
- `evolve acs suite --cycle 1723` — green=174 red=0 skip=54 total=228.
- Dogfood: `go run ./cmd/commentaudit comments -base HEAD internal/commentaudit` lists exactly the two comments this change adds.

## Compatibility
`check`, `verify` and `rank` keep their behavior and output. `TestC1723_007` and the existing `TestMain_Check` pin `check`. The usage string gains `comments`. `AddedNarrative` keeps its signature.

## Limitations
Trailing comments on code lines are not counted, matching `check`'s whole-line rule. Scout Task 2 is out of this lane: citing the command from the architecture reviewer persona and the auditor's craft audit. Triage dropped it as a protected surface, so inbox acceptance[1] is only partly served, by the convention-doc bullet.
