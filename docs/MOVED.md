# Doc Path Migration — v9.1.x

> **Transitional file.** This index documents the old→new path mapping introduced by the
> v9.1.x documentation consolidation. It will be removed in v9.2.x or v9.3.x. External
> bookmarks and blog links that referenced the old paths should be updated.

## Why these moved

evolve-loop previously had **two parallel documentation hierarchies**:

- `docs/` — most documentation
- `knowledge-base/` — research archive structurally excluded from agent context

The v9.1.x doc consolidation unified everything under a **single `docs/` root**. The
agent-context exclusion boundary moved from the top-level `knowledge-base/` folder to a
clearly-named subfolder, `docs/private/`. **No runtime behavior changed** — the same files are
blocked from the same agent context via the same three defense layers; only the path moved.

See [`README.md`](README.md) for the new layout and [`architecture/private-context-policy.md`](architecture/private-context-policy.md)
for the architectural rationale.

## Old → new mapping

### Folders

| Old path | New path |
|---|---|
| `knowledge-base/` | `docs/private/` |
| `knowledge-base/research/` | `docs/private/research/` |
| `docs/release/` | (split — see files below) |
| `docs/release-notes/` | `docs/operations/release-notes/` |

### Files

| Old path | New path |
|---|---|
| `knowledge-base/README.md` | `docs/private/README.md` (content rewritten) |
| `knowledge-base/research/*.md` | `docs/private/research/*.md` (~42 files, byte-identical) |
| `docs/release/release-protocol.md` | `docs/guides/publishing-releases.md` |
| `docs/release/release-archive.md` | `docs/operations/release-archive.md` |
| `docs/release-notes/index.md` | `docs/operations/release-notes/index.md` |
| `docs/architecture/knowledge-base.md` | `docs/architecture/private-context-policy.md` (content rewritten) |

### Unchanged (kept where they were)

| Path | Reason |
|---|---|
| `docs/research/*.md` (5 files) | Already in the right place. These are agent-accessible research citations; the `knowledge-base/research/` files are the agent-excluded archive. |
| `docs/architecture/`, `docs/reference/`, `docs/incidents/`, `docs/reports/` | No path change. |

## How to update your links

If you had a link to `knowledge-base/research/agent-economics.md`, change it to
`docs/private/research/agent-economics.md`. Same filename in every case — only the parent path
moved.

Git history is preserved: `git log --follow docs/private/research/agent-economics.md` shows
the full pre-move history under the old path.

## Removal schedule

- **v9.1.x** (this release) — `MOVED.md` introduced; old paths fully gone from the tree
- **v9.2.x or v9.3.x** — `MOVED.md` removed. External 404s thereafter are an accepted cost.

Until then, this file is the canonical reference for "where did X go?"
