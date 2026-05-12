# ADR 0001 — Plugin-Dir Resolution

| Field | Value |
|---|---|
| Status | Accepted |
| Date | 2026-05-12 |
| Cycle | 23 (post-commit hotfix) |
| Affects | Builder profile, Auditor profile, subagent dispatch, Skill tool availability |

## Context

After cycle 22 deployed the `security-review-scored` skill and the `code-review-simplify` allowlist entries, the `EVOLVE_BUILDER_SELF_REVIEW=1` path inside `claude -p` subagents returned "Unknown skill" at runtime. The skill was correctly registered in `plugin.json` and symlinked under `skills/`, but subagents could not discover it.

Investigation (commit `c8ca3d7`, 2026-05-12) identified four variables through a smoke test matrix:

1. `--setting-sources project` — restricts settings to project scope.
2. `--disable-slash-commands` — disables the slash-command surface (falsely suspected as root cause in cycle 23).
3. `--plugin-dir` — points the claude runtime to a plugin manifest directory.
4. Marketplace plugin cache at `~/.claude/plugins/marketplaces/evolve-loop/` — pinned to v9.2.0; `security-review-scored` added post-publish was absent from the cache.

The root cause: `--setting-sources project` without `--plugin-dir` suppresses plugin loading entirely; it does not inherit from the user-level plugin cache. Anthropic Agent SDK documentation: "If you set settingSources explicitly, include 'user' or 'project' to keep skill discovery, or use the `plugins` option to load skills from a specific path."

## Decision

Add `"--plugin-dir", ".evolve/plugin"` to both `builder.json` and `auditor.json` `extra_flags`. The `.evolve/plugin/` directory holds a project-local `plugin.json` manifest and symlinks to `.agents/skills/`, giving subagents a stable, versioned, per-project plugin directory that travels with the repo.

The `.evolve/plugin/` directory was added to `.gitignore` as an exception so it ships with the repository.

## Consequences

**Positive:**
- Skill tool resolves correctly inside `claude -p` subagents launched with `--setting-sources project`.
- Project-local plugin manifest decouples subagent skill discovery from the marketplace cache version.
- Security posture unchanged: `--setting-sources project` still restricts to project settings; `--plugin-dir` is a scoped, explicit grant for one plugin owned by this repo.
- `--disable-slash-commands` remains (defense-in-depth); the ADR 0002 investigation confirms it was not the cause.

**Negative:**
- `.evolve/plugin/` requires maintenance when new skills are added; the symlink structure must be updated.
- The directory is repo-local only; operators who install evolve-loop without the repo content need to recreate it.

## Alternatives considered

| Alternative | Why rejected |
|---|---|
| Remove `--setting-sources project` entirely | Loses project-scope isolation; user-level hooks and MCP config could leak into subagents |
| Pin `--plugin-dir` to `~/.claude/plugins/...` absolute path | Breaks in multi-machine and CI environments; not portable |
| Publish a new marketplace release to update the cache | Works long-term but doesn't fix in-flight subagents; also doesn't solve the `--setting-sources project` isolation requirement |
| Add `"user"` to `--setting-sources` list | Defeats the purpose of project-scoped isolation; leaks user-level config |

## Implementation

- `profiles/builder.json` and `profiles/auditor.json` — `extra_flags` extended with `["--plugin-dir", ".evolve/plugin"]`
- `.evolve/plugin/plugin.json` — project-local manifest; symlinks to `.agents/skills/`
- `.gitignore` — `.evolve/plugin/` exception added
