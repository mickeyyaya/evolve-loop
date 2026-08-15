---
score_cap:
  - criterion: "a rename endpoint whose quoted filename contains ' -> ' yields exactly two decoded paths, never fragments"
    max_if_missing: 8
    evidence: "cd go && go test ./internal/phases/ship -run TestPorcelainChangedPaths_QuotedRenameArrowKeepsBothEndpoints -count=1"
  - criterion: "stagedGonePaths takes the quote-aware rename source, so the vanished path is filtered under its literal name"
    max_if_missing: 8
    evidence: "cd go && go test ./internal/phases/ship -run TestStagedGonePaths_QuotedRenameArrowSourceDecodes -count=1"
  - criterion: "malformed quoted rename input degrades to the verbatim token — no panic, no fabricated endpoints"
    max_if_missing: 6
    evidence: "cd go && go test ./internal/phases/ship -run TestPorcelainChangedPaths_RenameArrowMalformedIsSafe -count=1"
  - criterion: "ordinary unquoted renames, copies, and non-rename lines parse byte-identically to pre-change behaviour"
    max_if_missing: 7
    evidence: "cd go && go test ./internal/phases/ship -run TestPorcelainChangedPaths_OrdinaryRenameArrowUnchanged -count=1"
---

# Eval: Parse quoted rename arrows structurally (porcelainChangedPaths / stagedGonePaths)

> Pins the rename-delimiter contract for the two porcelain readers in
> `go/internal/phases/ship/manifest.go`. Both treated `" -> "` as an unconditional
> separator over the whole payload (`strings.Split` in `porcelainChangedPaths`,
> `strings.Cut` in `stagedGonePaths`) — but ` -> ` is a legal byte sequence inside a
> filename, and git QUOTES such a name rather than escaping the spaces. Verified against
> real git 2.50.1 (2026-08-15): `?? "we -> ird.txt"`, and `core.quotePath=false` does
> **not** suppress that quoting. A staged rename of that file prints
> `R  "we -> ird.txt" -> renamed.txt`, which the old readers tore into three
> unbalanced-quote fragments (`"we`, `ird.txt"`, `renamed.txt`). Those fragments are not
> decodable by `unquoteGitPath`, so they flowed verbatim into the `git add -- <paths>`
> pathspec; `git add` exits 128 ("did not match any files") on the first such token and
> fails the ENTIRE add — the exact rc=128 ship-killer `stagedGonePaths` exists to
> prevent, reproduced by the input class it was written for. Inbox reconciliation
> produces renames by construction, so this is a live boundary-ship hazard.
> Source incident: cycle-1466 audit H2 (carried unclosed); authored cycle 1469.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| structural-split | Quoted endpoint holding the delimiter yields exactly 2 decoded paths (source, destination, or both quoted) | 8/10 | `go test -run TestPorcelainChangedPaths_QuotedRenameArrowKeepsBothEndpoints` |
| gone-set-parity | `stagedGonePaths` filters the renamed-away path under its literal name, and never a torn fragment | 8/10 | `go test -run TestStagedGonePaths_QuotedRenameArrowSourceDecodes` |
| malformed-safe | Edge/OOD: unbalanced quote, empty side, undecodable escape degrade verbatim | 6/10 | `go test -run TestPorcelainChangedPaths_RenameArrowMalformedIsSafe` |
| no-regression | Plain renames, copies, and `M` lines are byte-identical after the tokenizer change | 7/10 | `go test -run TestPorcelainChangedPaths_OrdinaryRenameArrowUnchanged` |
