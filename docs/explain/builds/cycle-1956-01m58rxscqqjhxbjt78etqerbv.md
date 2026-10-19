# Build Explanation — Cycle 1956

## Build Binding
- Cycle: 1956
- Base SHA: c14bd04e4ec8be3b7e87ba845308d10856081089

## Summary
Parallel branches that archive comment history no longer conflict on the generated index or on a shared page.

## Rationale
The index and the pages are append-only. A union merge keeps the lines of both branches, so one attribute rule replaces a conflict that needed a manual fix on every parallel merge. The existing `go/.apicover-enforce` rule uses the same mechanism. Rejected: a change to `go/internal/commentaudit`, because that package is protected in this lane.

## Changed Areas
- `.gitattributes` — sets `merge=union` on `docs/history/code-comments/*.md`, so the index and the pages merge without conflict.
- `docs/conventions/code-comments.md` — states the merge behavior next to the `history` command description.
- `go/internal/rawgitratchet/rawgitratchet.go` — the scanner no longer counts the string `"init"` that follows `"-m"` as a `git init`; a commit message is not a repo init.
- `go/internal/rawgitratchet/rawgitratchet_test.go` — a table-driven test pins the commit-message case, the raw-init case and the mixed case.
- `go/internal/rawgitratchet/baseline.json` — lowers 16 entries that counted commit messages and removes 2 entries with no init site left. The list only shrinks.

## Design Decisions
The rule matches only the archive directory. Other documents keep the default merge.

## Verification
`go test -tags acs -count=1 ./acs/cycle1956` runs the real `commentaudit history` on two branches in a fixture and merges them with git. All six predicates pass.

## Compatibility
The generator output format is unchanged. The ratchet scanner counts fewer false init sites and flags every real `git init` as before.

## Limitations
When two branches change the same index row, a union merge can keep both old rows. A later `commentaudit history` run rewrites the index from the pages and removes the duplicates.
