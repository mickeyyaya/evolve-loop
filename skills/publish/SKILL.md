---
name: publish
description: Use when the user invokes /publish or asks to release a new version, ship a release, or publish a tag. Wraps the go-native self-healing release pipeline (`evolve release`) — pre-flight checks, auto-changelog, atomic ship, marketplace propagation poll, auto-rollback on failure.
argument-hint: "<target-version> [--dry-run] [--no-rollback] [--skip-tests] [--max-poll-wait-s 300]"
---

# /publish

> Canonical release entry point. Owns the full publish lifecycle: pre-flight → bump → changelog → ship → propagate → rollback-on-failure. NOT a synonym for "git push" — see [docs/release-protocol.md](../../docs/release-protocol.md) for vocabulary.

## What this skill does

When the user types `/publish 18.5.0` (or similar), invoke the go-native release pipeline with the target version. The pipeline owns every step (implementation: `go/internal/releasepipeline/`, journal under `.evolve/release-journal/`):

| Step | Operation | Failure → action |
|---|---|---|
| 1 | Pre-flight gate (`evolve release-preflight`) | exit non-zero; abort, no mutations |
| 2 | Auto-changelog | abort |
| 3 | Version bump (6 markers) | abort |
| 3.5 | Rebuild tracked binary `go/evolve` | abort |
| 4 | Consistency check (`evolve release-consistency`) | abort, files in working tree |
| 5 | Atomic ship | abort, nothing pushed |
| 6 | Marketplace propagation poll (up to 5 min default) | auto-rollback unless `--no-rollback` |
| 7 | Cache refresh | logged WARN; manual fix |

> Steps 1–7 are the **`gh`-free** pipeline binary. It cannot see GitHub CI — it ends by printing a `NOTE: GitHub CI is NOT verified by this pipeline` advisory. CI gating is **this skill's** job (next section); never treat a green pipeline as a green CI.

## CI gating (this skill adds what the binary cannot)

`evolve release` is self-contained and headless-safe, so it does **not** verify GitHub CI. This skill wraps the pipeline with two `gh`-based checks. If `gh` is unavailable, report "cannot verify CI — confirm manually" and stop; never silently skip.

**Before** invoking the pipeline — base CI must be green (catches *"releasing from an already-red `main`"*, the v20.1.0 trigger):

```bash
gh run list --branch main --limit 1 --json headSha,status,conclusion,url
```

Require `headSha == $(git rev-parse origin/main)`, `status == "completed"`, `conclusion == "success"`. Anything else → **STOP** with the run URL:
- *in-progress* → wait for it.
- *failure* → fix `main` green first.
- *stale SHA / local `main` ahead of `origin`* → you'd publish commits CI has never seen; **push `main` and let CI run first**, then release. (This is the same gate [`/release`](../release/SKILL.md) runs; it is hoisted here so `/publish`-direct callers are protected too.)

**After** the pipeline reports success — the *released commit's* CI must go green (catches the version-bump + freshly-pushed commits; this is what actually let v20.1.0 ship red):

```bash
sha=$(git rev-parse origin/main)
for wf in go CI; do
  rid=$(gh run list --commit "$sha" --workflow "$wf" --json databaseId -q '.[0].databaseId')
  gh run watch "$rid" --exit-status || echo "RED: $wf on $sha"
done
```

- **Both green → done.** Report the run URLs.
- **Red → the release is published but its CI is red.** Do **not** auto-rollback a propagated release — **fix forward**: land the CI fix on `main`, then cut the next patch (`/publish <x.y.z+1>`). Report the failing job + `gh run view --log-failed` excerpt.

## Invocation

```bash
/publish 18.5.0                       # full publish, default 5-min poll, auto-rollback on
/publish 18.5.0 --dry-run             # simulate every step, mutate nothing
/publish 18.5.0 --skip-tests          # hot-fix path: skip preflight gate-test execution
/publish 18.5.0 --no-rollback         # post-push failure → exit 3, no auto-revert
/publish 18.5.0 --max-poll-wait-s 600 # tolerate slower marketplace propagation
```

The slash command translates to:

```bash
"$CLAUDE_PROJECT_DIR/go/evolve" release <args>
```

Optional hardening: `--require-preflight` (or `EVOLVE_RELEASE_REQUIRE_PREFLIGHT=1`) runs the full-dry-run harness before any step; `EVOLVE_RELEASE_STRICT_PASS=1` rejects WARN preflight verdicts.

## When to use this skill

- **Always for any version bump** (patch, minor, major). The pipeline guarantees consistency, propagation, and rollback.
- Gate readiness first with [`/release`](../release/SKILL.md) (read-only checks: preflight, consistency, CI-green-on-main, no-WIP-commits) — it delegates here when green.

## When NOT to use this skill

- **Not for non-release commits.** If you're committing a feature without bumping the version, use [`/commit`](../commit/SKILL.md) (gated attestation → `evolve ship --class manual`). The release pipeline assumes a version bump and will fail-fast in preflight if `<target>` equals current.
- **Not as a substitute for testing.** `--skip-tests` is for hot-fix scenarios where CI already verified. Routine releases must run the full preflight test suite.

## Checking what `/publish` would do (dry-run)

Operators new to the pipeline: always start with `--dry-run` to see the proposed changelog block, version bump diff, and lifecycle plan before mutating:

```bash
/publish 18.5.0 --dry-run
```

The pipeline emits each step's proposed output without writing or committing.

## Common failure modes

| Symptom | Likely cause | Fix |
|---|---|---|
| `preflight: target X not greater than current Y` | Version already bumped, OR you typo'd the arg | Run `cat .claude-plugin/plugin.json` and confirm; pick a higher target |
| `preflight: most recent audit-report.md does not declare 'Verdict: PASS'` | Last audit was WARN/FAIL or stale | Run a fresh audit cycle (`evolve loop`) or `evolve subagent run auditor` |
| `marketplace-poll: TIMEOUT` after `git push` | Marketplace checkout didn't pull within deadline | Pipeline auto-rolls-back. Investigate: `git -C ~/.claude/plugins/marketplaces/evolve-loop log --oneline \| head` |
| `SELF_SHA_TAMPERED` on the next ship | Rebuilt binary pinned but not committed in the release | Known structural residue — see runtime-reference.md binary-rebuild procedure; fix tracked in the release-rebuild-binary-not-committed work package |
| Hand-curated CHANGELOG entry overwritten | (Won't happen) | Changelog step is idempotent — if `## [<version>]` exists it skips |

## Vocabulary refresher

If unsure what "publish" means in this project: open [docs/release-protocol.md](../../docs/release-protocol.md). The short version:

- **push** ≠ **publish**. `git push` only moves a remote ref; `publish` runs the full lifecycle.
- **ship** is the per-commit primitive (`evolve ship --class manual|cycle`); `publish`/`release` is the version lifecycle on top of it.

## Implementation note

This skill is a thin wrapper over the go-native pipeline (`evolve release`, `go/internal/releasepipeline/`) plus the `gh`-based CI gating the binary can't do (the pipeline is intentionally `gh`-free so it stays headless/cron-safe). All `evolve release` flag semantics match the underlying command verbatim; the pre/post CI checks above are the skill's own and live only here. (History: through v8.13.x this wrapped `legacy/scripts/release-pipeline.sh`; the bash pipeline was deleted in the go-only consolidation and this skill now fronts its go-native successor.)
