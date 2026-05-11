# AGENTS.md — Cross-CLI Canonical Instructions

> **Read this file first if you are an AI agent (Claude Code, Codex CLI, Gemini CLI, or generic) working in this repository.** It is the source-of-truth for cross-CLI invariants. CLI-specific runtime details live in companion files: [CLAUDE.md](CLAUDE.md), [GEMINI.md](GEMINI.md). All three reference back to this document.

## What evolve-loop is

A self-evolving development pipeline that orchestrates 4 specialized agents (Scout, Builder, Auditor, Orchestrator) through 6 lean phases per cycle (Calibrate → Intent → Scout → Build → Audit → Ship → Learn). Tier-1 kernel hooks enforce phase ordering, role-scoped write paths, atomic ship semantics, ledger SHA verification, and v8.37+ tamper-evident hash-chained recording.

## Cross-CLI invariants (the universal rules)

These rules apply regardless of which CLI you are running under. They are STRUCTURAL — enforced by kernel hooks, not by prompt instructions.

### 1. Pipeline ordering is non-negotiable
Phases run Scout → Builder → Auditor → Ship/Record in that exact order. The phase-gate kernel hook (`scripts/guards/phase-gate-precondition.sh`) denies any subagent invocation that violates the sequence. There is no bypass short of an emergency operator override (`EVOLVE_BYPASS_PHASE_GATE=1`) which is logged loudly and considered a CRITICAL violation.

### 2. Subagents are spawned via `subagent-run.sh`, never via in-process tool calls
Every phase agent is spawned by `bash scripts/dispatch/subagent-run.sh <agent> <cycle> <workspace>`. This is enforced by the kernel hook. The in-process `Agent` (Claude Code) / `activate_skill` (Gemini) / equivalent (Codex) is **denied during a cycle**. Reason: in-process subagents bypass profile-scoped permissions and the tamper-evident ledger.

### 3. Commits go through `scripts/lifecycle/ship.sh`, never bare `git commit / git push`
The ship-gate kernel hook (`scripts/guards/ship-gate.sh`) denies bare git commit/push/gh release create. The only canonical entry point is `scripts/lifecycle/ship.sh`. ship.sh enforces audit verification, cycle binding (HEAD + tree_state_sha match), and v8.32+ version-aware self-SHA pinning. Operator escape: `--class manual` (interactive) or `EVOLVE_BYPASS_SHIP_GATE=1` (emergency).

### 4. Builder writes only inside its worktree
Each cycle gets a per-cycle git worktree (provisioned by `run-cycle.sh`, recorded in `cycle-state.json:active_worktree`). Builder's profile (`.evolve/profiles/builder.json`) restricts Edit/Write to the worktree path. The role-gate kernel hook (`scripts/guards/role-gate.sh`) denies edits outside that boundary. v8.31+ closed the Bash-redirect leak by adding interpreter-execution Bash denials.

### 5. Audits are PASS/WARN/FAIL — and WARN ships by default
Auditor writes `audit-report.md` with one of three verdicts. PASS ships. **WARN ships** (v8.28.0+ fluent-by-default; orchestrator persona aligned in v8.35.0). FAIL blocks. Operator opts back to legacy strict-on-WARN via `EVOLVE_STRICT_AUDIT=1`. The fluency policy is encoded in ship.sh; the orchestrator persona invokes ship.sh in both PASS and WARN paths.

### 6. The ledger is tamper-evident (v8.37.0+)
`.evolve/ledger.jsonl` records every subagent invocation with cycle binding, challenge token, artifact SHA, **prev_hash**, and **entry_seq**. Each new entry's prev_hash is the SHA256 of the previous entry's full JSON line. `.evolve/ledger.tip` records the latest entry's SHA atomically — truncation detection. Run `bash scripts/observability/verify-ledger-chain.sh` to confirm history integrity. Modifying any historical entry breaks the chain at the next entry.

### 7. Failure adaptation is fluent-by-default (v8.28.0+)
Prior failures are recorded in `state.json:failedApproaches[]` with structured classifications (infrastructure-transient, code-audit-fail, code-audit-warn, etc.). The failure-adapter (`scripts/failure/failure-adapter.sh`) returns deterministic decisions; the orchestrator follows them verbatim. Default mode is fluent (would-have-blocked rules emit awareness, not BLOCK). Strict mode (`EVOLVE_STRICT_FAILURES=1`) restores legacy block-on-recurring behavior.

