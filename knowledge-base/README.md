# Knowledge Base

> Developer-only reference content. **Not loaded into agent runtime context.**

## What lives here

Long-form research notes, archived design references, and exploratory
material that does not need to be in an agent's browsable surface during
a cycle. Files here are git-tracked so contributors can read them, but
the trust kernel (agent profile `deny_subpaths` + `role-context-builder.sh`
Layer B) blocks every runtime agent from reading them.

## Why a separate folder

Per [Liu et al. 2023 — Lost in the Middle](https://arxiv.org/abs/2307.03172):
LLMs degrade on retrieval when relevant info is surrounded by irrelevant
content, even passively (via `Glob` / `Grep` over the project dir). Cycle 13
deleted 42 docs/research/ files for exactly this reason. This folder is the
companion structure: same content, kept retrievable, but explicitly excluded
from the runtime browsable surface.

## What's where

| Path | Contains |
|---|---|
| `knowledge-base/research/` | 42 design references (140-280 LOC each, 7,737 LOC total) restored from commit `35b31c4^`. Topics: agent capabilities, multi-agent patterns, agent economics, autonomous loops, sandboxing, RAG, prompt evolution, and more. |

`docs/research/` (separate, sibling-but-distinct) holds the 5 *actively
referenced* research files (cited by agents/skills/scripts). Those DO load
into runtime context. The split is structural, not migration — files don't
move between the two; their reference status determines which side they
sit on.

## Where to file new entries

When a cycle produces a research note, ask:

> Will any persona, skill, or script reference this doc?

- **Yes** → `docs/research/` (runtime-visible; agents read it during cycles)
- **No**  → `knowledge-base/research/` (developer-only; agents blind to it)

Cross-references count even if the docs/research/ note isn't loaded into
every cycle's context — what matters is whether any runtime artifact
*could* read it (e.g., persona-included as a citation, skill-tested by an
eval, etc.).

## How a contributor reads these files

```bash
# Just open them directly — they're in your local clone:
ls knowledge-base/research/

# Browse one:
cat knowledge-base/research/agent-economics.md

# Grep across the archive:
grep -r "your-topic" knowledge-base/research/
```

This works because you're a human contributor with full filesystem access.
What's blocked: a *running cycle agent* attempting Read/Grep/Glob against
knowledge-base/ paths. The OS sandbox enforces the block at the kernel
layer (sandbox-exec on macOS, bwrap on Linux) regardless of which CLI
(Claude Code, Gemini CLI, Codex CLI) is dispatching.

## How runtime exclusion is enforced (cross-CLI)

Three layers, kernel-first:

| Layer | Mechanism | Scope |
|---|---|---|
| **L1 — OS sandbox** | Agent profile `sandbox.deny_subpaths` includes `knowledge-base`. The CLI adapter (`scripts/cli_adapters/<cli>.sh`) compiles this into `sandbox-exec` / `bwrap` rules; the OS blocks `read()`/`stat()` syscalls against denied paths. | All CLIs (Claude Code, Gemini CLI, Codex CLI) |
| **L2 — Adapter passthrough** | The same `deny_subpaths` is honored by `claude.sh`'s permission-mode plumbing. Even if the OS sandbox fails (e.g., nested-sandbox EPERM), Claude Code's own permission system rejects writes to denied paths. | Claude Code primary; CLI-specific |
| **L3 — Layer-B context filter** | `scripts/lifecycle/role-context-builder.sh:emit_artifact()` early-returns for any `knowledge-base/*` path. Even if some future code accidentally tries to inject a knowledge-base file into an agent's prompt, this filter no-ops the call. | All CLIs (cross-CLI Layer B) |

Verified by `scripts/tests/knowledge-base-exclusion-test.sh`.

## Plugin distribution caveat

`.gitattributes` declares `knowledge-base/ export-ignore`. This is the
standard Git mechanism for "track in repo, exclude from `git archive`".
However, **Claude Code's plugin installer does not use `git archive`** —
it git-clones the source repo into `~/.claude/plugins/cache/` and applies
a separate hardcoded filter. As of v9.1.0, `knowledge-base/` may still ship
in `/plugin install` for users.

The user impact is bounded: shipped files do not load into runtime context
(L1 + L3 still hold even on user installs). Users who clone the repo as
plugin contributors see knowledge-base/; users who only `/plugin install`
may also see it depending on Claude Code's installer evolution. Either way,
the runtime exclusion is independent of the install-surface footprint.

If a future Claude Code version honors `.gitattributes export-ignore`
during plugin install, `knowledge-base/` will automatically be excluded
from user installs without further changes here.

## Recovery procedure

If `knowledge-base/research/` gets accidentally deleted, restore from cycle
13's parent commit:

```bash
mkdir -p knowledge-base/research
git show --name-status 35b31c4 | grep '^D	' | cut -f2 | while IFS= read -r f; do
    git show "35b31c4^:$f" > "knowledge-base/research/$(basename "$f")"
done
```

Cycle 13's commit deleted these files from `docs/research/`; cycle 13^
still has them. The restoration is byte-identical.
