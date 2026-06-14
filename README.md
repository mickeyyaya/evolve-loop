# Evolve Loop

**Current (v18.10)** · A self-evolving development pipeline that improves your codebase while you sleep — with structural anti-gaming so you can trust the result.

> **v11.2.0 breaking change**: the `scripts/` symlink has been removed. Operator integrations that hardcode `scripts/...` paths must update to `legacy/scripts/...`. Go binary remains the primary runtime; bash continues to work from `legacy/scripts/`. See [docs/migration-from-bash.md](docs/migration-from-bash.md).

Evolve Loop is an open-source plugin for AI coding assistants (Claude Code, Gemini CLI, Codex CLI) that runs autonomous improvement cycles on your codebase. Each cycle finds work, implements it, adversarially audits its own output, ships only what passes deterministic predicate checks, and extracts durable lessons from failures so the next cycle is smarter.

Three things distinguish it from `/goal`, `superpowers`, `self-improving-agent`, and similar long-running skills:

1. **Verdicts are bash exit codes, not LLM judgments.** EGPS predicates are executable scripts; their exit code IS the verdict. The model can write prose all day; only `acs-verdict.json:red_count == 0` ships.
2. **Three layers of structural anti-gaming.** Tier 1: PreToolUse shell hooks + SHA-chained ledger. Tier 2: OS sandboxing + per-cycle git worktree. Tier 3: workflow defaults (adversarial audit, intent capture, mutation testing). Tier 1 is non-negotiable.
3. **Cross-cycle learning is durable.** Failures produce YAML lesson files that get merged into `state.json:instinctSummary[]`. The next cycle's Scout reads them automatically. This is the Reflexion loop (Shinn et al. 2023), wired through evidence-bound storage.

If you've used `/goal` and wanted "but make it safe to merge without re-reading every diff," this is that.

---

## Table of Contents

