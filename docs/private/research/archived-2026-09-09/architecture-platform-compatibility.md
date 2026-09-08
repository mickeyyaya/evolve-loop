# Platform Compatibility (v8.51.0+)

> Which AI coding CLIs can run evolve-loop, and at what **capability tier**.

## Capability model (v8.51.0+)

evolve-loop's pipeline is **CLI-independent**. The cycle state machine, kernel gates (role-gate, ship-gate, phase-gate-precondition, ledger SHA chain), and ship logic are pure shell + jq — they fire identically regardless of which CLI invokes the pipeline.

What varies per-CLI is the **adapter layer**: how subagent prompts are dispatched, whether subprocess isolation is available, whether budget caps can be enforced natively, etc. Each adapter ships a **capability manifest** (`legacy/scripts/cli_adapters/<cli>.capabilities.json`) declaring which guarantees it can structurally provide.

Pipeline behavior is deterministic per-tier:
- **`full`** — adapter provides all expected guarantees natively (e.g., Claude Code).
- **`hybrid`** — adapter delegates to a more-capable runtime (e.g., Gemini → Claude binary).
- **`degraded`** — adapter runs in same-session mode; reduced isolation but pipeline completes.
- **`none`** — adapter cannot provide the capability; relies entirely on pipeline-level structural defenses.

**Critical safety invariant**: missing capabilities **never block the pipeline**. They only lower the quality (more warnings, less subprocess isolation, weaker forgery defenses) and surface as `quality_tier` annotations in ledger entries.

## Quick answer

| CLI | Default tier | With claude on PATH | Without claude | Notes |
|---|---|---|---|---|
| **Claude Code** | `full` | `full` | n/a (native) | Reference runtime. `claude -p` per subagent + profile permissions + sandbox-exec/bwrap + native budget cap. |
| **Gemini CLI** | depends on env | `hybrid` (full caps via delegation) | `degraded` (same-session) | v8.51.0+: graceful degradation. Pre-v8.51 exit-99 behavior preserved via `--require-full` opt-in. |
| **Codex CLI** | depends on env | `hybrid` (full caps via delegation) | `degraded` (same-session) | v8.51.0+: hybrid like Gemini. Pre-v8.51 was tier-3 stub. |
| **Antigravity CLI (agy)** | depends on env | `hybrid` (full caps via delegation) | `degraded` (same-session) | v10.19.0+: NATIVE mode (`agy -p`) available when agy binary on PATH; cost_blind:true (rollout cycle must add billing tap). |
| **Copilot / others** | `none` | (no adapter) | (no adapter) | Skill content surface portable; runtime adapter unimplemented. |

Run `./bin/check-caps <adapter>` (or just `./bin/check-caps` to auto-detect) to see your environment's resolved capability tier before running cycles.

## The capability dimensions

Each adapter declares values for these five capabilities:

| Capability | Full | Hybrid | Degraded | None | Quality impact when missing |
|---|---|---|---|---|---|
| `subprocess_isolation` | `claude -p` per subagent + profile | inherited via claude.sh delegation | same-session execution | n/a | Builder + Auditor share session memory; less isolation between phases |
| `budget_cap` | native flag (`--max-budget-usd`) | inherited | none | none | Runaway cycles can exceed cost budget; mitigation: `EVOLVE_RUN_TIMEOUT` external bound |
| `sandbox` | sandbox-exec / bwrap | inherited | none | none | Adapter writes are not OS-isolated; mitigation: kernel hooks fire on bash commands regardless |
| `profile_permissions` | `--allowedTools` / `--disallowedTools` | inherited | none | n/a | Subagents can call any tool the calling LLM has access to; mitigation: anti-forgery prompt inoculation in SKILL.md, post-hoc artifact verification |
| `challenge_token` | embedded in profile prompt | inherited | post-hoc artifact verification | n/a | Forgery slightly harder to detect early; mitigation: artifact content checks (v7.9.0+ defenses) |

The kernel hooks (`role-gate`, `ship-gate`, `phase-gate-precondition`, ledger SHA chain) fire on **bash commands**, not on adapter dispatch. They protect the pipeline regardless of adapter mode. The Gemini Forgery defenses (v7.9.0: artifact content checks, git diff substance gate, state.json checksum, .sh write protection, anti-forgery prompt) ALSO operate at the pipeline layer. Together they mean a degraded adapter cannot bypass structural safety; it can only operate with reduced isolation.

## Per-CLI installation

### Claude Code (full caps)

```bash
# Install evolve-loop as a Claude plugin (see README.md for marketplace URL)
claude --version  # 1.0.0+ recommended
./bin/check-caps claude  # → quality_tier: full
```

### Gemini CLI (hybrid or degraded)

```bash
gemini --version
# Optional: install Claude CLI for hybrid mode (full caps)
claude --version

# Verify resolved tier
./bin/check-caps gemini

# Hybrid (claude on PATH):    quality_tier: hybrid
# Degraded (no claude):       quality_tier: degraded or none
```

