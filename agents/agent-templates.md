---
name: agent-templates
description: Base schemas and context blocks for all agents.
tools: []
---

# Agent Templates — Shared Schemas

> **v12.0.0 status:** `legacy/scripts/...` paths referenced below were removed in the v12 flag day. Subagent dispatch, ledger writes, and team-context coordination are now in-process in the Go orchestrator + `evolve subagent run` CLI. Treat bash snippets as contracts; do not invoke them directly.

Shared input/output schemas for evolve-loop agents. Each agent references this file instead of duplicating boilerplate. Agent-specific fields are documented in the individual agent files.

## Agent Definition Schema

Every agent file MUST include these frontmatter fields (in addition to `name`, `description`, `model`, `capabilities`, `tools`):

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `perspective` | string | **yes** | The evaluative lens through which this agent interprets every input. Sets the default bias for all judgments. Example: `"adversarial reviewer seeking failure modes"`. |
| `output-format` | string | **yes** | Canonical structure of this agent's primary artifact — file name + section list. Enables deterministic merging in parallel fan-out scenarios. Example: `"audit-report.md — Verdict, Defect Table, Eval Gate result"`. |

**Why these fields matter:** In parallel builder fan-out (multiple specialist builders per cycle), the Orchestrator uses `output-format` to identify merge semantics automatically. The `perspective` field is injected verbatim into the agent's system prompt preamble, replacing fragile inline persona instructions.

---

## Shared Context Block

All agents receive a JSON context block with these common fields:

| Field | Type | Description |
|-------|------|-------------|
| `cycle` | number | Current cycle number |
| `workspacePath` | string | Path to `.evolve/workspace/` |
| `strategy` | string | Evolution strategy: `balanced`, `innovate`, `harden`, `repair`, `ultrathink` |
| `challengeToken` | string | Per-cycle random hex token — embed in workspace output header and ledger entry |
| `instinctSummary` | array | Compact instinct array from state.json (inline) |

| `budgetRemaining` | object | Token/cycle budget awareness — see Budget-Aware Behavior below |

Agent-specific additions (e.g., `task`, `buildReport`, `mode`, `projectContext`) are documented in each agent file.

## Budget-Aware Behavior

Every agent receives a `budgetRemaining` object in context. Agents should adapt their behavior based on remaining resources — this is **not** a hard limit, but a signal for self-regulation. (Research basis: BATS framework [arXiv:2511.17006] — budget-aware agents self-regulate without additional training.)

```json
{
  "budgetRemaining": {
    "cyclesLeft": 7,
    "estimatedTokensLeft": 140000,
    "budgetPressure": "low|medium|high"
  }
}
```

| Pressure | Meaning | Agent Behavior |
|----------|---------|----------------|
| **low** | >60% budget remaining | Explore broadly, full analysis, comprehensive output |
| **medium** | 30-60% remaining | Focus on highest-priority items, trim verbose output |
| **high** | <30% remaining | Minimal output, skip optional sections, fastest path to completion |

The orchestrator computes `budgetPressure` at cycle start:
- `low`: `cyclesLeft / totalCycles > 0.6`
- `medium`: `cyclesLeft / totalCycles` between 0.3 and 0.6
- `high`: `cyclesLeft / totalCycles < 0.3`

Agents should **not** refuse to work under high pressure — they should work more efficiently. For example, Scout under high pressure selects 1-2 tasks instead of 3-4. Builder under high pressure skips alternative analysis in the design step.

## Strategy Handling

Adapt behavior based on the active `strategy` from context. See SKILL.md Strategy Presets table for definitions:

- **`balanced`** — standard approach, mixed focus
- **`innovate`** — prefer additive changes, relaxed on style
- **`harden`** — defensive coding, strict on all dimensions
- **`repair`** — fix-only, smallest diff, strict on regressions
- **`ultrathink`** — maximum reasoning budget, stepwise confidence

Each agent applies strategy to its own domain:
- **Scout:** adapts discovery scope and task selection priorities
- **TDD Engineer:** adapts test depth and coverage threshold enforcement
- **Builder:** adapts implementation approach and risk tolerance
- **Auditor:** adapts audit strictness and checklist depth

## Skill Awareness

Agents may receive `recommendedSkills` in their task context — a compact list of external skills (from installed plugins) that the orchestrator or Scout matched to the current task.

**Schema:**