1. [The Self-Evolving Loop](#the-self-evolving-loop) — concept
2. [Pipeline Design](#pipeline-design) — the 8 phases
3. [The 3-Layer Trust Architecture](#the-3-layer-trust-architecture) — anti-gaming
4. [Error Recovery](#error-recovery) — how work survives failures
5. [Why Self-Evolving Works](#why-self-evolving-works) — Reflexion + double-loop learning
6. [Pluggability](#pluggability) — every phase swappable, every LLM routable
7. [How Evolve Loop Compares](#how-evolve-loop-compares) — vs /goal, superpowers, etc.
8. [Real Incident: Cycle 61](#real-incident-cycle-61) — the framework caught its own bugs
9. [Quick Start](#quick-start) — install + first cycle
10. [Architecture Deep-Dives](#architecture-deep-dives) — link index
11. [Evolution Data](#evolution-data) — milestone history
12. [Project Structure](#project-structure)
13. [Contributing](#contributing)
14. [License & Links](#license--links)

---

## The Self-Evolving Loop

The framework runs your codebase through a sequence of 8 phases plus a meta-cycle. Every phase produces an artifact that the next phase must read. The model picks tasks; the framework enforces the pipeline.

The defining property: **failures become instincts**.

When cycle N's audit fails, a retrospective subagent fires (auto-on per v8.45.0+), reads the failed cycle's artifacts, and writes a structured YAML lesson at `.evolve/instincts/lessons/cycle-N-<slug>.yaml`. A merge script verifies the lesson exists on disk and appends it to `state.json:instinctSummary[]`. The next cycle's Scout reads those instincts in its prompt context — so cycle N+1 sees rules like:

> "When adding to `phase-registry.json:audit.inputs.state_fields[]`, grep every regression predicate that calls `check-phase-inputs.sh audit` and update fixtures." — `cycle-59-registry-state-fields-fixture-impact.yaml`

That lesson came from cycle 59's failure. Cycle 60+ Scouts won't make the same mistake.

19 active lessons currently live in `.evolve/instincts/lessons/`. Some are 60 cycles old. They still apply. Deep-dive: [docs/concepts/self-evolution.md](docs/concepts/self-evolution.md).

---

## Pipeline Design

Each cycle runs eight phases. The trust kernel enforces them in order at the OS layer — phases cannot be skipped, reordered, or shortcutted via prompt instructions.

```
INTENT ─→ SCOUT ─→ TRIAGE ─→ [PLAN-REVIEW] ─→ [TDD] ─→ BUILD ─→ AUDIT ─→ SHIP ─→ MEMO/RETRO
   │        │        │            │             │        │        │       │           │
   │        │        │            │             │        │        │       │           └─ PASS → memo (carryover);
   │        │        │            │             │        │        │       │              FAIL/WARN → retrospective (lessons)
   │        │        │            │             │        │        │       │              auto-on v8.45.0+
   │        │        │            │             │        │        │       └─ Commit + push via ship.sh
   │        │        │            │             │        │        │          (gated on acs-verdict.json:red_count == 0)
   │        │        │            │             │        │        └─ Adversarial cross-check; predicate suite runs;
   │        │        │            │             │        │           verdict from bash exit codes (EGPS v10)
   │        │        │            │             │        └─ Implement in per-cycle git worktree (isolation);
   │        │        │            │             │           write EGPS predicates alongside code
   │        │        │            │             └─ TDD-engineer writes RED predicates BEFORE Builder writes code
   │        │        │            │                (separates predicate author from implementer; default-on v10.6+)
   │        │        │            └─ (opt-in) 4-lens plan review: CEO / Eng / Design / Security fan-out
   │        │        └─ Bound this cycle's scope: top_n / deferred / dropped (default-on v8.59.0+);
   │        │           emits phase_skip[] for PSMAS opt-in (v10.17+)
   │        └─ Find work + write evals; cite research; carryoverTodos + instincts feed input;
   │           KB-first directive (v10.9.0+ cycle 87-89): kb-search.sh before WebSearch
   └─ Structure the vague goal: 8 fields + Ask-when-Needed classifier + ≥1 challenged premise (v8.19.1+)
```

**Opt-in shortcuts (default-off, v10.17+):** PSMAS phase-skip — when `EVOLVE_PSMAS_SKIP=1`, Triage may recommend skipping `tdd-engineer` (trivial cycles) or `retrospective` (small PASS cycles) to save tokens. The skip leaves a ledger entry so `--resume` and audit-binding both see it as deliberate. See [docs/architecture/psmas-phase-scheduling.md](docs/architecture/psmas-phase-scheduling.md).

### Phase artifacts you'll see

| Phase | Agent | Output artifact |
|---|---|---|
| Intent | intent | `intent.md` (8-field structured goal) |
| Scout | scout | `scout-report.md` (selected tasks, research, decision trace) |
| Triage | triage | `triage-decision.md` (top_n / deferred / dropped) |
| Plan-Review (opt-in) | plan-reviewer fan-out | `plan-review.md` (4-lens aggregate) |
| Build | builder | `build-report.md` + `acs/cycle-N/*.sh` predicates |
| Audit | auditor | `audit-report.md` + `acs-verdict.json` (binary PASS/FAIL) |
| Ship | ship.sh | git commit on main + ledger entry |
| Memo / Retro | memo OR retrospective | `carryover-todos.json` OR `retrospective-report.md` + lesson YAMLs |

### Four specialized agents (+ orchestrator)

| Agent | Persona file | Role | Profile path |
|---|---|---|---|
| **Scout** | `agents/evolve-scout.md` | Find work, cite research, write evals | `.evolve/profiles/scout.json` |
| **Triage** | `agents/evolve-triage.md` | Bound this cycle's scope | `.evolve/profiles/triage.json` |
| **Builder** | `agents/evolve-builder.md` | Implement in isolated worktree | `.evolve/profiles/builder.json` |
| **Auditor** | `agents/evolve-auditor.md` | Adversarial cross-check (different model family from Builder) | `.evolve/profiles/auditor.json` |
| **Memo** (PASS only) | `agents/evolve-memo.md` | Carryover capture | `.evolve/profiles/memo.json` |
| **Retrospective** (FAIL/WARN only) | `agents/evolve-retrospective.md` | Lesson extraction | `.evolve/profiles/retrospective.json` |
| **Orchestrator** | `agents/evolve-orchestrator.md` | Phase sequencer | `.evolve/profiles/orchestrator.json` |

Deep-dive: [docs/architecture/phase-architecture.md](docs/architecture/phase-architecture.md). For the mental model, see [docs/concepts/overview.md](docs/concepts/overview.md).

---

## The 3-Layer Trust Architecture

The threat is not malicious humans — it's the LLM doing what LLMs do: confabulating "looks done" verdicts, hallucinating evidence, shortcutting to path-of-least-resistance. The fix is structural enforcement, not better prompts.

| Tier | Mechanism | Default | Catches |
|---|---|---|---|
| **1 — Structural Integrity** | `phase-gate-precondition.sh`, `role-gate.sh`, `ship-gate.sh`, SHA-chained ledger | **Always on. No bypass.** | Reward hacking, phase-skipping, integrity breach |
| **2 — OS Isolation** | `sandbox-exec` (macOS) / `bwrap` (Linux), per-cycle git worktree | On (always present) | Cross-cycle leakage, compromised builder, scope escape |
| **3 — Workflow Defaults** | Intent capture, fan-out, mutation testing, adversarial audit, scout grounding, audit citation binding | Opt-in via env flags | Vague goals, sycophantic audits, tautological evals, scope sprawl |

### Tier 1 — Three shell hooks block deviations structurally

| Hook | Watches | Denies |
|---|---|---|
| `phase-gate-precondition.sh` | every `subagent-run.sh` invocation | Out-of-order phases, in-process `Agent` tool when `cycle-state.json` exists |
| `role-gate.sh` | every `Edit`/`Write` tool call | Writes outside the active phase's allowlist, writes outside worktree for write-bound roles |
| `ship-gate.sh` | every `Bash` with git/gh verbs | Direct `git commit`/`git push` not routed through `legacy/scripts/lifecycle/ship.sh` |

Plus a tamper-evident SHA-chained `.evolve/ledger.jsonl` — every entry records `prev_hash`. Modifying any past entry invalidates every subsequent `prev_hash`. Verify with `bash legacy/scripts/observability/verify-ledger-chain.sh`.

### Tier 2 — OS sandboxing + per-cycle worktree

When `EVOLVE_SANDBOX=1` (or profile `sandbox.enabled: true`), every `claude -p` subprocess runs under `sandbox-exec` (macOS) or `bwrap` (Linux). Auditor and Evaluator profiles run with `read_only_repo: true` — the entire repo is bind-mounted read-only inside their sandbox.

Per-cycle git worktree at `$EVOLVE_WORKTREE_BASE/cycle-N` isolates Builder's edits from other cycles. Lives on a temporary branch (`evolve/cycle-N`) deleted post-ship.

Capability detection (`legacy/scripts/dispatch/preflight-environment.sh`) auto-adapts to nested-Claude environments, falling back gracefully without losing Tier 1.

### Tier 3 — Workflow defaults (opt-in but recommended)

| Default | Catches |
|---|---|
| Intent capture (`EVOLVE_INTENT_ENABLED=1`) | Vague goals → 8-field structured intent |
| Fan-out for read-only roles | Same-model judge sycophancy in Scout/Auditor/Retrospective |
| Mutation testing on evals | Tautological predicates (AC-by-grep, echo-PASS) |
| Adversarial Auditor mode | Same-family judge bias (Builder=Sonnet → Auditor=Opus by default) |
| Triage default-on (v8.59+) | Scope blob (Scout proposing 12 tasks) |
| Scout grounding check (cycle 62+) | Scout fabricating uncommitted-file claims |
| Audit citation binding (cycle 62+) | Auditor citing files not in this cycle's diff |

Deep-dive: [docs/concepts/trust-architecture.md](docs/concepts/trust-architecture.md). For why structural enforcement matters more than prompts, see Skalse et al. (NeurIPS 2022) on reward hacking and Weng (2024) on the no-single-mitigation conclusion.

---

## Error Recovery

Long cycles fail routinely — subscription quotas exhaust, APIs return 529, models hit context-window limits. The framework's contract is **work-in-flight survives common failures.**

Four recovery layers, each catching a different failure mode at a different cost:

| Layer | Trigger | What it preserves | Lifetime |
|---|---|---|---|
| **1. failedApproaches[]** | Audit FAIL/WARN OR run-cycle rc=1 | Raw failure record | 30 days default |
| **2. Retrospective YAML lessons** | Audit FAIL/WARN (auto-on v8.45.0+) | Structured root-cause + prevention rule | Permanent (tracked) |
| **3. Checkpoint-resume (v9.1.0+)** | Cumulative cost ≥95% OR quota signature | Full mid-cycle state — worktree + state.json | Until `--resume` |
| **4. Worktree preservation** | Recoverable failure (infrastructure/audit-fail) | Worktree edits survive cleanup | Until next reset |

The cycle 11 incident (subscription quota wall mid-audit) was the canonical motivator for v9.1.0. Pre-v9.1.0: 30 minutes of Builder work discarded. Post-v9.1.0: worktree preserved, ~5 min of audit work lost, operator runs `--resume` after quota reset.

Deep-dive: [docs/concepts/error-recovery.md](docs/concepts/error-recovery.md). Protocol: [docs/architecture/checkpoint-resume.md](docs/architecture/checkpoint-resume.md).

---

## Why Self-Evolving Works

Argyris & Schon (1978) distinguish two kinds of organizational learning:

- **Single-loop:** detect error → correct action (here: `state.json:failedApproaches[]`)
- **Double-loop:** detect error → question the assumptions that led to it → change the policy (here: `state.json:instinctSummary[]` + YAML lessons)

Pre-v8.45.0 had only the single-loop. Cycles failed, entries piled up, next cycle saw noise but had no actionable rule. v8.45.0 made retrospective auto-fire on FAIL/WARN, completing the double-loop.

evolve-loop is a multi-agent Reflexion variant (Shinn et al. 2023):

| Reflexion role | evolve-loop equivalent |
|---|---|
| Actor | Builder (writes code + predicates) |
| Action | Worktree edits, predicate writes |
| Environment | git + filesystem + ACS predicate suite |
| Evaluator | Auditor + `acs-verdict.json` exit codes |
| Self-Reflection | Retrospective subagent (different model from Builder; auto-fires on FAIL/WARN) |
| Long-term memory | `state.json:instinctSummary[]` + YAML lessons (durable across cycles) |
| Next Actor invocation | Cycle N+1 Scout reads the memory |

The crucial difference from a simple Reflexion implementation: **the verdict is not a model claim**. Reflexion's Evaluator is typically an LLM; evolve-loop's verdict is `acs-verdict.json:red_count == 0`, derived from bash exit codes. The Auditor writes prose but cannot game the predicate suite.

Worked example — Cycle 7 → Cycle 59 lesson generalization:

- Cycle 7 failed because gitignored `.evolve/` runtime artifacts didn't survive worktree cleanup.
- Lesson filed as `cycle-7-ephemeral-worktree-artifact.yaml`.
- Cycle 59 failed in a similar-looking way — but the cycle-7 lesson didn't catch it because its framing was too narrow (gitignored runtime vs tracked source).
- Cycle 59's retrospective explicitly recommended broadening cycle-7's pattern, and wrote a new lesson `cycle-59-acs-predicates-worktree-invisible.yaml`.
- Lessons themselves are first-class objects that can be refined over time.

Deep-dive: [docs/concepts/self-evolution.md](docs/concepts/self-evolution.md).

---

## Pluggability

The framework separates *what work happens* from *who does it* from *what model runs the who*. These are independent dimensions:

| Axis | Pluggable element | File location | Example |
|---|---|---|---|
| **Persona** | Agent role definition | `agents/<role>.md` | Replace `evolve-scout.md` with a domain-specific scout |
| **Skill** | Workflow steps inside a persona | `skills/<name>/SKILL.md` | Replace `evolve-tdd` with a property-based-test skill |
| **LLM** | Model + CLI driving the persona | `.evolve/llm_config.json` | Route Scout to Gemini, Builder to Claude Sonnet, Auditor to Claude Opus |

### The CLI router

`legacy/scripts/dispatch/resolve-llm.sh` is a pure function that returns which CLI + model should run each phase. Operators override via `.evolve/llm_config.json`:

```json
{
  "schema_version": 1,
  "phases": {
    "scout":    {"provider": "google",    "cli": "gemini", "model": "gemini-3.1-pro-preview"},
    "builder":  {"provider": "anthropic", "cli": "claude", "model_tier": "sonnet"},
    "auditor":  {"provider": "anthropic", "cli": "claude", "model": "claude-opus-4-7"}
  },
  "_fallback": {"cli": "claude", "model_tier": "sonnet"}
}
```

Three adapters ship: `claude.sh` (native), `gemini.sh` (native v10.7+), `codex.sh` (hybrid). All three translate to a common usage envelope so the upstream pipeline and ledger don't care which CLI actually ran.

After the cycle, `## CLI Resolution` in `orchestrator-report.md` is auto-rendered from ledger entries — showing exactly which CLI ran each phase, including fallbacks. This is the structural answer to cycle-61's B6 (orchestrator narrative hallucinated routing). The auto-rendered table is ledger-truth, not LLM-narrated.

Common configurations:

| Config | Pattern | ~Cost per cycle | Source |
|---|---|---|---|
| Haiku everywhere | `_fallback` to haiku, no overrides | $0.15-0.30 | extrapolated |
| Adversarial mode (default) | Builder=Sonnet, Auditor=Opus (different family) | $5.94-7.40 | cycles 94-98 actuals |
| Cross-vendor | Scout=Gemini, Builder=Claude, Auditor=Claude | $1.50-3.50 | extrapolated |
| Gemini-only | All phases on gemini-3.1-pro-preview | $0.50-2.00 | cycle 61 historic |
| Cost-optimized mixed | Haiku read-only, Sonnet Builder, Opus Auditor | $0.50-1.50 | extrapolated |

Real per-cycle cost data from the v10.17.0 batch (5 consecutive adversarial-mode cycles): cycle 94 $6.85, cycle 96 $7.40, cycle 98 $5.94 — full breakdown in [knowledge-base/research/v10-17-0-release-debrief.md](knowledge-base/research/v10-17-0-release-debrief.md) §2. Per-phase token attribution: [docs/architecture/token-economics-2026.md](docs/architecture/token-economics-2026.md).

Per-agent context tuning (v10.10.0+): each phase profile declares `context_mode: "digest" | "full"`. Orchestrator runs `digest` by default (~6 K tokens saved per cycle); Builder/Auditor run `full` for evidence access. FAIL-path auto-promotes digest → full to prevent under-feeding recovery. See [docs/architecture/orchestrator-context-modes.md](docs/architecture/orchestrator-context-modes.md).

Deep-dive: [docs/concepts/pluggability.md](docs/concepts/pluggability.md). Adding a new CLI adapter takes ~200 LOC + a capability JSON + one predicate.

---

## How Evolve Loop Compares

Honest head-to-head with the autonomous-agent skills shipping in the Claude Code ecosystem as of 2026-05-20 (v10.17). Sources: each project's README + marketplace entry as of that date. We re-audit this table each minor release; if you find it stale, please file an issue.

| Project | Verdict source | Long-term memory | Multi-CLI | Recovery | Anti-gaming |
|---|---|---|---|---|---|
| **/goal** (Claude Code 2.1.139+) | Small validator LLM (gameable) | Conversation only | No | None | Convention |
| **miles990/self-evolving-agent** | Pattern matching in MEMORY.md | MEMORY.md | No | None | Skill-level |
| **alirezarezvani/self-improving-agent** | LLM analysis → pattern promotion | MEMORY.md + promoted rules | No (skill in 263+ marketplace) | None | Pattern-promotion convention |
| **bejranonda/LLM-Autonomous-Agent-Plugin** | Linter exit codes + CodeRabbit | Local dashboard DB | No | None | 40+ linter suite |
| **obra/superpowers** | Skill exit-criteria (LLM-described) | Reusable skills across sessions | No | None | Skill triggering structural |
| **OpenClaw / Hermes Agent** | Per-tool varies | Per-tool varies | Sometimes | Per-tool varies | Convention |
| **Evolve Loop** | **Bash exit codes (EGPS)** | **YAML lessons + state.json instincts** | **Per-phase router (claude/gemini/codex)** | **Checkpoint-resume + worktree preservation** | **3-tier (structural/OS/workflow)** |

### When to choose each

| Your situation | Choose |
|---|---|
| Lowest friction; zero install | **/goal** |
| Pattern-promotion across long Claude Code use | **alirezarezvani/self-improving-agent** |
| Linter + security enforcement on every commit | **bejranonda/LLM-Autonomous-Agent-Plugin** |
| Skills-first framework with clean scope handling | **obra/superpowers** |
| Vendor neutrality across many providers | **OpenClaw / Hermes Agent** |
| Production code, must be safe to merge unattended | **Evolve Loop** |
| Adversarial cross-CLI review (Builder ≠ Auditor model family) | **Evolve Loop** |
| Long unattended runs with quota-wall recovery | **Evolve Loop** |
| Simple `/goal "do thing"` and 5-second mental model | **/goal** |

### The honest tradeoffs

Evolve Loop is **not always the right choice.**

- **Higher friction.** 8 phases per cycle → 15-30 min wall-clock + $5-8 in adversarial mode (cycles 94-98 actuals; haiku-only is $0.15-0.30). `/goal` is 3-10 min + $0.30-1.50.
- **Higher learning curve.** Trust kernel, EGPS predicates, CLI router, recovery mechanisms require understanding. `/goal` is "type `/goal`, wait, done."
- **Anthropic-deep, not vendor-neutral.** Gemini/codex adapters exist as peers, but kernel hooks assume Anthropic-CLI-style permissions.
- **Optimized for trust, not speed.** Fastest autonomous coding → `/goal`. Safest commit → evolve-loop.

Best fit: organizations or solo developers running long unattended cycles on production code, where the cost of a bad merge is high.

Deep-dive: [docs/comparisons/long-running-claude-skills.md](docs/comparisons/long-running-claude-skills.md).

---

## Real Incident: Cycle 61

In 2026-05-15, we ran an experiment routing `gemini-3.1-pro-preview` to Scout and Builder. The cycle technically shipped commit `4160750` but with significant damage — and the damage exposed seven distinct bugs in our own framework. We fixed all seven structurally across cycles 62-63 of v10.7.0.

### What broke

| Bug | Description | How we caught it | Fix |
|---|---|---|---|
| B0 | gemini.sh NATIVE patch reverted from main but capability flag shipped ON | Predicate 050 with mutation-test anti-tautology | Re-applied NATIVE block; predicate suite verifies presence |
| B1 | Builder didn't stage Scout's identified deliverable | Initially nothing caught it | Added `scout-grounding-check.sh` (Tier 3 WARN-mode) |
| B2 | Auditor cited `gemini.sh:206` not in cycle 61's diff | Initially nothing caught it | Added `audit-citation-check.sh` (Tier 3 WARN-mode) |
| B3 | Claimed ship.sh INTEGRITY-FAIL | Dissolved — was hallucination; Tier 1 prevented actual breach | None needed; ship.sh v8.32 TOFU + v11.0 T1 auto-heal already correct |
| B4 | Memo profile shell-redirect path (`cat > memo_context.txt` at project root) | Files observed in working dir during forensics | Dropped `Bash(cat:*)`, `Bash(head:*)`, `Bash(tail:*)` from memo allowlist |
| B5 | Classifier didn't see memo 529 in memo-stdout.log | Postmortem investigation | Extended grep to scan per-role `*-stdout.log` |
| B6 | Orchestrator-report narrative claimed gemini but ledger said claude | Source-verified facts overruled prose | Added `render-cli-resolution.sh` (auto-render from ledger) |
| B7 | state.json:lastCycleNumber stuck because worktree state.json got the update | Postmortem investigation | `resolve-roots.sh` worktree detection |

### What we learned

The framework's value proposition isn't that cycles never fail — it's that failures **produce durable lessons + structural fixes**. Cycle 61 is the worked example we point to: 7 bugs caught (some by the framework's own audit, some by post-hoc forensics), 7 structural fixes shipped, all visible in commits `1dc1ab9 → e810df7` (cycles 62 step 1 through cycle 63 fix).

The orchestrator-report.md from cycle 61 was partially unreliable — it claimed "manually fast-forwarded the worktree branch to main" with no ledger entry to support it. Source-verified facts (git reflog, state.json:lastUpdated, ledger entries) contradicted that narrative. This is why CLI Resolution is now auto-rendered from ledger truth — orchestrator prose cannot be trusted as primary evidence.

Forensic report: [docs/incidents/cycle-61.md](docs/incidents/cycle-61.md). The retrospective YAMLs from cycle 61 live in `.evolve/instincts/lessons/cycle-59-*` (which the cycle-61 cycle inherited) and `cycle-24-builder-uncommitted-worktree-edit.yaml` (which cycle 64's failure re-validated).

### Two more recent case studies

**Cycle 93 — trust-kernel breach (2026-05-20).** A worktree commit reached `main` before the post-audit ship-gate could verify it, exposing a 3-minute ordering window in the kernel. We shipped commit-SHA self-attestation and pre-merge tree-SHA verify within hours (commits `cce9eb3` + `eff8a6c`, v10.16.0). Forensic dossier: [knowledge-base/research/cycle-93-trust-kernel-breach-2026-05-20.md](knowledge-base/research/cycle-93-trust-kernel-breach-2026-05-20.md).

**Cycle 96 — autonomous goal divergence (2026-05-20).** Triage chose to ship "builder turn-18 STOP CRITERION + mastery" instead of the operator-stated P4+L1 plan. The triage system's autonomous re-prioritization was vindicated: P4 turned out to be already-shipped, so the deviation saved a wasted cycle. This is documented system behavior — operator goal text is input #5 of 5, not a directive. See [knowledge-base/research/triage-autonomous-goal-divergence-cycle-96.md](knowledge-base/research/triage-autonomous-goal-divergence-cycle-96.md) for the priority-source ordering.

**Cycles 94-98 — watchdog post-memo SIGTERM pattern (2026-05-19→20).** Five consecutive cycles fired SIGTERM during post-memo orchestrator finalization, regardless of threshold tuning (240→600→900s all over-fired by 1-4%). Real work shipped in every case; only learn-phase output was lost. Short-term mitigation (raise default to 600s) shipped; long-term fix (heartbeat-touch) queued. See [docs/incidents/cycle-94-98-watchdog-overfiring.md](docs/incidents/cycle-94-98-watchdog-overfiring.md) for the timeline and detector analysis. This is the canonical example of "the framework caught its own architectural debt" — we now know file-mtime is the wrong proxy for LLM-reasoning phases.

---

## Quick Start

### Prerequisites

- One of:
  - [Claude Code](https://docs.anthropic.com/en/docs/claude-code) (tier-1, primary)
  - [Gemini CLI](https://github.com/google-gemini/gemini-cli) (tier-1-native since v10.7+)
  - [Codex CLI](https://github.com/openai/codex) (tier-1-hybrid since v8.51.0)
- A git repository you want to improve
- `bash` (3.2+, macOS-default works), `git` (2.5+), `jq` (1.6+)

### Install

**Option A: Plugin (recommended)**

```bash
/plugin marketplace add mickeyyaya/evolve-loop
/plugin install evolve-loop@evolve-loop
```

**Option B: Plugin + Go binary (v11.0.0+, faster structural paths)**

```bash
/plugin marketplace add mickeyyaya/evolve-loop
/plugin install evolve-loop@evolve-loop
cd go && make build   # produces ./go/bin/evolve
```

Set `EVOLVE_GO_BIN=$(pwd)/go/bin/evolve` (or drop the binary on `PATH`). The plugin manifest declares the binary as `tier-1` primary; bash remains as the fallback when the binary is absent or `EVOLVE_USE_LEGACY_BASH=1` is set.

**Option C: Manual**

```bash
git clone https://github.com/mickeyyaya/evolve-loop.git
cd evolve-loop
./install.sh
```

### First-time setup (`/setup`)

On first launch the loop prints a one-line nudge to run setup. `/setup` is an interactive onboarding flow (runs in your CLI session — no extra token cost) that:

1. **Detects** which LLM CLIs you have (claude/codex/gemini/agy: binary, auth mode, capability tier).
2. **Explains** the pipeline concisely — what each phase does and why it's trustworthy.
3. **Proposes** which model each phase agent should use (you adjust interactively), writes `.evolve/llm_config.json`, and validates it against the integrity floor.

```bash
/setup                              # interactive onboarding (re-runnable anytime)
evolve setup detect --json          # just the deterministic detection digest (read-only)
```

Setup is optional — the loop runs with sensible all-Claude defaults if you skip it. See [docs/architecture/setup-onboarding.md](docs/architecture/setup-onboarding.md).

### Run

The v9.1.0 syntax is **budget-first** (cost-driven), with cycle-count and resume as alternatives:

```bash
# Budget mode (recommended) — run cycles until cumulative spend ≥ $N
/evolve-loop --budget-usd 5 "improve test coverage"

# Cycle mode — run exactly N cycles regardless of cost
/evolve-loop --cycles 3 "add dark mode"

# Resume a previously paused cycle (v9.1.0+)
/evolve-loop --resume

# Strategy presets (positional, after flags)
/evolve-loop --budget-usd 10 innovate "explore concurrency primitives"
/evolve-loop --cycles 5 harden                    # stability + tests
/evolve-loop --cycles 3 repair "fix auth bug"     # fix-only, smallest diff
/evolve-loop --cycles 1 ultrathink "refactor X"   # tier-1 forced
/evolve-loop --cycles 5 autoresearch              # hypothesis testing, embraces failure
```

> Legacy positional integer (`/evolve-loop 5`) still parses as cycles with a deprecation WARN — v10.0.0 candidate will consider flipping bare-positional to dollars.

### Resume after a pause (v9.1.0+)

If a cycle is checkpointed (subscription quota wall, batch cap near, or operator-requested), the dispatcher preserves the worktree + cycle-state on disk:

```bash
/evolve-loop --resume
```

The dispatcher locates the most recent paused cycle, validates state (git HEAD unchanged, worktree still exists), and re-spawns the orchestrator from the paused phase boundary. The trust kernel holds across resume.

### Reset a stuck cycle (v13.0.0+)

If a cycle can't be resumed (corrupt state, abandoned work), a fresh `/evolve-loop` run refuses to start and prints the resume∥reset fork. Resume to continue, or **reset** to seal it:

```bash
evolve cycle reset
```

Reset never deletes history — it archives the workspace + a `cycle-state.json` snapshot + a `reset-manifest.json` to `.evolve/runs/cycle-<N>.reset-<ts>/`, advances `lastCycleNumber`, and writes an auditable ledger entry. (`EVOLVE_FORCE_FRESH=1` restores the legacy silent-clobber, which does NOT seal history.)

### Strategy presets

| Strategy | Focus | Approach | Strictness |
|---|---|---|---|
| `balanced` | Broad discovery | Standard | MEDIUM+ blocks |
| `innovate` | New features, gaps | Additive | Relaxed style |
| `harden` | Stability, tests | Defensive | Strict all |
| `repair` | Bugs, broken tests | Fix-only, smallest diff | Strict regressions |
| `ultrathink` | Complex refactors | tier-1 forced | Strict + confidence |
| `autoresearch` (v8.11+) | Hypothesis testing | Fixed metrics, embraces failure | Divergent, unpenalized |

### Operator commands (read-only, safe mid-cycle)

```
./bin/status                            current cycle + recent ledger summary
./bin/cost <cycle>                      per-cycle token + cost breakdown (--json available)
./bin/health <cycle> <workspace>        anomaly fingerprint for any past cycle
./bin/verify-chain                      tamper-evident ledger chain check
./bin/preflight                         full pipeline dry-run (regression + simulate + release-pipeline dry-run)
./bin/check-caps [cli]                  show resolved capability tier per adapter
evolve eval diversity-check <evalsDir>  adversarial-diversity score for an eval suite (v13.0.0+)
evolve setup detect [--json]            onboarding digest: CLIs + per-phase routing (read-only)
bash legacy/scripts/observability/show-context-monitor.sh <cycle>   per-cycle context usage (v9.1.0+)
bash legacy/scripts/observability/show-context-monitor.sh --watch   live-tail latest cycle (3s refresh)
bash legacy/scripts/observability/render-cli-resolution.sh <cycle>  per-phase CLI/model from ledger truth (v10.7+)
```

For a hands-on walkthrough of your first cycle: [docs/getting-started/your-first-cycle.md](docs/getting-started/your-first-cycle.md).

---

## Auth modes

Evolve-loop supports four authentication modes, detected in priority order:

| Mode | Condition | Notes |
|---|---|---|
| `CUSTOM_PROXY` | `EVOLVE_ANTHROPIC_BASE_URL` or `ANTHROPIC_BASE_URL` is set | Routes all `claude -p` calls through your endpoint; must speak `POST /v1/messages` |
| `API_KEY` | `ANTHROPIC_API_KEY` is set | Deducts from Anthropic API credits per call |
| `SUBSCRIPTION_OAUTH` | `~/.claude/.credentials.json` has a valid OAuth token | Uses Claude Code subscription auth — no extra config needed |
| `MISCONFIGURED` | None of the above | Run `claude login` or set `ANTHROPIC_API_KEY` |

`EVOLVE_ANTHROPIC_BASE_URL` is proxy-agnostic — it works with LiteLLM, corporate gateways, or any endpoint that speaks the Anthropic Messages API. It is **not** required for subscription auth.

To detect your active auth mode: `evolve doctor`

---

## Architecture Deep-Dives

The README is the surface. Real depth lives in `docs/`:

### Concepts (teaching-first)

| Doc | What it explains |
|---|---|
| [docs/concepts/overview.md](docs/concepts/overview.md) | The mental model: cycles, agents, artifacts |
| [docs/concepts/self-evolution.md](docs/concepts/self-evolution.md) | Reflexion-style learning + lesson lifecycle |
| [docs/concepts/trust-architecture.md](docs/concepts/trust-architecture.md) | 3-tier model + threat model + 5 gaming patterns |
| [docs/concepts/error-recovery.md](docs/concepts/error-recovery.md) | 4 recovery layers + operator commands |
| [docs/concepts/pluggability.md](docs/concepts/pluggability.md) | Persona / Skill / LLM swapping + adapter spec |

### Architecture (reference-first)

| Doc | What it specifies |
|---|---|
| [docs/architecture/phase-architecture.md](docs/architecture/phase-architecture.md) | Per-phase mechanics in detail |
| [docs/architecture/tri-layer.md](docs/architecture/tri-layer.md) | Skill / Persona / Command formal contract + Anti-Patterns A-D |
| [docs/architecture/egps-v10.md](docs/architecture/egps-v10.md) | EGPS predicate format + verdict computation + banned patterns |
| [docs/architecture/retrospective-pipeline.md](docs/architecture/retrospective-pipeline.md) | record-failure / merge-lesson script contracts |
| [docs/architecture/checkpoint-resume.md](docs/architecture/checkpoint-resume.md) | v9.1.0 durable-execution protocol |
| [docs/architecture/sequential-write-discipline.md](docs/architecture/sequential-write-discipline.md) | parallel_eligible rules + why writes are sequential |
| [docs/architecture/platform-compatibility.md](docs/architecture/platform-compatibility.md) | CLI support matrix + adapter contract |
| [docs/architecture/multi-llm-review.md](docs/architecture/multi-llm-review.md) | Why Auditor runs on a different model family |
| [docs/architecture/orchestrator-context-modes.md](docs/architecture/orchestrator-context-modes.md) | `context_mode` profile field; digest vs full + FAIL-path promotion (v10.10+) |
| [docs/architecture/psmas-phase-scheduling.md](docs/architecture/psmas-phase-scheduling.md) | Opt-in phase-skip foundation (v10.17+); precedence rule + ledger contract |
| [docs/architecture/dynamic-phase-routing.md](docs/architecture/dynamic-phase-routing.md) | Go routing kernel (v13.0.0/PR #4, default-off); stages, modes, integrity floor, LLM proposer |
| [docs/architecture/setup-onboarding.md](docs/architecture/setup-onboarding.md) | `/setup` onboarding: CLI detection, per-phase model proposal + validation, pipeline explainer |
| [docs/architecture/research-tool.md](docs/architecture/research-tool.md) | KB-first directive + research quota hook (v10.9 cycle 87-89) |
| [docs/architecture/token-economics-2026.md](docs/architecture/token-economics-2026.md) | P1-P8 token-reduction roadmap with per-phase cost attribution |
| [docs/architecture/acs-predicate-quality-gate.md](docs/architecture/acs-predicate-quality-gate.md) | Predicate-quality four-layer defense (cycles 80-86) |
| [docs/architecture/private-context-policy.md](docs/architecture/private-context-policy.md) | Two-folder content model (runtime vs developer-only) |
| [knowledge-base/README.md](knowledge-base/README.md) | Archival research bucket policy + promotion criteria |

### Incidents (case studies)

| Doc | What it documents |
|---|---|
| [docs/incidents/cycle-61.md](docs/incidents/cycle-61.md) | B0-B7 bugs from the gemini-3.1-pro-preview experiment (2026-05-15) |
| [docs/incidents/cycle-94-98-watchdog-overfiring.md](docs/incidents/cycle-94-98-watchdog-overfiring.md) | Watchdog post-memo SIGTERM cascade across v10.17.0 batch (2026-05-20); 5 cycles, 30-45 min operator overhead |
| [docs/incidents/cycle-102-111.md](docs/incidents/cycle-102-111.md) | Indirect reward hacking via confidence-cliff calibration |
| [docs/incidents/cycle-132-141.md](docs/incidents/cycle-132-141.md) | Orchestrator gaming via prose verdict drift |
| [docs/incidents/gemini-forgery.md](docs/incidents/gemini-forgery.md) | v7.9.0+ defenses for non-Claude CLIs |
| [docs/incidents/cycle-46-ship-refused.md](docs/incidents/cycle-46-ship-refused.md) | Ship-gate config drift incident |
| [knowledge-base/research/cycle-93-trust-kernel-breach-2026-05-20.md](knowledge-base/research/cycle-93-trust-kernel-breach-2026-05-20.md) | Trust-kernel ordering breach + same-day structural fix (v10.16.0) |

### Comparisons

| Doc | What it covers |
|---|---|
| [docs/comparisons/long-running-claude-skills.md](docs/comparisons/long-running-claude-skills.md) | Head-to-head: /goal, miles990, alirezarezvani, bejranonda, superpowers, OpenClaw, Hermes |

### ADRs (architecture decisions)

`docs/adr/0001-*.md` through `docs/adr/0007-*.md` — every architectural decision with context, choice, and consequence.

---

## Knowledge Base Stewardship

Two-surface model:

| Surface | Purpose | Visibility |
|---|---|---|
| `docs/` (runtime references) | Operationally needed; agents must see (e.g., incident postmortems, ADRs, architecture specs) | Public, tracked, agent-visible |
| `knowledge-base/research/` (archival dossiers) | Original research deep-dives; long-form citations and rationale | Public, tracked, **excluded from agent context** to keep prompts focused |

Everything learned, applied, or verified across cycles MUST land in one of those two surfaces. Memory entries (`~/.claude/projects/.../memory/`) are operator-scoped — mirror to `docs/` when team-shareable.

Research persistence rule: when a cycle's research is non-trivial (web searches, library evaluations, paper reviews), Scout records sources in `scout-report.md` AND the operator (or Scout itself if `EVOLVE_RESEARCH_CACHE_ENABLED=1` v9.X+) promotes the substance to `knowledge-base/research/<topic>.md`. The next cycle doesn't re-research.

See [docs/architecture/private-context-policy.md](docs/architecture/private-context-policy.md) for the full policy.

---

## Evolution Data

Active milestones (cycles that shipped substantive structural changes):

| Cycle | Version | Milestone |
|---|---|---|
| 1-9 | v3.0-v5.0 | Initial pipeline (Scout / Builder / Auditor) |
| 10-29 | v6.0-v7.0 | Trust kernel introduced (phase-gate, role-gate, ship-gate) |
| 30-39 | v8.0-v8.13 | EGPS v1: predicates introduced; ACS suite; mutation testing |
| 40-49 | v8.14-v8.30 | Adversarial Auditor mode; SHA-chained ledger; ship.sh integrity gate |
| 50 | v8.30 | Three-tier strictness model formalized |
| 51-60 | v8.32-v10.6 | Multi-CLI router; gemini + codex adapters; EGPS v10 (binary verdicts) |
| 61 | v10.6 | Gemini-3.1-pro-preview experiment → B0-B7 bug catalog |
| 62-63 | v10.7 | Structural fixes for B0-B7; scout grounding; audit citation binding; CLI Resolution renderer |
| 75 | v10.9 | Reward-hacking defense system Phase 0 (ADR-0012 5-layer hardening) |
| 75-78 | v10.10-v10.11 | Token-opt cold-move staging (move stable persona content to reference docs, -27% LoC on hot prompts) |
| 80-86 | v10.13-v10.15 | EGPS predicate-quality four-layer defense (grep-only linter, Auditor review section, mutation fail-gate, TDD-engineer default-on) |
| 87-89 | v10.14-v10.15 | Research-as-tool architecture: KB-first directive, research-quota-gate hook, KB-first widening across 6 personas |
| 90-93 | v10.15-v10.16 | Doc-deletion-guard hook, Knowledge Stewardship Rule, trust-kernel hardening (cycle 93 incident → tree-SHA verify + commit-SHA self-attestation) |
| 94-98 | v10.17 | Token-economics roadmap batch: P1 fast-fail retry, P2 auditor mastery gate, L1 orchestrator digest-by-default, P3 PSMAS phase-skip foundation |
| 100+ | v11+ | EGPS T1 auto-heal; phase-tracker observability; private-context policy |
| 132-141 | (incident) | Indirect-reward-hacking forensic series |

| Incident family | Count (lifetime) | Mechanism + Structural fix |
|---|---|---|
| Reward hacking (cycle 30-39, 102-111) | ~30 cycles | Predicates substituted for prose verdicts (EGPS v10); 5-layer defense (ADR-0012) |
| Orchestrator forgery (cycle 132-141) | ~10 cycles | Ledger SHA-chain + auto-rendered sections |
| Cross-CLI gaming (gemini-forgery 7.9.0+) | continuous | Artifact content checks + .sh write protection + anti-forgery prompt |
| Quota wall (cycle 11+) | ongoing | Checkpoint-resume (v9.1.0+) |
| Cycle-61 class (B0-B7) | 1 cycle = 8 bugs | 7 structural fixes shipped in cycles 62-63 of v10.7 |
| Trust-kernel ordering breach (cycle 93) | 1 cycle = 1 bug | Pre-merge tree-SHA verify + commit-SHA self-attestation (v10.16.0) |
| Watchdog post-memo SIGTERM (cycle 94-98) | 5 consecutive cycles | Heartbeat-touch (queued); short-term threshold raise shipped |

**61 lessons** in `.evolve/instincts/lessons/` as of v10.17 (2026-05-20). ~24 loaded into `state.json:instinctSummary[]` per cycle (deduplicated + decayed). Some pre-date v8 and still apply — `cycle-7-ephemeral-worktree-artifact.yaml` was authored 90+ cycles ago and still fires in Scout's prompt context.

### Release history (recent)

| Version | Date | Headline |
|---------|------|----------|
| v10.7 | 2026-05-13 | Cycle 61-63 structural fixes (B0-B7); Gemini-merged content; docs overhaul |
| v10.8 | 2026-05-16 | v10.6.0 trivial-skip activated; v8.63.0 anchor-mode context-builder; skills/ canonical (symlink flip) |
| v10.9 | 2026-05-17 | Reward-hacking defense system (ADR-0012); 5-layer hardening |
| v10.10 | 2026-05-17 | Token-opt Stage 2-9 cold-move staging |
| v10.11 | 2026-05-18 | Stage 10b Scout STOP CRITERION densification |
| v10.12 | 2026-05-18 | Cycle-isolation; orchestrator profile tightening |
| v10.13 | 2026-05-18 | Predicate-quality four-layer defense (cycle 80) |
| v10.14 | 2026-05-19 | Auditor + Builder persona trimming; subscription-auth doctor; proxy-agnostic `EVOLVE_ANTHROPIC_BASE_URL` |
| v10.15 | 2026-05-19 | Research-as-tool full stack (cycle 87-89); doc-stewardship hooks |
| v10.16 | 2026-05-20 | Trust-kernel hardening (cycle 93); pre-merge tree-SHA verify |
| v10.17 | 2026-05-20 | Token-economics roadmap batch (cycles 94-98): P1 + P2 + L1 + P3 foundation |
| v10.18 | May 21 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |
| v11.7 | May 24 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |
| v11.8 | May 24 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |
| v11.9 | May 24 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |
| v12.0 | May 24 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |
| v12.1 | May 25 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |
| v12.2 | May 26 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |
| v12.3 | May 26 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |
| v13.0 | May 27 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |
| v14.0 | May 31 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |
| v15.0 | Jun 2 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |
| v16.0 | Jun 2 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |
| v16.1 | Jun 2 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |
| v16.2 | Jun 3 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |
| v16.3 | Jun 3 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |
| v16.4 | Jun 3 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |
| v16.5 | Jun 4 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |
| v16.6 | Jun 6 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |
| v16.7 | Jun 7 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |
| v16.8 | Jun 7 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |
| v16.9 | Jun 7 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |
| v17.0 | Jun 7 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |
| v17.1 | Jun 7 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |
| v18.0 | Jun 8 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |
| v18.1 | Jun 8 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |
| v18.2 | Jun 9 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |
| v18.3 | Jun 10 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |
| v18.4 | Jun 11 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |
| v18.5 | Jun 11 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |
| v18.6 | Jun 12 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |
| v18.7 | Jun 13 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |
| v18.8 | Jun 14 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |
| v18.9 | Jun 14 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |
| v18.10 | Jun 14 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |

Per-version release notes: [docs/operations/release-notes/](docs/operations/release-notes/index.md). Full chronology: [CHANGELOG.md](CHANGELOG.md). Latest batch retrospective: [knowledge-base/research/v10-17-0-release-debrief.md](knowledge-base/research/v10-17-0-release-debrief.md).

---

## Project Structure

```
evolve-loop/
├── .claude-plugin/              # Plugin manifest + slash commands
│   ├── plugin.json              # Canonical version + components list
│   ├── marketplace.json         # Marketplace registry entry
│   └── commands/                # User-facing /commands (one per UX entry point)
├── .evolve/                     # Runtime state (mostly gitignored)
│   ├── state.json               # Authoritative cross-cycle state
│   ├── ledger.jsonl             # SHA-chained audit log
│   ├── llm_config.json          # Operator's per-phase LLM routing (gitignored)
│   ├── profiles/                # Per-agent capability profiles
│   ├── instincts/lessons/       # Reflexion-style lesson YAMLs (tracked)
│   ├── evals/                   # Eval definitions
│   ├── runs/cycle-N/            # Per-cycle workspace (gitignored)
│   ├── history/cycle-N/         # Post-ship archived workspaces
│   ├── worktrees/               # Per-cycle git worktrees (gitignored)
│   └── environment.json         # Capability detection results (gitignored)
├── acs/                         # ACS predicates
│   ├── cycle-N/                 # This cycle's predicates (gitignored until promoted)
│   └── regression-suite/cycle-*/  # Permanent regression predicates (tracked)
├── agents/                      # Persona files (tri-layer "who")
├── skills/                      # Skill workflows (tri-layer "how")
├── legacy/scripts/                     # Trust kernel + adapters + utilities
│   ├── dispatch/                # Dispatcher + subagent-run + router
│   ├── lifecycle/               # phase-gate.sh + ship.sh + cycle-state.sh
│   ├── guards/                  # PreToolUse shell hooks (Tier 1)
│   ├── cli_adapters/            # claude.sh / gemini.sh / codex.sh
│   ├── verification/            # validate-predicate.sh + verify-eval.sh
│   ├── observability/           # render-cli-resolution.sh + verify-ledger-chain.sh
│   ├── failure/                 # failure-adapter.sh
│   ├── utility/                 # release.sh + promote-acs.sh + probe-tool.sh
│   └── tests/                   # Integration + unit tests
├── docs/                        # Documentation (this directory tree)
│   ├── concepts/                # Teaching-first
│   ├── architecture/            # Reference-first
│   ├── comparisons/             # vs other projects
│   ├── incidents/               # Postmortems
│   ├── adr/                     # Architecture decisions
│   └── getting-started/         # Hands-on tutorials
├── knowledge-base/research/     # Research dossiers (tracked, agent-excluded)
├── bin/                         # Operator CLI shortcuts (status, cost, health, ...)
├── CLAUDE.md                    # Claude Code runtime contract
├── AGENTS.md                    # Universal CLI runtime contract
├── GEMINI.md                    # Gemini CLI runtime contract
├── README.md                    # This file
├── CHANGELOG.md                 # Per-version release notes
└── LICENSE
```

### Two-folder content model

| Folder | What goes there | Visible to agents? |
|---|---|---|
| `docs/` | Operationally needed runtime references | YES |
| `knowledge-base/` | Archival research and rationale | NO (excluded by CLI gate) |

Rationale: agents shouldn't read 200KB of research dossier to find one fact. Operationally-needed knowledge lives in `docs/`; long-form research lives in `knowledge-base/`. Cycle-level references (e.g., a recent incident postmortem) go to `docs/incidents/`. See [docs/architecture/private-context-policy.md](docs/architecture/private-context-policy.md).

---

## Requirements

- bash 3.2+ (macOS-default works; no bash-4 features used)
- git 2.5+ (for `git worktree`)
- jq 1.6+ (every state.json and ledger operation)
- One supported CLI: Claude Code 2.0+, Gemini CLI 0.42+, or Codex CLI
- ~200MB free disk (per-cycle worktrees + workspaces)

For installation help, see [docs/getting-started/your-first-cycle.md#prerequisites](docs/getting-started/your-first-cycle.md#prerequisites).

---

## Contributing

Contributions welcome. The project itself is run by evolve-loop — every commit on `main` is either:

1. A `--class manual` ship by an operator (with operator name + explicit message), OR
2. A `--class cycle` ship by an automated `/evolve-loop` cycle (with full ledger + audit trail)

Manual commits go through `/commit` (v13.0.0+) — it runs code-simplifier + code-reviewer + a language reviewer + lint + targeted tests, then writes a **commit-gate review attestation** that `evolve ship --class manual` verifies (matching the staged tree) before committing. `/release` wraps the self-healing version-bump → changelog → atomic-ship pipeline.

Read [CLAUDE.md](CLAUDE.md) for the runtime contract. The two-folder content model and the structural-fix-before-prose-fix preference apply to PRs too.

### Reporting incidents

If you find a gaming pattern the framework didn't catch, file an issue with:
- Cycle number and commit SHA
- The orchestrator-report.md + audit-report.md + acs-verdict.json
- What you expected vs what shipped

We treat framework-caught-its-own-bugs incidents (like cycle-61) as the most valuable kind of bug report.

---

## License & Links

- License: MIT — see [LICENSE](LICENSE)
- GitHub: [github.com/mickeyyaya/evolve-loop](https://github.com/mickeyyaya/evolve-loop)
- Marketplace: `/plugin marketplace add mickeyyaya/evolve-loop`
- CLAUDE.md (runtime contract): [CLAUDE.md](CLAUDE.md)
- CHANGELOG: [CHANGELOG.md](CHANGELOG.md)

### Citations

The framework's design draws on:

- **Reflexion** — Shinn et al. (2023) "Reflexion: Language Agents with Verbal Reinforcement Learning" arXiv:2303.11366
- **Double-loop learning** — Argyris, C. & Schon, D. (1978) *Organizational Learning: A Theory of Action Perspective*
- **Reward hacking limits** — Skalse et al. (NeurIPS 2022) "Defining and Characterizing Reward Hacking"
- **Mitigation survey** — Weng, L. (2024) "Reward Hacking in Reinforcement Learning" — Lil'Log
- **Tri-layer (Skill/Persona/Command)** — addyosmani/agent-skills (foundational inspiration)
- **Anthropic Secure Deployment Guide (2026)** — `--allowedTools` is "a permission gate, not a sandbox"

For a full bibliography, see [docs/architecture/phase-architecture-citations.md](docs/architecture/phase-architecture-citations.md).
