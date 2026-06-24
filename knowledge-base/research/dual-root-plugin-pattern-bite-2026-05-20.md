# Dual-Root Plugin Pattern Bite — 2026-05-20

**Status:** Resolved (v10.17.0 marketplace-sync closed the window)
**Severity:** MEDIUM (operator confusion + one cycle of env-var workaround required; no integrity breach)
**Functional impact:** Operator project-repo edits to dispatcher scripts don't reach the running plugin install. Workaround: env-var override.
**Structural impact:** Two parallel "source of truth" file trees coexist; only marketplace-sync reconciles them.

## 1. What happened

During cycle 95-96 of the v10.17.0 batch, the operator edited the watchdog default from 240s→600s via direct `Edit` calls on `~/ai/claude/evolve-loop/scripts/dispatch/phase-watchdog.sh:32` (and sibling files). The edit was committed via `ship.sh --class manual` as `ad07d25` and pushed to origin/main. The next `/evolve-loop` dispatcher invocation, however, showed:

```
[run-cycle] watchdog spawned (pid=92143 pgid=91530 threshold=240s)
[phase-watchdog] started: ... threshold=240s
```

The threshold was STILL 240s despite the project-repo edit. Investigation revealed the dispatcher runs from the plugin install at `~/.claude/plugins/marketplaces/evolve-loop/scripts/dispatch/phase-watchdog.sh`, NOT from the project-repo path. The `find` expression in `SKILL.md`'s STRICT MODE one-liner explicitly looks in `$HOME/.claude/plugins/` — the project-repo copy is never consulted at runtime.

Workaround applied for cycles 96-98: pass `EVOLVE_INACTIVITY_THRESHOLD_S=600 EVOLVE_OBSERVER_STALL_S=600` as env vars on the dispatcher invocation. This bypassed the hardcoded default at the plugin install path.

Closure: v10.17.0 release-pipeline propagated the project-repo changes to the plugin install via marketplace sync (the pipeline polls the marketplace for up to 5 minutes after push, then refreshes `installed_plugins.json`). After v10.17.0 published, subsequent dispatcher invocations correctly showed `threshold=600s` without env-var override.

## 2. Research

### Path topology

Two parallel file trees:

| Tree | Purpose | Path | Editable by operator? |
|---|---|---|---|
| **Project repo** | Source of truth; commits land here; pushed to origin | `~/ai/claude/evolve-loop/` | Yes — direct edits via Edit/Write |
| **Plugin install** | Runtime executable; dispatcher resolves to this | `~/.claude/plugins/marketplaces/evolve-loop/` | No — populated by marketplace sync |

The dispatcher's `find` expression in `skills/evolve-loop/SKILL.md`:
```bash
find $HOME/.claude/plugins \( -path '*/marketplaces/evolve-loop/scripts/dispatch/evolve-loop-dispatch.sh' \
  -o -path '*/cache/evolve-loop/evolve-loop/*/scripts/dispatch/evolve-loop-dispatch.sh' \) \
  -type f 2>/dev/null | sort | tail -1
```

This explicitly finds the marketplace install, NEVER the project-repo copy. The project repo is the source; the plugin install is the runtime.

### Update flow

```
operator Edit/Write          →  project repo file
git commit (via ship.sh)     →  project repo commit
git push origin              →  origin/main on GitHub
release-pipeline.sh X.Y.Z    →  triggers marketplace sync
marketplace polls origin     →  detects new tag
marketplace updates install  →  plugin install file updated
dispatcher runs              →  reads updated plugin install
```

The middle steps (release-pipeline → marketplace polling → install refresh) are the closure path. Without a release, the plugin install lags origin/main indefinitely.

### Pre-existing memory

Memory `feedback_dual_root_pattern.md` captures this: "Plugin-installed kernel scripts must source resolve-roots.sh and split PLUGIN_ROOT (reads) from PROJECT_ROOT (writes); no single REPO_ROOT". The dual-root pattern is a known architectural feature, not a bug — the bite is that operator edits that change runtime behavior (not just persistent project state) need a release to propagate.

## 3. Reasoning

The architecture is correct: separating project source from runtime install lets the plugin run while the project is being edited, and lets marketplace consumers receive vetted releases rather than every commit. The bite is operator-side: when the operator's intent is "fix the running pipeline NOW," a project-repo edit is necessary-but-not-sufficient.

Three failure modes the operator may hit:
1. **Operator edits project, expects runtime to change immediately.** Mitigation: env-var override (worked for cycle 96-98 watchdog).
2. **Operator edits project, releases via release-pipeline, doesn't wait for marketplace polling.** Mitigation: release-pipeline.sh already waits up to 5 min and rolls back on failure.
3. **Operator edits both project and plugin install directly (to skip release).** Anti-pattern — plugin install will be overwritten on next marketplace sync, losing the manual edit.

The watchdog default change in v10.17.0 hit failure mode 1, recovered via env-var override, then closed properly via release-pipeline.

## 4. Fix

### Operator runbook (now documented here)

When making changes to dispatcher scripts that need to take effect IMMEDIATELY for the current dispatcher invocation:

1. **Edit project repo** at `~/ai/claude/evolve-loop/scripts/...`
2. **Commit + push** via `ship.sh --class manual` or `release-pipeline.sh`
3. **For this dispatcher session only**, pass env-var override that matches the new behavior (e.g., `EVOLVE_INACTIVITY_THRESHOLD_S=600`)
4. **For future sessions**, run `release-pipeline.sh X.Y.Z` to propagate to marketplace
5. **Verify post-release**: `bash $(find ~/.claude/plugins -name evolve-loop-dispatch.sh | sort | tail -1) --version` should match the published version

### Long-term (proposed, not yet shipped)

Add a `--watch-project-repo=$PWD` mode to the dispatcher that prefers project-repo files over plugin-install files for development workflows. Risk: bypasses the marketplace-vetted release boundary; should be operator-flag-gated only.

## 5. Lessons

- **`feedback_dual_root_pattern.md`** (memory) — architectural baseline; plugin scripts must source `resolve-roots.sh` and never assume single repo root
- **[[cycle-A-trust-kernel-hardening]]** — kernel hooks ARE installed to the plugin install path, so they enforce against the project repo (the asymmetry is intentional for security)
- **[[watchdog-post-memo-sigterm-pattern-2026-05-20]]** — sibling dossier; the dual-root bite extended the watchdog SIGTERM workaround from "1 env var" to "5 manual recovery dances" before v10.17.0 closed the loop

## 6. References

- Source commit: `ad07d25 perf(watchdog): raise default stall threshold 240s → 600s`
- Release commit: `6505fd3 release: v10.17.0`
- GitHub release: https://github.com/mickeyyaya/evolve-loop/releases/tag/v10.17.0
- Skill file: `skills/evolve-loop/SKILL.md` STRICT MODE section (the `find` expression)
- Memory: `feedback_dual_root_pattern.md`
- Cross-references:
  - [`watchdog-post-memo-sigterm-pattern-2026-05-20.md`](watchdog-post-memo-sigterm-pattern-2026-05-20.md)
  - [`acs-promote-recovery-dance-2026-05-20.md`](acs-promote-recovery-dance-2026-05-20.md)
