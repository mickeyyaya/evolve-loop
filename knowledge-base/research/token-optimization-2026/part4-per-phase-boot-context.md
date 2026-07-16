# Part 4 — Per-Phase Boot-Context Minimization ("clean-start / safe-mode")

> **Added 2026-07-17.** Companion to [README.md](README.md) + parts 1–3. Those cover
> *within-session* (observation masking) and *handoff* (report contracts) economics.
> This part isolates a lever they did not: the **fixed context re-ingested on every
> phase-agent CLI spawn** — ~7–11 spawns/cycle, each a fresh `claude`/`agy`/`codex`.
> Motivating operator ask (2026-07-17): "start the LLM CLI in a safe mode — a clean
> start without loading any context" + leaner prompts + single-responsibility phases.

## The hotspot

Every phase spawn re-ingests the SAME governance/boot context, none of which the
current Go dispatch (`go/internal/bridge`) suppresses:

| Boot component | ~tokens | Loaded because | Needed by the phase agent? |
|---|---:|---|---|
| `CLAUDE.md` (repo) + global `~/.claude/CLAUDE.md` + `rules/**` | 1.3K + ~2K | claude auto-discovers up the tree from the worktree | **No** — integrity is enforced by OUTER hooks, not inner context |
| Cascade if agent obeys "read AGENTS.md/runtime-reference first" | +4.4K / +15K | the CLAUDE.md instruction | No — only loop-behavior phases need runtime-reference |
| Auto-loaded skills (loop `SKILL.md` alone = 20KB ≈ 5K tok) | ~5K+ | plugin/skill auto-injection on startup | **No** — a builder does not need the loop skill |
| Full built-in tool schemas (all default tools) | ~1–3K | default `claude` boots every tool | No — audit needs Read/Grep/Bash; not Write/Edit |
| Dynamic system-prompt sections (cwd, env, git status) | ~100–300 | default system prompt | Same info fine in first user msg (cache-friendlier) |

**Per-spawn redundant boot ≈ 5–15K tokens** (CLAUDE.md cascade + skills + schemas),
re-paid **7–11×/cycle × N cycles**. NOT hotspots (verified): the giant
`--allowedTools`/`--disallowedTools` pattern lists are **harness-side, zero model
tokens** (permission checks, not context) — do not waste effort trimming them.

> **Measured corroboration (cycle-867, after the telemetry-attribution fix):** a
> single build-phase assistant turn carried **`cache_read` = 173,121 tokens** of
> context. That is the boot+accumulated context re-presented each turn — precisely
> the surface this part targets. Before the fix it was recorded as `input:0` (see
> the token-telemetry attribution defect writeup).

## The levers — real, current `claude` CLI flags (2026)

| Flag | Effect | Risk |
|---|---|---|
| **`--bare`** | *"skip hooks, LSP, plugin sync, attribution, auto-memory, background prefetches, keychain reads, AND CLAUDE.md auto-discovery"* (`CLAUDE_CODE_SIMPLE=1`). Context must be provided explicitly (`--append-system-prompt[-file]`, `--add-dir`, `--agents`). | Medium — see rollout |
| **`--disable-slash-commands`** | drops all skill (`SKILL.md`) auto-injection; phase loads only what it needs | Low |
| **`--tools "Read,Grep,Bash"`** | whitelist which built-in tools load → fewer tool **schemas** in context (unlike `--allowedTools`, this DOES cut tokens) | Low, per-phase |
| **`--exclude-dynamic-system-prompt-sections`** | moves machine-local sections to first user msg → cross-spawn prompt-cache reuse | **Zero** — no context lost |
| **`--setting-sources local`** | ignore global+project user customizations | Low |

`--safe-mode` also exists (more nuclear — also drops policy settings); prefer `--bare`.

## Why clean-start is INTEGRITY-SAFE (the load-bearing argument)