```json
"recommendedSkills": [
  {"name": "everything-claude-code:security-review", "priority": "primary", "rationale": "security-type task"},
  {"name": "python-review-patterns", "priority": "supplementary", "rationale": "Python codebase"}
]
```

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Skill name as registered (e.g., `"everything-claude-code:security-review"`) |
| `priority` | string | `"primary"` (strongly relevant — invoke before design) or `"supplementary"` (nice to have — invoke only if needed) |
| `rationale` | string | Why this skill is recommended (under 50 chars) |

**Rules:**
- 0-3 skills per task (compact — adds ~200 tokens to context)
- Agents invoke via the `Skill` tool — invocation is **optional**, based on agent judgment
- Under budget pressure (medium/high): invoke at most 1 primary skill
- Skip supplementary skills if an applied instinct already covers the pattern
- Each invocation costs ~2-5K tokens

**Cross-platform & Fallbacks:** Use your platform's native skill invocation method (e.g., the `Skill` tool on Claude Code). On Gemini CLI / generic platforms, read the skill's `SKILL.md` file directly if available at the path in the skill inventory. If a specific recommended skill (e.g., `everything-claude-code:security-review`) is not found in the inventory, search the inventory for the closest available alternative in the same category.

## Shared Output Conventions

### Challenge Token

**Required header — applies to EVERY phase report** (scout-report.md, triage-report.md, test-report.md, build-report.md, audit-report.md, retro-report.md). Cycles 130/131/132/134 surfaced recurring CRITICAL audit failures when scout or builder omitted this header or used the wrong format; this section is the canonical contract.

**Exact format** — line 2 of every report file, immediately under the `# <Phase> Report — Cycle <N>` heading:

```markdown
<!-- challenge-token: <actual-token-value> -->
```

The auditor matches **case-sensitively** against the hyphenated lowercase form `challenge-token:` (NOT `Challenge:` or `Challenge-Token:`). A different casing OR a missing line fails the auditor's protocol check.

**Token source — read in this precedence order, fail loudly if all three miss:**

1. `inputs.challengeToken` from the agent context block (the canonical path; passed at phase launch).
2. `$WORKSPACE/challenge-token.txt` — read if context input is empty or absent.
3. **STOP and report `FAIL: challenge-token unavailable`.** Do NOT invent, placeholder, or substitute. Cycle 134's CRITICAL `C1` was scout using `no-token-manual-run-cycle-134` as a placeholder — the auditor treats placeholder values as forgery indicators (CRITICAL FAIL).

**Also include the token in your ledger entry** under `data.challenge` (same value, no comment syntax). Cross-references the report's header for hash-chain integrity.

**Worked example for `build-report.md`:**

```markdown
# Build Report — Cycle 134
<!-- challenge-token: 918dd68d5b81e0e3 -->

## Task: <slug>
...
```

### Ledger Entry

Every agent writes a structured ledger entry on completion. Common fields:

```json
{
  "ts": "<ISO-8601>",
  "cycle": "<N>",
  "role": "<scout|builder|auditor>",
  "type": "<discovery|build|audit>",
  "data": {
    "challenge": "<challengeToken>",
    "prevHash": "<hash of previous ledger entry>",
    "...": "<agent-specific fields>"
  }
}
```

Agent-specific `data` fields are defined in each agent file's Ledger Entry section.

### Mailbox Protocol

- **On start:** Read `workspace/agent-mailbox.md` for messages addressed to you (by role name) or `to: "all"`. Apply any hints, flags, or persistent warnings from prior agents.
- **On completion:** Post messages for other agents if you identified concerns worth carrying forward (e.g., fragile files, recurring smells, follow-up suggestions).
- Use `persistent: true` only for concerns spanning multiple cycles.

---

## Reflection Journal Schema

Every phase agent (Intent, Scout, Triage, Plan-Review, Build-Planner, TDD-Engineer, Tester, Builder, Auditor) appends a bounded `## Reflection` section to its report **and** writes a machine-readable `<phase>-reflection.yaml` sidecar. The learn phase consumes both surfaces: humans read the markdown, the reflector persona aggregates the YAML.

Gated on `EVOLVE_REFLECTION_JOURNAL` (default `1` at v10.20.0, opt-out via `0`). See [docs/architecture/reflection-journal.md](../docs/architecture/reflection-journal.md) for design rationale and rollout ladder.

### When to emit

Append the reflection **after** the phase's primary deliverable is complete (after `build-report.md`, `audit-report.md`, etc. have been written) but **before** posting the completion ledger entry. Use the same Write call sequence as the rest of the report — never delegate to a parallel sub-agent (single-writer invariant).