### 8. Cost adaptation (v8.35.0+)
The auditor profile defaults to Opus, but `scripts/utility/diff-complexity.sh` auto-downgrades to Sonnet for trivial diffs (≤3 files, ≤100 lines, no security paths). Saves ~$1.89/cycle on routine cycles. Operator override: `MODEL_TIER_HINT=opus` forces Opus regardless.

## Per-CLI runtime details

This file covers the universal contract. CLI-specific runtime details live in companion files:

- **Claude Code**: see [CLAUDE.md](CLAUDE.md). Tier-1 production. Skills at `skills/<name>/SKILL.md`, plugin manifest at `.claude-plugin/plugin.json`. Slash commands at `.claude-plugin/commands/`. Kernel hooks fire as PreToolUse hooks per `.claude/settings.json`.

- **Codex CLI**: skills auto-discovered at `.agents/skills/<name>/SKILL.md` (this directory exists as symlinks to `skills/<name>/`). Codex reads this AGENTS.md as its canonical config. Tier-1 hybrid since v8.51.0: `scripts/cli_adapters/codex.sh` delegates to `claude.sh` when `claude` is on PATH (full caps), or runs in same-session DEGRADED mode otherwise (pipeline still completes; reduced isolation). Capability tier visible via `./bin/check-caps codex`.

- **Gemini CLI**: skills auto-discovered at `.agents/skills/<name>/SKILL.md`. See [GEMINI.md](GEMINI.md) for Gemini-specific notes. Tier-1-hybrid: skill activates from Gemini, runtime delegates to `claude` binary via `scripts/cli_adapters/gemini.sh`.

- **Generic / unsupported CLI**: see [skills/evolve-loop/reference/generic-runtime.md](skills/evolve-loop/reference/generic-runtime.md). Tool name translation tables at `skills/evolve-loop/reference/<platform>-tools.md`.

## Discovery contract for AI agents reading this file

If you are an AI agent activating in this repository:

1. **Identify your CLI**: Claude Code, Codex, Gemini, or other.
2. **Read your CLI-specific overlay**: CLAUDE.md, GEMINI.md, or `docs/architecture/platform-compatibility.md`.
3. **Read this AGENTS.md** in full — the cross-CLI invariants apply to you.
4. **Discover available skills**: scan `.agents/skills/*/SKILL.md` (cross-CLI standard) or `skills/*/SKILL.md` (Claude Code primary).
5. **Discover available agents**: scan `agents/*.md`.

Skill files use YAML frontmatter (`name`, `description`) followed by markdown instructions. Skills include subdirectories (`scripts/`, `references/`, `assets/`) for resources used during execution.

## Trust boundary summary

The pipeline's safety properties stack into three tiers:

| Tier | Layer | What it catches |
|---|---|---|
| Tier 1 | Kernel hooks (phase-gate, role-gate, ship-gate, ledger SHA, cycle binding, hash chain) | Reward hacking, phase-skipping, integrity breach, tampering |
| Tier 2 | OS isolation (sandbox-exec on macOS, bwrap on Linux) | Compromised builder writing outside its sandbox |
| Tier 3 | Workflow defaults (intent capture, fan-out, mutation testing, adversarial audit) | Vague goals, sycophantic audits, tautological evals |

Tier 1 is non-negotiable and runs in privileged shell context. Tier 2 adapts to the environment. Tier 3 is operator-controlled per-run.

## Where to file issues

- Security vulnerabilities: see [SECURITY.md](SECURITY.md)
- Code of conduct: see [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)
- Contributions: see [CONTRIBUTING.md](CONTRIBUTING.md)
- Pipeline issues: GitHub Issues at https://github.com/mickeyyaya/evolve-loop/issues
- Architecture / release protocol: [docs/guides/publishing-releases.md](docs/guides/publishing-releases.md), [docs/architecture/tri-layer.md](docs/architecture/tri-layer.md)
