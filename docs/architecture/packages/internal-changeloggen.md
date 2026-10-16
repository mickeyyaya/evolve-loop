# internal/changeloggen

## Purpose

`internal/changeloggen` is the Go port of `legacy/scripts/release/changelog-gen.sh`. It is a pure conventional-commits parser. It takes commit subjects (typically from `git log <from>..<to>`). It renders a Keep-a-Changelog block, `## [<version>] - <date>`, in the Added, Fixed, Changed, Documentation and Other buckets.

## Design

- **Idempotent prepend.** When `CHANGELOG.md` already has a `## [<version>]` heading for the target version, the prepend does nothing and manual curation stays (`HasEntry`, `WriteEntry`). `WriteEntry` returns two flags: it wrote, and it skipped as idempotent.
- **Placement.** `PrependEntry` inserts the entry after the file header (everything up to the first `## [` line) and before the first version block. With no version block, it appends. `firstVersionBlock` finds a `## [` only at the start of a line and gives -1 for none.
- **Classification.** `Classify` buckets one subject. The types are case-insensitive.
  - `feat` or `feature`: Added.
  - `fix` or `bugfix`: Fixed.
  - `refactor`, `perf`, `performance`, `stability` or `techdebt`: Changed.
  - `docs`, `documentation` or `doc`: Docs.
  - `chore`, `ci`, `test`, `build`, `style`, `revert`, `meta` or `release`: skipped by convention.
  - Any other typed prefix: Other, with a `(<type>)` tag.
  - No recognized prefix: Other, raw.

  The bash script used two `sed` patterns for the `<type>(<scope>): <rest>` shape. The Go version merges them into one regular expression with named groups.
- **Dedup per bucket.** `appendBucket` collapses identical items into one bullet and keeps first-occurrence order. A merge can make one commit reachable by two paths, which once rendered the same bullet twice (release 22.13.1, both ending `(#406)`). The dedup is per bucket, so the same text classified into two sections still renders once in each.
- **Git access goes through `gitexec`.** `ReadGitLogWith` and `VerifyRefWith` take the executor, and the public `ReadGitLog` and `VerifyRef` inject `gitexec.Default(repoRoot)`. The core is context-first so a test drives it with `fixtures.FakeExec` and needs no real repository. `ReadGitLog` returns `ErrNoCommits` for an empty range, so the caller chooses a placeholder entry or a skip.
- `RenderEntry` puts the from and to refs in the generator byline and takes the date from `now`.
- **The git integration fixture uses a hermetic environment,** so a test never picks up the user's git config or signing keys.

## Invariants

- `IsSemver` accepts `X.Y.Z` with no leading `v`.
- `ResolveChangelogPath` is `<repoRoot>/CHANGELOG.md`; a missing file starts from `DefaultFreshContent`.
