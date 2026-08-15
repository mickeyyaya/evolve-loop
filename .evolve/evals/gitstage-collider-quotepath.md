---
score_cap:
  - criterion: "detectColliders compares a C-quoted incoming filename as its literal repo-relative path (porcelain stream)"
    max_if_missing: 7
    evidence: "cd go && go test ./internal/phases/ship -run TestDetectColliders_QuotePathDecodesPorcelainEntries -count=1"
  - criterion: "detectColliders decodes C-quoted entries from the git diff --name-only stream too"
    max_if_missing: 6
    evidence: "cd go && go test ./internal/phases/ship -run TestDetectColliders_QuotePathDecodesDiffNameOnly -count=1"
  - criterion: "decoding does not widen the collider set — worktree-only, main-tracked, and deleted entries stay unreported"
    max_if_missing: 8
    evidence: "cd go && go test ./internal/phases/ship -run TestDetectColliders_QuotePathNonCollidersStaySilent -count=1"
  - criterion: "both collider classification reads carry -c core.quotePath=false before the subcommand"
    max_if_missing: 6
    evidence: "cd go && go test ./internal/phases/ship -run TestDetectColliders_QuotePathDisabledOnGitReads -count=1"
---

# Eval: Normalize quoted collider inputs (detectColliders)

> Pins the quote-path contract for `detectColliders` (`go/internal/phases/ship/gitops.go`),
> the shared collider source for both the `shipFromWorktree` ff-merge pre-flight and the
> collider repair ladder (`repair.go`). Cycle-1108 enrolled every other path-classifying
> git reader in this package in the contract — `-c core.quotePath=false` on the read
> (`rawPathRead`) plus `unquoteGitPath` for the residue that flag does not suppress —
> but `detectColliders` was missed. It read `git diff --name-only` with no decoding at
> all and `git status --porcelain` with a naive strip of the wrapping quotes, so a
> non-ASCII, quote-bearing, or backslash-bearing filename compared as escaped text
> (`caf\303\251.txt`) that exists on no disk. The subsequent `os.Stat` failed, the entry
> was skipped, and a REAL untracked main-side collider became invisible to the pre-flight
> and to the repair — the ff-merge then hit the file git refuses to overwrite.
> Source incident: `todo-gitstage-quotepath-collider-rename` + cycle-1466 audit H2;
> authored cycle 1469. Input shapes verified against real git 2.50.1 (2026-08-15).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| porcelain-decode | Quoted porcelain entries (octal non-ASCII, `\"`, `\\`, quoted space) compare as literal paths | 7/10 | `go test -run TestDetectColliders_QuotePathDecodesPorcelainEntries` |
| diff-decode | The `diff --name-only` stream decodes identically (it had no decoding at all) | 6/10 | `go test -run TestDetectColliders_QuotePathDecodesDiffNameOnly` |
| no-false-colliders | Anti-no-op: worktree-only / main-tracked / deleted entries are never reported | 8/10 | `go test -run TestDetectColliders_QuotePathNonCollidersStaySilent` |
| quotepath-argv | Both reads issue `-c core.quotePath=false` before the subcommand | 6/10 | `go test -run TestDetectColliders_QuotePathDisabledOnGitReads` |