### Markdown section (operator-facing, ≤ 350 tokens)

Appended to the phase's primary report:

```markdown
## Reflection
<!-- reflection-version: 1 -->
<!-- BEGIN reflection -->

### What slowed this phase (required)
- Concrete bullet citing artifact path or tracker event. Empty only if `phase_smooth: true` asserted in the YAML companion AND backed by phase-tracker numbers.

### Pipeline friction received from upstream (required)
- Friction this phase received from a prior phase. Cite upstream artifact + anchor.

### Suggested improvement for next cycle (required, ≥1)
- Imperative voice. MUST cite positive evidence (artifact, log line, ledger entry).

### Self-acknowledged blind spots (optional)
- "I did not verify X because Y" — max one bullet.

### Reflection confidence (required)
- `confidence: 0.0–1.0` — grounded in artifacts, not vibes.

<!-- END reflection -->
```

The `<!-- BEGIN reflection -->` / `<!-- END reflection -->` anchors enable idempotent replacement on re-run (same pattern as `append-phase-perf.sh`).

### YAML sidecar (machine-readable)

Written to `$WORKSPACE/<phase>-reflection.yaml`. Flat, jq-greppable, ~30 lines.

```yaml
schema_version: 1
cycle: <N>
phase: <scout|tdd|build|audit|...>
agent: <evolve-scout|evolve-builder|...>
phase_smooth: <true|false>
slowdowns:
  - category: <slowdown-category>          # see enum below
    evidence: "<file-path>:line=<N>"       # required when category present
    severity: <low|medium|high>
friction_received_from:
  - upstream_phase: <phase-name>
    issue: "<short description>"
    evidence: "<file-path>#<anchor>"
suggested_improvements:
  - action: "<imperative-voice action>"    # e.g., "Bump kb-search quota to 30"
    target_file: "<path-to-file-to-change>"
    evidence_pointer: "<artifact-citation>"
    priority: <low|medium|high>
blind_spots:
  - "<one-line acknowledgement>"           # optional
reflection_confidence: <0.0-1.0>
phase_tracker_refs:                        # MUST come from .ephemeral/metrics/<phase>.json
  latency_ms: <int>
  cost_usd: <float>
  turns: <int>
```

### Slowdown category enum (closed set)

| Category | Use when |
|----------|----------|
| `research-quota` | A research/search tool refused calls due to quota |
| `tool-error` | A tool returned an error that required a workaround |
| `context-saturation` | Approaching token/turn cap forced abbreviated output |
| `ambiguous-input` | Upstream artifact was ambiguous and required interpretation |
| `tool-batching` | Too many serial tool calls that could have been parallelized |
| `profile-restriction` | Permission profile blocked an action that turned out necessary |
| `other` | None of the above — REQUIRES a free-form note in `evidence` |

Cross-cycle aggregation tallies by these categories; using `other` undermines the rollup, so reach for it only when nothing else fits.

### Anti-sycophancy rule (verbatim)

Include this directive in every phase agent's reflection authoring instructions:

> A reflection is NOT a status report. "Phase went smoothly" is only acceptable when `phase_smooth: true` is asserted AND `phase_tracker_refs` shows no over-budget signal (cost ≤ baseline × 1.1, turns ≤ profile max). Otherwise you MUST cite at least one slowdown with artifact evidence. Affirmation without evidence is a `reflection-sycophancy` defect the Auditor flags MEDIUM (advisory, non-blocking).

### Reusing phase-tracker data

The `phase_tracker_refs` block MUST be read from `.evolve/runs/cycle-<N>/.ephemeral/metrics/<phase>.json` (already produced by `legacy/scripts/observability/rollup-cycle-metrics.sh`). Do not recompute timing or cost — single source of truth.

### Aggregation surface

- **Per-cycle:** the new `evolve-reflector` agent reads every `<phase>-reflection.yaml` in the cycle dir and emits `learn/reflector-synthesis.md`.
- **Cross-cycle:** the reflector calls `legacy/scripts/observability/aggregate-reflections.sh --window 5` for a 5-cycle rollup (slowdown categories tallied, upstream friction sources, recurring suggestions).
- **Operator view:** `legacy/scripts/observability/dashboard.sh` displays a one-line "Recent reflection hot-spots" summary sourced from the aggregator's `--format=json` mode.

---

## Pipeline Agents

