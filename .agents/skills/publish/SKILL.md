---
name: publish
description: Use when the user invokes /publish or asks to release a new version, ship a release, or publish a tag. Wraps the v8.13.2 self-healing release pipeline (scripts/release-pipeline.sh) — pre-flight checks, auto-changelog, atomic ship, marketplace propagation poll, auto-rollback on failure.
argument-hint: "<target-version> [--dry-run] [--no-rollback] [--skip-tests] [--max-poll-wait-s 300]"
---

# /publish

> Canonical release entry point. Owns the full publish lifecycle: pre-flight → bump → changelog → ship → propagate → rollback-on-failure. NOT a synonym for "git push" — see [docs/release-protocol.md](../../docs/release-protocol.md) for vocabulary.

## What this skill does

When the user types `/publish 8.13.4` (or similar), invoke `scripts/release-pipeline.sh` with the target version. The pipeline owns every step:

| Step | Operation | Failure → action |
|---|---|---|
| 1 | Pre-flight gate (`scripts/release/preflight.sh`) | exit 1; abort, no mutations |
| 2 | Auto-changelog (`scripts/release/changelog-gen.sh`) | exit 1; abort |
| 3 | Version bump (`scripts/release/version-bump.sh`) | exit 1; abort |
| 4 | Consistency check (`scripts/utility/release.sh`) | exit 1; abort, files in working tree |
| 5 | Atomic ship (`scripts/lifecycle/ship.sh`) | exit 2; abort, nothing pushed |
| 6 | Marketplace propagation poll (up to 5 min default) | exit 3; auto-rollback unless `--no-rollback` |
| 7 | Cache refresh (re-run release.sh) | logged WARN; manual fix |

## Invocation

```bash
/publish 8.13.4                       # full publish, default 5-min poll, auto-rollback on
/publish 8.13.4 --dry-run             # simulate every step, mutate nothing
/publish 8.13.4 --skip-tests          # hot-fix path: skip preflight gate-test execution
/publish 8.13.4 --no-rollback         # post-push failure → exit 3, no auto-revert
/publish 8.13.4 --max-poll-wait-s 600 # tolerate slower marketplace propagation
```

The slash command translates to:

```bash
bash scripts/release-pipeline.sh <args>
```

## When to use this skill

- **Always for any version bump** (patch, minor, major). The pipeline guarantees consistency, propagation, and rollback.
- **Before manual `bash scripts/lifecycle/ship.sh`** — only fall back to `ship.sh` directly for non-version commits (the orchestrator's audit-PASS branch for non-release cycles).

## When NOT to use this skill

- **Not for non-release commits.** If you're committing a feature without bumping the version, use `bash scripts/lifecycle/ship.sh "<msg>"` directly. The pipeline assumes a version bump and will fail-fast in preflight if `<target>` equals current.
- **Not as a substitute for testing.** `--skip-tests` is for hot-fix scenarios where CI already verified. Routine releases must run the full preflight test suite.

## Checking what `/publish` would do (dry-run)

Operators new to the pipeline: always start with `--dry-run` to see the proposed changelog block, version bump diff, and lifecycle plan before mutating:

```bash
/publish 8.13.4 --dry-run
```

The pipeline emits each step's proposed output without writing or committing.

## Common failure modes

| Symptom | Likely cause | Fix |
|---|---|---|
| `preflight: target X not greater than current Y` | Version already bumped, OR you typo'd the arg | Run `cat .claude-plugin/plugin.json` and confirm; pick a higher target |
| `preflight: most recent audit-report.md does not declare 'Verdict: PASS'` | Last audit was WARN/FAIL or stale | Run a fresh audit: `bash scripts/dispatch/subagent-run.sh auditor <cycle> <workspace>` |
| `marketplace-poll: TIMEOUT` after `git push` | Marketplace checkout didn't pull within deadline | Pipeline auto-rolls-back. Investigate: `git -C ~/.claude/plugins/marketplaces/evolve-loop log --oneline | head` |
| Hand-curated CHANGELOG entry overwritten | (Won't happen) | `changelog-gen.sh` is idempotent — if `## [<version>]` exists it skips |

## Vocabulary refresher

If unsure what "publish" means in this project: open [docs/release-protocol.md](../../docs/release-protocol.md). The short version:

- **push** ≠ **publish**. `git push` only moves a remote ref; `publish` runs the full lifecycle.
- **ship** is an informal alias retained for backwards compatibility. New work uses `publish`.

## Implementation note

This skill is a thin documentation/discoverability wrapper. The actual orchestration lives in `scripts/release-pipeline.sh`; this skill exists so operators can type `/publish` instead of remembering the script path. All flag semantics match the underlying script verbatim.
