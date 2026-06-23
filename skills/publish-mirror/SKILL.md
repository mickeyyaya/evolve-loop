---
name: publish-mirror
description: Use when the user asks to publish, sync, or refresh the PUBLIC open-source mirror (github.com/mickeyyaya/evolveloop) from the private source-of-truth tree, or invokes /publish-mirror. Wraps the all-Go `evolve publish-mirror` — snapshots the release ref, applies the residual transform (drop tracked binary, remove the chore(build) prefix, optional README swap), runs a deterministic PII/secret sanitizer gate, and with --push commits the orphan snapshot and pushes it by URL + tags. Dry-run by default. NOT the version release (use /publish for `evolve release`).
argument-hint: "[--push] [--tag vX.Y.Z] [--ref REF] [--public-readme PATH] [--remote URL]"
---

# /publish-mirror

> Builds (and optionally pushes) the **public OSS mirror** from the private tree.
> Separate from `/publish` (`evolve release`, which version-bumps + propagates to
> the Claude plugin marketplace). See
> [docs/operations/public-release.md](../../docs/operations/public-release.md) for
> the full process this automates.

## What this skill does

When the user types `/publish-mirror` (optionally `--push --tag vX.Y.Z`), invoke
the go-native command. It runs the release flow deterministically (implementation:
`go/internal/publishmirror/`):

| Step | Operation |
|---|---|
| 1 | Resolve the release ref (default HEAD) |
| 2 | Snapshot the tracked tree into an isolated `--detach` worktree |
| 3 | Residual transforms: remove the `chore(build)` commit-prefix entry; optional `--public-readme` swap |
| 4 | Orphan branch (severs all private history) + drop the tracked `go/evolve` binary |
| 5 | **Deterministic PII/secret sanitizer gate** — macOS-home-path + secret-key patterns + an operator denylist auto-derived from the login name + git email/name. A non-empty violation set is a hard stop (exit 2); nothing is pushed. |
| 6 | **Dry-run by default.** With `--push`: commit + push-by-URL to the mirror `main` + tag + verify. |

## Two invariants (non-negotiable)

1. **Never `git push` private → public.** Every run re-snapshots a clean tree on an
   orphan branch; the private 2,200-commit history (with PII) never travels.
2. **The sanitizer must pass before any push.** Dry-run first; resolve every
   violation before `--push`.

## Invocation

```bash
/publish-mirror                          # dry-run: build + sanitize, never push
/publish-mirror --push --tag v1.2.3      # publish a tagged release to the mirror
/publish-mirror --public-readme PATH     # swap in a condensed public README
/publish-mirror --ref <commit>           # snapshot a specific commit (default HEAD)
/publish-mirror --remote URL             # override the mirror URL (must be a URL/path, never a bare remote name)
```

The slash command translates to (using the tracked binary, matching `/publish`):

```bash
"$CLAUDE_PROJECT_DIR/go/evolve" publish-mirror <args>
```

Exit codes: `0` clean — a dry-run with no violations, **or** a successful push · `1` runtime failure · `2` sanitizer found PII/secret violations (nothing pushed).

## When to use this skill

- **Always start with a dry-run** (no `--push`) to confirm the sanitizer is clean
  and see what would publish.
- Publishing a tagged release to the public mirror after a version release.

## When NOT to use this skill

- **Not the version release.** Use [`/publish`](../publish/SKILL.md) for
  `evolve release X.Y.Z` (version bump + marketplace propagation).
- **Not a non-mirror commit.** Use [`/commit`](../commit/SKILL.md).

## Pre-push checklist

Before the first `--push` to the real mirror, the dry-run must report **zero
violations** (the gate is fail-closed). On a converged tree the remaining
violations are generic `/Users/<placeholder>` paths in test fixtures — including
the sanitizer's own positive-test fixtures, which by design must contain a
`/Users/<name>` to test detection. These are **not** real PII but they DO trip
the gate, so they are an action item: before the first publish, either scrub
those paths or exempt those files (sanitizer allowlist). Re-run the dry-run until
it reports zero.
