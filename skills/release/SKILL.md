---
name: release
description: Use when the user asks whether a release is ready, or to gate/verify release criteria before publishing. A thin readiness-gate that runs the existing read-only verifiers (evolve release-preflight, evolve release-consistency), adds the CI-green-on-main and no-WIP-commit checks they don't cover, then delegates execution to /publish. Does NOT reimplement the release pipeline.
argument-hint: "<target-version> [--dry-run]"
---

# /release

> Release **readiness-gate**, not a release executor. It composes the checks that already exist (`evolve release-preflight`, `evolve release-consistency`) and adds the two gaps they miss — **CI green on `main` HEAD** and **no WIP/fixup commits** — then hands a green report to [`/publish`](../publish/SKILL.md), which owns the actual bump → changelog → ship → propagate lifecycle.

## Why this exists

`evolve release-preflight` already gates: clean tree, attached branch, semver bump, recent audit PASS, gate-test suites. `evolve release-consistency` already verifies the 6 version markers (plugin.json, marketplace.json, README, CHANGELOG, SKILL headings). Neither checks whether **GitHub CI is actually green on the commit you're about to release from**, nor screens for accidental WIP commits. This skill closes exactly those gaps — nothing more.

> Defense-in-depth: [`/publish`](../publish/SKILL.md) now performs the same pre-release CI-green check itself (so a direct `/publish` call is still gated) **and** adds a post-release watch of the released commit's `go`/`CI` workflows. Running `/release` first stays the recommended path; `evolve release` (the raw binary) is `gh`-free and only prints a "CI not verified" advisory.

## Procedure

Run in order. Any **FAIL** → print the reason and stop (do not delegate to `/publish`).

1. **Preflight (5 gates)** — read-only:
   ```bash
   $CLAUDE_PROJECT_DIR/go/bin/evolve release-preflight <target> --dry-run
   ```
   Non-zero exit → stop.
2. **Consistency (version markers)**:
   ```bash
   $CLAUDE_PROJECT_DIR/go/bin/evolve release-consistency <target>
   ```
   Non-zero exit → stop.
3. **Gap 1 — CI green on `main` HEAD** (requires `gh`; if absent, report "cannot verify CI" and stop):
   ```bash
   gh run list --branch main --limit 1 --json headSha,status,conclusion,url
   ```
   Confirm `headSha` matches `git rev-parse origin/main`, `status == "completed"`, `conclusion == "success"`. Anything else (in-progress, failure, stale SHA) → stop with the run URL.
4. **Gap 2 — no WIP/fixup commits since the last tag**:
   ```bash
   git log "$(git describe --tags --abbrev=0)..HEAD" --format=%s
   ```
   If any subject matches `^(WIP|fixup!|squash!|amend!)` → stop and list them.
5. **All green → delegate**: invoke `/publish <target>` (pass `--dry-run` through if the user gave it). Report the readiness summary first so the user sees what passed.

## Output

Emit a compact readiness table (✅/❌ per check) before delegating or stopping, e.g.:

| Check | Result |
|---|---|
| preflight (5 gates) | ✅ |
| version markers | ✅ |
| CI green on main HEAD | ❌ run failed (url) |
| no WIP/fixup commits | ✅ |

→ verdict: **NOT READY** (or **READY → handing off to /publish**).

## When NOT to use this skill

- **To actually publish.** This only gates. `/publish` executes.
- **For ordinary commits.** Use `/commit`.