The loop's readiness banner states it plainly: *"the outer Claude Code session +
Tier-1 hooks (phase-gate, role-gate, ledger SHA) are the only confinement."* The
gates that enforce TDD, role boundaries, ledger integrity, and ship eligibility run
in the **supervisor** (the Go loop + kernel hooks), NOT inside the phase agent's
context window. So stripping the inner agent's CLAUDE.md/skills weakens **no**
guarantee — the agent only needs its *phase-specific* instructions, which the loop
already injects via the profile + resolved-prompt (self-contained, not the global
governance). Clean-start removes redundant belt; the suspenders are external.

## SkillReducer (arXiv 2603.29919) — per-phase skill JIT

Delta-encode skills (store only differences across related skills) + progressive
disclosure (base template loads; params/examples load only when invoked):
**40–60% skill-token reduction, <2% quality drop.** Directly actionable: give each
phase only its skill delta, not the whole SKILL.md; share a base template across
phases. Pairs with `--disable-slash-commands` (drop all) + inject the phase's skill
digest via `--append-system-prompt-file`.

## MEASUREMENT FIRST (hard prerequisite — now UNBLOCKED)

Parts 1–3 open with "baseline before optimizing." Until 2026-07-17 we **could not**:
per-phase telemetry recorded `input:0, cache_read:0` because the claude transcript
tier was never attributed (an exact `cwd == Worktree` gate that failed under the
tmux driver's lossy worktree propagation). **Fixed** by keying attribution on the
launch-unique `ArtifactPath` (`go/internal/tokenusage/scanner.go`). Real per-phase
input/cache now flows into `llm-calls.ndjson`. Capture a per-phase baseline (S7
`evolve tokens report`) BEFORE flipping any lever below, and re-measure after — every
saving must be a demonstrated delta, not an asserted one.

## Rollout (config-only, shadow-validated — no Go literals)

Levers are **config**: the `claude-tmux`/`agy-tmux`/`codex-tmux` manifests are
`go:embed` (read-only) but `LoadManifest` consults a writable override dir first, and
the per-driver launch flags are threaded through `realizer.go`. The clean-boot flags
belong in **policy.json**, read by `realizer.go` and appended to the driver's launch
flags (config values, no Go literals — `phases_are_config_only` honored). Sequence:

1. **Telemetry baseline** — land S7 `evolve tokens report`; capture per-phase input:output.
2. **Zero-risk first**: `--exclude-dynamic-system-prompt-sections` on all drivers (no
   context lost, pure cache win). Measure the cache-hit delta.
3. **Shadow-validate `--bare`**: run each phase under `--bare` + injected phase
   context in shadow; assert the resolved-prompt is **self-contained** (equivalent
   deliverable without auto-loaded CLAUDE.md/skills). Gate enable on per-phase
   equivalence — a phase whose prompt secretly relied on ambient CLAUDE.md must have
   that dependency moved into its profile first.
4. **Per-phase `--tools` whitelist** from each profile's real needs.
5. **Skill JIT** (SkillReducer) once #3 proves the injection path.

## Expected savings (bracketed, quantify post-telemetry)

- `--exclude-dynamic…`: ~100–300 tok/spawn + cache-reuse across spawns.
- `--bare` + `--disable-slash-commands`: ~5–15K tok/spawn (CLAUDE.md cascade + skills) — the big lever, ×7–11 spawns × N cycles.
- Per-phase `--tools`: ~1–2K tok/spawn on over-provisioned phases.
- **Order of magnitude: 50–150K tokens/cycle** on boot alone, before touching within-session (parts 1–3). Confirm on real telemetry — this is a hypothesis, not a measured result.

## Sources
claude CLI `--help` (2026: `--bare`, `--exclude-dynamic-system-prompt-sections`,
`--tools`, `--disable-slash-commands`, `--setting-sources`) · SkillReducer arXiv
2603.29919 · Anthropic context-engineering (curate the optimal token set) · this
repo's readiness-banner confinement note · parts 1–3 + the token-telemetry campaign.
