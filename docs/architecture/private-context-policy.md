# Private-Context Policy — runtime/reference content separation (v9.1.x+)

> Canonical reference for the two-folder content model that separates
> runtime-visible context from developer-only reference material.

## Why this exists

Cycle 13 (commit `35b31c4`, 2026-05-11) deleted 42 substantive `docs/research/`
files (140-280 LOC each, 7,737 LOC total, 498 KB plugin install reduction) on
the empirically-correct grounds that they were unreferenced by any runtime
agent, skill, or script — and contributed to `Glob` / `Grep` context noise per
[Liu et al. 2023 — Lost in the Middle (arXiv:2307.03172)](https://arxiv.org/abs/2307.03172).
Audit verdict was PASS (0.86 confidence).

The deletion was correct AND it lost a valuable shared knowledge surface. This
document formalizes the structural split that resolves the tension. The split
folder was originally named `knowledge-base/` (top-level) and was relocated to
`docs/private/` in the v9.1.x doc consolidation. Runtime semantics are
unchanged; only the path moved. See [`docs/MOVED.md`](../MOVED.md) for the
old→new mapping.

| Class | Purpose | Loaded into agent context? | Ships in /plugin install? |
|---|---|---|---|
| **Runtime context** (`agents/`, `scripts/`, `skills/`, `docs/architecture/`, `docs/reference/`, `docs/research/` keepers) | Code, contracts, in-cycle reference | YES — agents read these during cycles | YES |
| **Agent-context-excluded reference** (`docs/private/`) | Long-form research, archived design refs | NO — agents never see these during cycles | NO (best-effort) — repo-only, contributor-visible |

> **Naming note:** `docs/private/` is **public-readable on GitHub** — the
> "private" label means "private from the agent's reasoning context", not
> "secret from humans". See [`docs/private/README.md`](../private/README.md).

## The split is structural, not migration

Files do not migrate between the two folders over time. Their reference status
determines which side they sit on:

- A note that ends up being cited by a persona, skill, or script lives in
  `docs/research/` (or a more specific subdir).
- A note that is purely exploratory / archived / cross-reference-only lives in
  `docs/private/research/`.

For new entries, the test is:

> Will any persona, skill, or script reference this doc?

- **Yes** → `docs/research/` (agent-accessible on demand)
- **No**  → `docs/private/research/` (developer-only)

Cross-references count even if the cite isn't actually loaded into every
cycle's context — what matters is whether any runtime artifact *could* read it.

## Architecture of runtime exclusion

Three layers enforce the "agents are blind to `docs/private/`" invariant.
Kernel-first, defense-in-depth, cross-CLI.

### L1 — OS sandbox (primary, cross-CLI)

Each agent profile (`.evolve/profiles/{scout,auditor,orchestrator}.json`)
includes `docs/private` in `sandbox.deny_subpaths`. The per-CLI adapter
(`scripts/cli_adapters/<cli>.sh`) compiles these into OS sandbox rules:

- macOS: `sandbox-exec` profile denies `file-read` and `file-read-data` against
  denied paths
- Linux: `bwrap` denies bind-mounts for denied paths

This is the strongest layer — kernel-enforced, syscall-level. It works
identically across Claude Code, Gemini CLI, and Codex CLI because the sandbox
setup is in the shared adapter codepath.

Builder is exempt-by-construction: it runs in a per-cycle worktree with
`add_dir: ["{worktree_path}"]`, so `docs/private/` (outside the worktree) is
structurally invisible regardless of deny_subpaths.

### L2 — Adapter permission-mode passthrough (Claude Code primary)

Claude Code's permission system honors the same `deny_subpaths` even when the
OS sandbox is disabled (e.g., nested-sandbox EPERM on macOS Darwin 25.4+). The
deny rule fires at the CLI's permission gate before any tool invocation.

### L3 — Layer-B context-builder filter (cross-CLI safety net)

`scripts/lifecycle/role-context-builder.sh:emit_artifact()` early-returns for
any path matching `docs/private/*` / `./docs/private/*` / `*/docs/private/*`.
Even if a future code change attempts to inject a private file into an agent's
prompt context, this filter no-ops the call.

This layer is CLI-agnostic — `role-context-builder.sh` is the canonical prompt
assembler used by Claude Code, Gemini CLI, and Codex CLI adapters.

### Verification

`scripts/tests/private-context-exclusion-test.sh` asserts:

1. Each of scout/auditor/orchestrator profile has `docs/private` in
   `sandbox.deny_subpaths`
2. `role-context-builder.sh:emit_artifact()` has the early-return pattern for
   `docs/private/*`, `./docs/private/*`, `*/docs/private/*`
3. No agent's `add_dir` explicitly lists `docs/private`
4. `.gitattributes` declares `docs/private/ export-ignore`
5. `docs/private/research/` contains the 42 restored files
6. Three spot-checks verify byte-identical match against cycle 13's parent

Runs alongside `swarm-architecture-test.sh` (41 assertions) and
`role-gate-test.sh` (23 assertions) in the trust-kernel regression suite.

## Plugin distribution

`.gitattributes` declares:

```
docs/private/ export-ignore
```

This is the standard Git mechanism for "track in repo, exclude from
`git archive`". However, Claude Code's plugin installer does **not** use
`git archive` — it git-clones the source repo into
`~/.claude/plugins/cache/` and applies a separate hardcoded filter
(documented at `docs/architecture/platform-compatibility.md`).

As of v9.1.x, `docs/private/` may still ship in `/plugin install` for users.
The user impact is bounded:

- Runtime exclusion (L1 + L3) holds even on user installs — agents in
  installed plugins are blocked just as agents in dev clones are.
- The 498 KB install-size cost of cycle 13's deletion is partially reclaimed
  when L1+L3 enforce, but the on-disk footprint returns.

If a future Claude Code version honors `.gitattributes export-ignore` during
plugin install, `docs/private/` will automatically be excluded from user
installs without further changes here.

## Recovery procedure

If `docs/private/research/` is accidentally deleted, restore from cycle 13's
parent commit (the files are byte-identical, only the path moved):

```bash
mkdir -p docs/private/research
git show --name-status 35b31c4 | grep '^D	docs/research/' | cut -f2 | while IFS= read -r f; do
    git show "35b31c4^:$f" > "docs/private/research/$(basename "$f")"
done
```

The restoration is byte-identical — verified by `diff` against `35b31c4^` in
`scripts/tests/private-context-exclusion-test.sh` Test 6.

## Future considerations

- **L1 verification cycle**: re-run a future cycle to empirically confirm that
  the OS sandbox blocks `docs/private/` reads under all three CLIs.
- **Install-size sealing**: if Claude Code adds a `.pluginignore` convention or
  honors `.gitattributes export-ignore`, this folder auto-disappears from user
  installs. Track in a future cycle's retrospective.
- **Per-role exemptions**: a future change may want to allow ONE role (e.g.,
  retrospective synthesizer) to read `docs/private/`. The Layer-B filter would
  gain a role-aware allowlist; profile deny_subpaths would be lifted for that
  role only. Until justified, default is no exemption.

## Related documents

- [`docs/private/README.md`](../private/README.md) — the operator-facing README inside the folder
- [`docs/research-index.md`](../research-index.md) — index of both active and archived research
- [`docs/MOVED.md`](../MOVED.md) — old→new path map for the v9.1.x consolidation
- `agents/evolve-scout.md`, `agents/evolve-auditor.md`, `agents/evolve-orchestrator.md` —
  the persona docs whose profiles include the deny_subpaths entry
- `scripts/lifecycle/role-context-builder.sh` — L3 filter implementation
- `scripts/tests/private-context-exclusion-test.sh` — verification harness