To enforce hybrid-only and exit-99 if claude is missing:
```bash
EVOLVE_GEMINI_REQUIRE_FULL=1 bash legacy/scripts/cli_adapters/gemini.sh
# or pass --require-full
```

### Codex CLI (hybrid or degraded — v8.51.0+)

```bash
codex --version
# Optional: install Claude CLI for hybrid mode
claude --version

./bin/check-caps codex
# Same hybrid/degraded resolution as Gemini.
```

To enforce hybrid-only:
```bash
EVOLVE_CODEX_REQUIRE_FULL=1 bash legacy/scripts/cli_adapters/codex.sh
```

### Antigravity CLI / agy (native or hybrid or degraded — v10.19.0+)

```bash
agy --version
# Optional: install Claude CLI for hybrid mode (full caps)
claude --version

./bin/check-caps antigravity
# NATIVE (agy on PATH):          quality_tier: hybrid (agy subprocess, cost_blind)
# HYBRID (claude on PATH):       quality_tier: hybrid (full caps via claude.sh)
# DEGRADED (neither):            quality_tier: degraded (same-session)
```

To enforce NATIVE/HYBRID-only (exit-99 if neither binary found):
```bash
EVOLVE_AGY_REQUIRE_FULL=1 bash legacy/scripts/cli_adapters/agy.sh
# or pass --require-full
```

Note: in NATIVE mode, cost is reported as `cost_blind:true` (zero attribution). A rollout cycle must add a billing tap or turn-cap before cost tracking is accurate. See [reference/agy-runtime.md](../../skills/loop/reference/agy-runtime.md).

### Other CLIs (`none` — skill content only)

You can read SKILL.md and the phase docs from any CLI. To run cycles, implement an adapter at `legacy/scripts/cli_adapters/<your-cli>.sh` mirroring `gemini.sh`'s pattern + ship a `<your-cli>.capabilities.json` manifest. See [Adapter contract](#adapter-contract) below.

### Native plugin install — Claude Code, Codex, agy (ADR-0066)

evo installs as a **native plugin** on all three plugin-host CLIs from the same canonical repo. The
manifests are projected from `.claude-plugin/plugin.json` by `skillcheck` (single source; `evolve skills
generate` writes them, `evolve skills check` gates drift) and carry **only** fields each host's schema
accepts — vendor metadata lives in evo-owned files (`.goreleaser.yml`, this doc), never in platform manifest
keys. See [ADR-0066](adr/0066-cross-cli-plugin-install-and-manifest-schema-conformance.md).

| CLI (current) | Install | Surface |
|---|---|---|
| Claude Code 2.1.195 | `/plugin marketplace add mickeyyaya/evolve-loop` → `/plugin install evo@evo` | 23 skills + 23 `/evo:<name>` commands; `.claude-plugin/{plugin,marketplace}.json` |
| Codex 0.142.2 | `codex plugin marketplace add mickeyyaya/evolve-loop` → `codex plugin add evo@evo` | 23 skills under `~/.codex/plugins/cache/evo/evo/<ver>/`; `.codex-plugin/plugin.json` + `.agents/plugins/marketplace.json` (`source.path:"."`) |
| Antigravity (agy) 1.0.13 | `evolve skills publish --target agy --install` → `agy plugin install <bundle>` | whole skill trees incl. `reference/` overlays; prunes the pre-rename `evolve-loop` plugin |

> **Manifest-schema gotcha (ADR-0066):** platform manifests must hold only schema-recognized keys. CC 2.1.195
> rejecting evo's `binaries` (a documentation array; CC types `binaries` as a record) + custom `compatibility`
> caused a hard `/plugin install` failure (surfaced misleadingly as *"source type … not supported"*). The
> `acs/regression/pluginschema` gate now pins this; the codex version stays in sync via the
> `versionbump.Paths.Files()` SSOT + `releaseconsistency` codex marker.

### Skill publishing to other CLIs (`evolve skills publish` — ADR-0041)

Independent of native-plugin install and *runtime* tiers above, canonical skills are also **projected** to
foreign skill surfaces via `evolve skills publish` (single-source projection; see
[ADR-0041](adr/0041-cross-cli-skill-publishing.md)):

| Target | Surface | What lands there |
|---|---|---|
| Codex CLI | `$CODEX_HOME/skills/evolve-<name>/` — **⚠️ superseded** by the native marketplace above (codex 0.142.2 no longer discovers loose skills); usable for the open-standard `--codex-home ~/.agents` surface | `evolve-` prefixed |
| Antigravity (agy) | `agy plugin install` → `~/.gemini/config/plugins/evo/` | all skills (whole trees), unprefixed under the `evo` plugin |
| Ollama | `ollama create evolve-<name>` Modelfiles | read-only reasoning/review subset only (no tool use in `ollama run`) |

