# `docs/private/` — Agent-Context-Excluded Research

> **"Private" here means "private from the agent's reasoning context", not "secret from humans".**
> These files are **public-readable on GitHub** and committed to the open-source repo. The "private"
> label is about *runtime exclusion*: no running evolve-loop agent (Scout, Builder, Auditor, etc.)
> can read them via Read/Grep/Glob during a cycle.

## What lives here

Long-form research notes, archived design references, and exploratory material that does not need
to be in an agent's browsable surface during a cycle. Files here are git-tracked so contributors
can read them, but the trust kernel (agent profile `deny_subpaths` + `role-context-builder.sh`
Layer B) blocks every runtime agent from reading them.

## Why a separate folder

Per [Liu et al. 2023 — Lost in the Middle](https://arxiv.org/abs/2307.03172): LLMs degrade on
retrieval when relevant info is surrounded by irrelevant content, even passively (via `Glob`
or `Grep` over the project dir). Cycle 13 deleted 42 `docs/research/` files for exactly this
reason. This folder is the companion structure: same content, kept retrievable for humans, but
explicitly excluded from the agent's runtime browsable surface.

## How runtime exclusion is enforced (cross-CLI)

Three defense-in-depth layers, kernel-first:

| Layer | Mechanism | Scope |
|---|---|---|
| **L1 — OS sandbox** | Agent profile `sandbox.deny_subpaths` includes `docs/private`. The CLI adapter (`scripts/cli_adapters/<cli>.sh`) compiles this into `sandbox-exec` / `bwrap` rules; the OS blocks `read()`/`stat()` syscalls against denied paths. | All CLIs (Claude Code, Gemini CLI, Codex CLI) |
| **L2 — Adapter passthrough** | The same `deny_subpaths` is honored by `claude.sh`'s permission-mode plumbing. Even if the OS sandbox falls back (e.g., nested-sandbox EPERM), Claude Code's own permission system rejects reads against denied paths. | Claude Code primary; CLI-specific |
| **L3 — Layer-B context filter** | `scripts/lifecycle/role-context-builder.sh:emit_artifact()` early-returns for any `docs/private/*` path. Even if some future code accidentally tries to inject a private file into an agent's prompt, this filter no-ops the call. | All CLIs (cross-CLI Layer B) |

## What's where

| Path | Contains |
|---|---|
| `docs/private/research/` | 42 design references (140-280 LOC each, ~7,700 LOC total) restored from cycle 13's parent commit. Topics: agent capabilities, multi-agent patterns, agent economics, autonomous loops, sandboxing, RAG, prompt evolution, and more. |

`docs/research/` (separate sibling folder, NOT this one) holds the 5 *actively referenced* research
files cited by agents/skills/scripts. Those DO load into runtime context on demand. The split is
structural — files don't migrate between the two; their reference status determines which side they
sit on.

## Where to file new entries

When a cycle produces a research note, ask:

> Will any persona, skill, or script reference this doc as a citation that an agent might read?

- **Yes** → `docs/research/` (agent-accessible on demand)
- **No**  → `docs/private/research/` (developer-only; agents structurally blind to it)

Cross-references count even if the `docs/research/` note isn't loaded into every cycle's context —
what matters is whether any runtime artifact *could* read it (persona-included as a citation,
skill-tested by an eval, etc.).

## How a contributor reads these files

```bash
# Just open them directly — they're in your local clone:
ls docs/private/research/

# Browse one:
cat docs/private/research/agent-economics.md

# Grep across the archive:
grep -r "your-topic" docs/private/research/
```

This works because you're a human contributor with full filesystem access. What's blocked is a
*running cycle agent* attempting Read/Grep/Glob against `docs/private/` paths. The OS sandbox
enforces the block at the kernel layer (`sandbox-exec` on macOS, `bwrap` on Linux) regardless of
which CLI (Claude Code, Gemini CLI, Codex CLI) is dispatching.

## Plugin distribution caveat

`.gitattributes` declares `docs/private/ export-ignore`. This is the standard Git mechanism for
"track in repo, exclude from `git archive`". However, **Claude Code's plugin installer does not
use `git archive`** — it git-clones the source repo into `~/.claude/plugins/cache/` and applies
a separate hardcoded filter. As of v9.1.x, `docs/private/` may still ship in `/plugin install`
for users.

The user impact is bounded: shipped files do not load into runtime context (L1 + L3 still hold
even on user installs). Users who clone the repo as plugin contributors see `docs/private/`;
users who only `/plugin install` may also see it depending on Claude Code's installer evolution.
Either way, the runtime exclusion is independent of the install-surface footprint.

If a future Claude Code version honors `.gitattributes export-ignore` during plugin install,
`docs/private/` will automatically be excluded from user installs without further changes here.

## History

- **v9.1.x doc consolidation** — this folder is the new home for what was previously the
  top-level `knowledge-base/` directory. The runtime-exclusion semantics are unchanged; only
  the path moved. See `docs/MOVED.md` for the full old→new mapping.
- **Cycle 13** — 42 files were deleted from `docs/research/` because they bloated agent
  context. The same 42 files were later restored under the (then-named) `knowledge-base/research/`
  folder, which is what now lives at `docs/private/research/`.

## Recovery procedure

If `docs/private/research/` ever gets accidentally deleted, restore from cycle 13's parent commit
(the files are byte-identical between that historical commit and today, only the path moved):

```bash
mkdir -p docs/private/research
git show --name-status 35b31c4 | grep '^D	docs/research/' | cut -f2 | while IFS= read -r f; do
    git show "35b31c4^:$f" > "docs/private/research/$(basename "$f")"
done
```

See also `docs/architecture/private-context-policy.md` for the architectural rationale.
