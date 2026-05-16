# Codex CLI Runtime (Hybrid Driver, v8.51.0+)

> How `/evolve-loop` reaches the dispatcher under OpenAI Codex CLI. **The Claude binary is recommended at runtime for full caps, but no longer required** — v8.51.0 added graceful degradation. See [v8.51 capability model](#v851-capability-model) below.

## Invocation chain

```
User: /evolve-loop 5 polish improve dispatcher
  (typed into Codex CLI)

  ↓ Codex resolves the skill
  ↓ activates SKILL.md content; reads reference/platform-detect.md
  ↓ detects platform = codex, reads reference/codex-runtime.md (this file)

Skill activates → STRICT MODE: execute exactly one shell command:
  bash scripts/dispatch/evolve-loop-dispatch.sh 5 polish "improve dispatcher"

  ↓ (Codex calls its shell-execution tool)

Dispatcher loops once per cycle:
  bash scripts/dispatch/run-cycle.sh "improve dispatcher"

  ↓

run-cycle.sh spawns the orchestrator subagent via:
  bash scripts/dispatch/subagent-run.sh orchestrator $CYCLE $WORKSPACE

  ↓

subagent-run.sh reads .evolve/profiles/orchestrator.json
  → cli = "codex"  (set by Codex CLI users via env or profile override)
  → reads scripts/cli_adapters/codex.capabilities.json
  → dispatches to scripts/cli_adapters/codex.sh

  ↓

codex.sh (HYBRID + DEGRADED, v8.51.0+):
  1. Probes for `claude` binary on PATH
  2. If found (HYBRID):    delegates to scripts/cli_adapters/claude.sh
                            → full caps via Claude subprocess isolation
  3. If missing (DEGRADED): same-session execution, pipeline still runs
                            → calling LLM (Codex) writes artifacts directly
                            → kernel hooks + forgery defenses still apply
  4. Opt-in hard-fail: --require-full or EVOLVE_CODEX_REQUIRE_FULL=1
                       → exit 99 if claude is missing
```

## v8.51 capability model

Codex's adapter declares modes per dimension via `scripts/cli_adapters/codex.capabilities.json`:

| Capability | Hybrid (claude on PATH) | Degraded (no claude) | Quality impact when degraded |
|---|---|---|---|
| `subprocess_isolation` | inherited from claude.sh | same-session execution | Builder + Auditor share session memory |
| `budget_cap` | inherited (`--max-budget-usd`) | none | Runaway cycles uncapped; mitigation: `EVOLVE_RUN_TIMEOUT` |
| `sandbox` | inherited (sandbox-exec / bwrap) | none | OS-level isolation absent; pipeline gates still fire |
| `profile_permissions` | inherited (`--allowedTools`) | none | Subagents have whatever tools the calling Codex session allows |
| `challenge_token` | embedded in profile prompt | post-hoc artifact verification | Forgery slightly harder to detect early |

**Critical safety invariant:** the four kernel hooks (`role-gate`, `ship-gate`, `phase-gate-precondition`, ledger SHA chain) fire on **bash commands**, not on adapter dispatch. They protect the pipeline regardless of whether the adapter is in HYBRID or DEGRADED mode. The Gemini Forgery defenses (v7.9.0+: artifact content checks, git diff substance gate, state.json checksum, .sh write protection, anti-forgery prompt inoculation) ALSO operate at the pipeline layer. Together they mean DEGRADED mode is structurally safe, just less isolated.

## Why hybrid (default for production)

OpenAI Codex CLI as of 2026-05 lacks the same primitives Gemini CLI lacks:

| Primitive | Codex status | What breaks without it |
|---|---|---|
| Non-interactive prompt mode (e.g., `codex -p`) | Unconfirmed support | Cannot spawn isolated subagent sessions natively |
| Native `--max-budget-usd` cost cap | Not supported | Runaway cycles uncapped without external timeout |
| Subagent / Task tool with profile-scoped permissions | Not supported | Builder/Auditor cannot be sandboxed natively |

When `claude` is on PATH, the hybrid shim delegates everything to `claude -p` — preserving the full Claude Code trust boundary (sandbox-exec, role-gate, ship-gate, phase-gate-precondition, profile permissions). Codex provides the conversational front-end; Claude provides the isolated execution back-end.

## Why degraded mode is acceptable (v8.51.0+)

Pre-v8.51, missing `claude` → `codex.sh` exited 99 → pipeline blocked. The user directive for v8.51 was: *"the process/pipeline should function regardless of which CLI is used. Missing features should only lower the quality, not block."*

DEGRADED mode runs the cycle in the same session (Codex itself produces artifacts via its file-write tools rather than spawning a Claude subprocess). The pipeline-level kernel hooks still gate every git/gh operation, the ledger SHA chain still enforces tamper-evident provenance, and the v7.9.0+ forgery defenses still validate artifact content. So even without subprocess isolation, the cycle cannot silently fabricate state.

Operators who NEED full hybrid (production with budget caps, subprocess isolation) opt back into hard-fail with `--require-full` or `EVOLVE_CODEX_REQUIRE_FULL=1`. Default is graceful degradation.

## Required environment

| Variable | Required | Purpose |
|---|---|---|
| `codex` binary on PATH | yes | The conversational driver |
| `claude` binary on PATH | **recommended** | Hybrid runtime engine (omit → DEGRADED mode) |
| `ANTHROPIC_API_KEY` | when in HYBRID mode AND not in a logged-in Claude session | Auth for `claude -p` |
| `OPENAI_API_KEY` (or equivalent) | yes | Auth for Codex CLI itself |

## Verifying capability tier before running cycles

```bash
# 1. Confirm Codex binary is present
command -v codex

# 2. Resolve current capability tier
./bin/check-caps codex
# Expected:
#   HYBRID:   Quality tier: hybrid
#   DEGRADED: Quality tier: degraded or none + per-capability warnings

# 3. Smoke-test detection
bash scripts/dispatch/detect-cli.sh
# Expected: prints "codex" if you're in a Codex session

# 4. Run the contract test
bash scripts/tests/codex-adapter-test.sh
# Expected: green (11 tests)
```

## Profile.cli is authoritative (v8.51.0+ → v8.52.0 hook)

Each phase profile (`.evolve/profiles/{scout,builder,auditor,intent,retrospective}.json`) declares its own `cli` field. After v8.51, `subagent-run.sh` reads `profile.cli` as the **authoritative dispatch target** — not session-wide CLI detection. This is the foundation for v8.52.0 multi-LLM-per-phase: operators can configure e.g. Scout=Claude, Builder=Codex, Auditor=Gemini within a single cycle. The capability framework records `quality_tier` per phase entry in the ledger.

## What if I want true Codex-driven phases (no claude binary)?

DEGRADED mode IS Codex-driven. Pipeline kernel hooks remain active. If you need full caps without a claude binary, two upstream conditions must hold before native Codex execution becomes safe:

1. Codex CLI ships non-interactive prompt mode (so subagent dispatch is structurally possible).
2. Codex CLI exposes profile-permission flags (so per-subagent isolation is enforceable).

Tracked in `docs/architecture/platform-compatibility.md` under v8.54.0 roadmap. Until then, DEGRADED mode is the supported native-Codex path.

## See also

- [reference/codex-tools.md](codex-tools.md) — tool name translation map
- [reference/platform-detect.md](platform-detect.md) — how the skill identifies its platform
- [reference/claude-runtime.md](claude-runtime.md) — what codex.sh delegates to in HYBRID mode
- [reference/gemini-runtime.md](gemini-runtime.md) — sister hybrid pattern; codex.sh mirrors gemini.sh
- [docs/architecture/platform-compatibility.md](../../../docs/architecture/platform-compatibility.md) — full capability matrix
- [docs/incidents/gemini-forgery.md](../../../docs/incidents/gemini-forgery.md) — historical incident motivating pipeline-level structural defenses

## Last verified

- **Date:** 2026-05-08
- **Codex CLI capability claims** (no native budget cap, no profile permission flags, non-interactive mode unconfirmed): based on public documentation as of 2026-05.
- **Re-verify cadence:** quarterly, or whenever OpenAI announces a Codex CLI feature release. If any of the three primitives ship natively, update `codex-tools.md` and the "Why hybrid" section above; bump capability manifest's defaults.
- **Quick re-verify command:** `codex --version && codex --help 2>&1 | grep -iE 'budget|prompt|tool|permission'`
