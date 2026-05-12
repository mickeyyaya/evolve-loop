# ADR 0002 — Disable-Slash-Commands Semantics

| Field | Value |
|---|---|
| Status | Accepted |
| Date | 2026-05-12 |
| Cycle | 23 |
| Affects | Builder profile, defense-in-depth posture, Skill tool allowlist |

## Context

Cycle 23 (`ff26e421`) removed `--disable-slash-commands` from `builder.json:extra_flags` as an attempted fix for the "Unknown skill" error observed in Builder subagents (see ADR 0001 for the actual root cause). The removal was premature: the cycle 23 Skill tool test ran in an interactive Claude Code session context, not in a `claude -p` subprocess — an incompatible test environment.

Without empirical evidence that `--disable-slash-commands` caused the failure, removing it abandoned a defense-in-depth control that had been in place to prevent slash-command abuse inside subagents. Commit `196fc5f` (2026-05-12) reverted the removal while the proper smoke test was planned out-of-band.

The cycle 22 improvements were preserved: `security-review-scored` skill registered in `plugin.json`, symlink added to `skills/`, and the dual-skill allowlist (`code-review-simplify` + `security-review-scored`) retained in `builder.json`.

## Decision

Restore `--disable-slash-commands` to `builder.json:extra_flags`. The flag is a master kill-switch for the slash-command surface; targeted allowlist entries (`security-review-scored`, `code-review-simplify`) are scoped grants on top of a secure default. Targeted grants beat master-off when both satisfy the need — `--disable-slash-commands` plus explicit allowlist entries is the correct defense-in-depth posture.

Core principle codified by this commit: the correct resolution to a permissions puzzle is empirical verification (smoke test matrix), not removal of controls.

## Consequences

**Positive:**
- Defense-in-depth posture restored: slash-command surface disabled by default; skills explicitly allowlisted.
- Cycle 22 improvements (dual-skill allowlist, skill registration) preserved without regression.
- Root cause investigation unblocked; ADR 0001 identifies the real fix (`--plugin-dir`).

**Negative:**
- One cycle (23) shipped without the `--disable-slash-commands` guard. No incident occurred; the Auditor flagged the change as a WARN, which triggered the investigation.

## Alternatives considered

| Alternative | Why rejected |
|---|---|
| Leave `--disable-slash-commands` removed and trust allowlist alone | Expands the subagent's slash-command surface unnecessarily; defense-in-depth principle requires layered controls |
| Replace with per-skill allowlist only (no master kill-switch) | Harder to audit; every new skill requires explicit security review before becoming available inside subagents |
| Diagnose in the same interactive session that cycle 23 used | Incompatible test environment for `claude -p` subprocess behavior; would perpetuate the false conclusion |
