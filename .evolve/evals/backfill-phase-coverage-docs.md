# Eval: backfill-phase-coverage-docs

## Goal

Verify that the documentation for artifact backfill and self-healing gaps is updated
to reflect the new retro and build-planner backfill coverage added in this cycle.

---

## Acceptance Criteria

### AC-1: artifact-backfill.md Header Map includes retro
The Header Map table in `docs/architecture/artifact-backfill.md` contains a row for
the `retro` phase.

```bash [code]
grep -q 'retro' docs/architecture/artifact-backfill.md
```

### AC-2: artifact-backfill.md Header Map includes build-planner
The Header Map table in `docs/architecture/artifact-backfill.md` contains a row for
the `build-planner` phase.

```bash [code]
grep -q 'build-planner' docs/architecture/artifact-backfill.md
```

### AC-3: self-healing-gaps.md references GAP 14
`docs/architecture/self-healing-gaps.md` contains a reference to the new retro and
build-planner backfill coverage (GAP 14 or equivalent new entry).

```bash [code]
grep -q 'retro\|build-planner\|backfill.*retro\|retro.*backfill' docs/architecture/self-healing-gaps.md
```

### AC-4: self-healing-gaps.md marks deferred/by-design gaps explicitly
Gaps 3, 4, 7, and 8 that were left as "open" are annotated with their deferral
rationale (e.g., "by design", "deferred", or "low priority") so they don't look
like untracked work.

```bash [code]
grep -q 'by design\|deferred\|low priority\|integrity-block\|maxRecoveryDepth\|reviewer reject' docs/architecture/self-healing-gaps.md
```

### AC-5: docs are non-empty and well-formed
The updated docs exist and have meaningful content (not stubs).

```bash [code]
wc -l docs/architecture/artifact-backfill.md | awk '{print $1}' | xargs -I{} sh -c 'test {} -gt 30'
```

---

## Negative / Edge Cases

### NEG-1: No code files modified in this doc-only task
This task only touches markdown files in docs/. No Go source files should be changed
(those belong to the companion task `backfill-phase-coverage`).

```bash [model]
Verify that the diff for this task only modifies files under docs/architecture/.
No .go files should be modified.
```