The full evolve-loop pipeline and the agent responsible for each phase. **Note (v10.20.0+):** the Learn phase is formally an umbrella containing the reflector + retrospective (FAIL/WARN) + memo (PASS); all three consume the per-phase reflection YAMLs.

| Phase | Agent | File | Output Artifact |
|-------|-------|------|-----------------|
| Calibrate | Orchestrator | `evolve-orchestrator.md` | cycle-state.json |
| Intent (opt-in) | Intent | `evolve-intent.md` | `intent.md` + `intent-reflection.yaml` |
| Research / Discover | Scout | `evolve-scout.md` | `scout-report.md` + `scout-reflection.yaml` |
| Triage | Triage | `evolve-triage.md` | `triage-decision.md` + `triage-reflection.yaml` |
| Plan Review (opt-in) | Plan Reviewer | `plan-reviewer.md` | `plan-review.md` + `plan-review-reflection.yaml` |
| Build Planner (rollout) | Build Planner | `evolve-build-planner.md` | `build-plan.md` + `build-planner-reflection.yaml` |
| Test Contract (TDD) | TDD Engineer | `evolve-tdd-engineer.md` | `test-report.md` + `tdd-reflection.yaml` |
| EGPS Tester | Tester | `evolve-tester.md` | `tester-report.md` + `tester-reflection.yaml` |
| Build | Builder | `evolve-builder.md` | `build-report.md` + `build-reflection.yaml` |
| Audit | Auditor | `evolve-auditor.md` | `audit-report.md` + `audit-reflection.yaml` |
| Ship | Orchestrator / ship.sh | `evolve-orchestrator.md` | commit SHA |
| Learn — Reflector | Reflector | `evolve-reflector.md` | `learn/reflector-synthesis.md` |
| Learn — Retrospective (FAIL/WARN) | Retrospective | `evolve-retrospective.md` | `retrospective-report.md` + lesson YAMLs |
| Learn — Memo (PASS) | Memo | `evolve-memo.md` | `memo.md` + carryoverTodos |

**TDD Engineer contract:** Runs after Scout selects a task and before Builder implements. Writes failing tests that encode acceptance criteria (RED phase). Builder must make those tests pass without modifying them. See [evolve-tdd-engineer.md](evolve-tdd-engineer.md) for the full workflow.

**Phase sequence enforcement:** `phase-gate-precondition.sh` blocks out-of-order agent invocations. The TDD engineer phase (`tdd`) must be advanced via `cycle-state.sh advance tdd tdd-engineer` before Builder can be invoked.

**Learn phase invocation:** `legacy/scripts/lifecycle/run-cycle.sh` invokes the reflector after Ship completes, then dispatches retrospective (FAIL/WARN) or memo (PASS) based on the audit verdict. The reflector runs on every cycle regardless of verdict.

---

## Team Context Bus

A human-readable shared narrative document at `.evolve/runs/cycle-N/team-context.md`. Replaces fragile JSON handoffs between phases — every pipeline agent appends a section before exiting; the next agent reads the whole bus before starting.

### Sections (canonical order)

| Section | Populated by | Purpose |
|---------|--------------|---------|
| Goal | Orchestrator (during init) | The user's instruction in their own words |
| Scout Findings | Scout | Selected task, acceptance criteria, research sources |
| TDD Contract | TDD Engineer | Test files written, RED evidence, exit criteria for Builder |
| Build Report | Builder | Files modified, test pass evidence, deviations from contract |
| Audit Verdict | Auditor | PASS/WARN/FAIL with evidence; defects table if non-PASS |

### Protocol

- **On start:** Read `.evolve/runs/cycle-<N>/team-context.md` in its entirety. Other agents' sections are your context — do not duplicate their work.
- **On completion:** Append your section via `bash legacy/scripts/utility/team-context.sh append <cycle> <workspace> <role> <body-file>`. Idempotent — re-running replaces your section's body without duplicating.
- **Verification:** `bash legacy/scripts/utility/team-context.sh verify <cycle> <workspace> --require scout,tdd-engineer,builder,auditor` exits non-zero if any required section is empty (still `_pending_`).

### Phase-gate hook (opt-in)

When `EVOLVE_REQUIRE_TEAM_CONTEXT=1` is exported in the dispatcher environment, `phase-gate-precondition.sh` blocks Builder invocations until both Scout's and TDD-Engineer's sections are populated in the bus. Default off for backward compatibility with cycles that predate the bus.

See [legacy/scripts/utility/team-context.sh](../legacy/scripts/utility/team-context.sh) for the implementation.
