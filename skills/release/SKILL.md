---
name: release
description: Use when the user asks whether a release is ready, or to gate/verify release criteria before publishing. A thin readiness-gate that runs the existing read-only verifiers (evolve release-preflight --require-ci, evolve release-consistency), then delegates execution to /evo:publish. Does NOT reimplement the release pipeline.
argument-hint: "<target-version> [--dry-run]"
---

# /evo:release

> Release **readiness-gate**, not a release executor. It composes the checks that already exist (`evolve release-preflight`, `evolve release-consistency`), then hands a green report to [`/evo:publish`](../publish/SKILL.md), which owns the actual bump → changelog → ship → propagate lifecycle.

## Why this exists

`evolve release-preflight` gates: clean tree, attached branch, semver bump, recent audit PASS and gate-test suites. It also refuses WIP commits since the last tag and a red `required.yml` CI verdict of `HEAD`. With `--require-ci`, an unavailable CI verdict (no run visible, or no `gh`) also fails. `evolve release-consistency` verifies the 6 version markers (plugin.json, marketplace.json, README, CHANGELOG, SKILL headings). This skill runs both and reports one verdict — nothing more.

> Defense-in-depth: [`/evo:publish`](../publish/SKILL.md) now performs the same pre-release CI-green check itself (so a direct `/evo:publish` call is still gated) **and** adds a post-release watch of the released commit's `required CI` and `release` workflows. Running `/evo:release` first stays the recommended path; `evolve release` (the raw binary) is `gh`-free and only prints a "CI not verified" advisory.

## Procedure

Run in order. Any **FAIL** → print the reason and stop (do not delegate to `/evo:publish`).

1. **Preflight** — read-only. Do not pass `--dry-run`: a dry run skips the WIP and CI gates.
   ```bash
   $CLAUDE_PROJECT_DIR/go/bin/evolve release-preflight <target> --require-ci
   ```
   Non-zero exit → stop. The output names the failing check. A WIP failure lists each subject that matches `^(WIP|fixup!|squash!|amend!)`. A CI failure gives the run URL.
2. **Consistency (version markers)**:
   ```bash
   $CLAUDE_PROJECT_DIR/go/bin/evolve release-consistency <target>
   ```
   Non-zero exit → stop.
3. **Main is current** — confirm `git rev-parse HEAD` equals `git rev-parse origin/main`, because the preflight checks the CI run of `HEAD`. A mismatch → stop.
4. **All green → delegate**: invoke `/evo:publish <target>` (pass `--dry-run` through if the user gave it). Report the readiness summary first so the user sees what passed.

## Output

Emit a compact readiness table (✅/❌ per check) before delegating or stopping, e.g.:

| Check | Result |
|---|---|
| preflight (with no-WIP and CI gates) | ✅ |
| version markers | ✅ |
| HEAD is origin/main | ✅ |

→ verdict: **NOT READY** (or **READY → handing off to /evo:publish**).

## When NOT to use this skill

- **To actually publish.** This only gates. `/evo:publish` executes.
- **For ordinary commits.** Use `/evo:commit`.
