# Comment history: `acs/cycle1274`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1274/predicates_test.go:3` — above `package cycle1274`

```text
// Package cycle1274 materialises the cycle-1274 acceptance criteria for the one
// task triage committed to `## top_n`:
//
//	changeloggen-dedup-duplicate-bullets → `go/internal/changeloggen` must emit
//	each identical rendered bullet at most once within a version section, and
//	the already-shipped verbatim duplicate at CHANGELOG.md:22-23 (both ending
//	`(#406)`) must collapse to a single bullet.
//
// The dropped todo-id (`close-out-cycle1272-fleet-scope-verification`) and the
// two deferred items (`acs-subprocess-timeout-floor`,
// `acs-heading-form-asymmetry-guard`) get ZERO predicates — R9.3 floor-binding:
// predicates bind only to triage-committed work.
//
// Seam choice (why these predicates pin RenderEntry). Both production callers —
// `opscmd.RunChangelogGen` (go/internal/cli/opscmd/changelog.go:86-87) and
// `releasepipeline.runChangelogGenLib` (go/internal/releasepipeline/bridges.go:53-54)
// — run the identical two-call sequence `ClassifyAll` → `RenderEntry`. RenderEntry
// renders exactly one `## [<version>]` section, so "dedup within a version
// section" is precisely a RenderEntry-output property, and a fix there is
// inherited by BOTH paths. 003 and 005 exist so that inheritance is PROVEN per
// path rather than assumed (#373: wired into one path only is the same defect).
//
// Predicate-quality note (cycle-85 ban). Every load-bearing assertion here is
// behavioural: 001/002/005 call the real `changeloggen` functions and assert on
// their returned text; 003 drives the actual exported CLI entry point
// `opscmd.RunChangelogGen` end-to-end over a throwaway git repo and asserts on
// its stdout; 004 asserts on the real emitted artifact (CHANGELOG.md). The one
// source-shaped assertion — 005's `CountInGoFunc` anchor on bridges.go — is a
// PATH-INVENTORY check (does the release path still route through the deduping
// seam), deliberately paired with, never substituted for, the behavioural half.
//
// Flaky-shape rules. No `go test` subprocess, no `./...` sweep, no wall-clock
// bound, no literal PID; every git invocation is `git -C <dir>` so it cannot
// resolve a repo from the lane's cwd. 003's git repo lives under t.TempDir().
```

### `go/acs/cycle1274/predicates_test.go:80` — above `func TestC1274_001_render_entry_dedups_identical_bullets(t *testing.T) {`

```text
// TestC1274_001_render_entry_dedups_identical_bullets is the crux predicate:
// two commits that render an IDENTICAL bullet inside one version section must
// produce ONE bullet, not two — the exact shape that corrupted the shipped
// [22.13.1] section (cycle-1272 audit carry-forward item #1, where an
// origin/main merge made one commit reachable by two paths).
```

### `go/acs/cycle1274/predicates_test.go:221` — above `func TestC1274_004_shipped_changelog_duplicate_collapsed(t *testing.T) {`

```text
// TestC1274_004_shipped_changelog_duplicate_collapsed asserts on the real
// emitted artifact: the already-shipped `## [22.13.1]` section must carry the
// `(#406)` bullet exactly ONCE, while every other line of that section survives
// untouched (edge axis — a fix that deletes the section also "removes" the
// duplicate and must fail here).
```