```bash
evolve skills publish                 # stage-only: gitignored mirrors under .evolve/publish/ + agy validate
evolve skills publish --install       # mutate: agy plugin install, ollama create (+ codex copy)
evolve skills publish --check         # CI drift gate against the staged mirrors (exit 2 on drift)
```

In-repo Codex discovery via the `.agents/skills/` symlink layer is unaffected.

## Adapter contract

Every adapter at `legacy/scripts/cli_adapters/<cli>.sh` is invoked by `subagent-run.sh` with these env vars:

| Var | Purpose |
|---|---|
| `PROFILE_PATH` | Absolute path to agent profile JSON |
| `RESOLVED_MODEL` | Resolved tier (haiku/sonnet/opus) |
| `PROMPT_FILE` | Path to prompt file |
| `CYCLE` | Cycle number (integer) |
| `WORKSPACE_PATH` | `.evolve/runs/cycle-N/` |
| `WORKTREE_PATH` | Optional builder isolation path |
| `STDOUT_LOG` | Where to redirect stdout |
| `STDERR_LOG` | Where to redirect stderr |
| `ARTIFACT_PATH` | Resolved output artifact path |
| `VALIDATE_ONLY` | If set, print the command and exit without invoking the LLM |

The adapter must:
1. Build the underlying CLI's invocation from profile fields (or delegate to claude.sh in HYBRID mode).
2. Stream stdout to `STDOUT_LOG`, stderr to `STDERR_LOG`.
3. Write the agent's report to `ARTIFACT_PATH` (or rely on the calling LLM to write it in DEGRADED mode).
4. Exit 0 on success; non-zero only on adapter-level failures (the pipeline distinguishes adapter exit codes from artifact-verification failures).
5. Ship a `<cli>.capabilities.json` manifest declaring resolved capabilities.

See `legacy/scripts/cli_adapters/claude.sh` for the canonical full-caps reference, `gemini.sh` for the hybrid+degraded pattern.

## Multi-LLM-per-phase (v8.52.0 roadmap)

Each phase profile (`scout.json`, `builder.json`, `auditor.json`, `intent.json`, `retrospective.json`) declares its own `cli` field. v8.51.0's adapter resolution reads `profile.cli` as authoritative (replacing session-wide CLI detection). v8.52.0 will expose this as an operator-facing UX: e.g., Scout=Claude (broad codebase scan), Builder=Codex (focused implementation), Auditor=Gemini (independent perspective). Per-phase capability tiers will compose at the cycle level; the ledger will record `quality_tier` per phase entry.

## Detection

The skill auto-detects which CLI it's running under via `legacy/scripts/dispatch/detect-cli.sh`:

1. `CLAUDE_CODE_INTERACTIVE` set → `claude`
2. `GEMINI_CLI` or `GEMINI_API_KEY` set → `gemini`
3. `CODEX_*` env vars → `codex`
4. `agy` binary on PATH → `antigravity`
5. Otherwise → `unknown`

Override with `EVOLVE_PLATFORM=<cli>`.

## Why graceful degradation (v8.51.0)

Pre-v8.51, Gemini CLI hit `exit 99` if `claude` binary was missing — pipeline blocked. Post-v8.51, the same scenario resolves to `quality_tier: degraded` and the pipeline runs with reduced isolation. The structural defenses (kernel hooks + Gemini Forgery v7.9.0 mitigations) make degraded mode safe to operate, even if less robust than full hybrid mode.

This shift follows the user's directive: *"the process/pipeline should function regardless of which CLI is used. Missing features should only lower the quality (e.g., less secure), not block the pipeline."*

Operators who require strict hybrid for production (budget caps, subprocess isolation) opt back into the pre-v8.51 hard-fail with `--require-full` or `EVOLVE_<ADAPTER>_REQUIRE_FULL=1`.

## See also

- [capability-schema.md](capability-schema.md) — adapter capability manifest schema + authoring guide for new CLIs
- [reference/platform-detect.md](../../skills/loop/reference/platform-detect.md) — env-var probe table consulted at skill activation
- [reference/claude-tools.md](../../skills/loop/reference/claude-tools.md), [gemini-tools.md](../../skills/loop/reference/gemini-tools.md), [codex-tools.md](../../skills/loop/reference/codex-tools.md), [agy-tools.md](../../skills/loop/reference/agy-tools.md) — per-platform tool name maps
- [reference/claude-runtime.md](../../skills/loop/reference/claude-runtime.md), [gemini-runtime.md](../../skills/loop/reference/gemini-runtime.md), [agy-runtime.md](../../skills/loop/reference/agy-runtime.md), [generic-runtime.md](../../skills/loop/reference/generic-runtime.md) — per-platform invocation patterns
- [docs/incidents/gemini-forgery.md](../incidents/gemini-forgery.md) — why structural defenses are pipeline-level, not adapter-level
