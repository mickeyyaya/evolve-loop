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
| `docs/research/` | `docs/private/research/` |
| `docs/release/` | (split — see files below) |
| `docs/release-notes/` | `docs/operations/release-notes/` |

### Files

| Old path | New path |
|---|---|
| `knowledge-base/README.md` | `docs/private/README.md` (content rewritten) |
| `docs/research/*.md` | `docs/private/research/*.md` (~42 files, byte-identical) |
| `docs/release/release-protocol.md` | `docs/guides/publishing-releases.md` |
| `docs/release/release-archive.md` | `docs/operations/release-archive.md` |
| `docs/release-notes/index.md` | `docs/operations/release-notes/index.md` |
| `docs/architecture/knowledge-base.md` | `docs/architecture/private-context-policy.md` (content rewritten) |

### Unchanged (kept where they were)

| Path | Reason |
|---|---|
| `docs/research/*.md` (5 files) | Already in the right place. These are agent-accessible research citations; the `docs/research/` files are the agent-excluded archive. |
| `docs/architecture/`, `docs/reference/`, `docs/incidents/`, `docs/reports/` | No path change. |

## How to update your links

If you had a link to `docs/research/agent-economics.md`, change it to
`docs/private/research/agent-economics.md`. Same filename in every case — only the parent path
moved.

Git history is preserved: `git log --follow docs/private/research/agent-economics.md` shows
the full pre-move history under the old path.

## Removal schedule

- **v9.1.x** (this release) — `MOVED.md` introduced; old paths fully gone from the tree
- **v9.2.x or v9.3.x** — `MOVED.md` removed. External 404s thereafter are an accepted cost.

Until then, this file is the canonical reference for "where did X go?"

## 2026-08-05 — documentation-root consolidation (three roots → one)

Open-source best practice: a single `docs/` tree (Diataxis-style). Two extra
roots had accreted; both research trees moved into `docs/research/`:

| Old path | New path |
|---|---|
| `kb/research/*` (4 research packages) | `docs/research/<same-dirname>/` |
| `kb/chronicle/` (engineering chronicle, unshipped at move time) | `docs/chronicle/` |
| `knowledge-base/research/*.md` (12 notes) | `docs/research/` |
| `knowledge-base/cycles/` | **not moved** — runtime write surface (`cmd_loop_outcome.go`); see `knowledge-base/README.md` |

All in-docs links rewritten in the same commit; `kb/` removed entirely.

Second pass (same PR): `knowledge-base/research/` **subdirectories** (10
research packages + `flag-campaign-plan.json`) moved to `docs/research/`; the
three `archived-YYYY-MM-DD/` dirs moved to `docs/private/research/` (the
documented agent-excluded archive tree). Three durable ACS predicates that
pinned old literal paths were repointed (cycle42 cache-ttl, cycle1168 tracker,
cycle1289 researchDoc) — intent preserved, paths updated. The `docdelete`
guard's archive convention was updated in the same commit
(`go/internal/guards/docdelete.go`): mv from a doc root must land under
`docs/`; the archive home is now `docs/private/research/archived-YYYY-MM-DD/`;
`knowledge-base/` is retired as a destination (`cycles/` remains a runtime
write surface).
