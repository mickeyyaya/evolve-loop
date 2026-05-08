> **Tri-Layer Architecture (Sprint 3, v8.16+)** — Formal contract for the Skill / Persona / Command separation in evolve-loop. Read this before authoring a new skill, persona, or slash command.

## Table of Contents

1. [The Three Layers](#the-three-layers)
2. [Endorsed Orchestration Patterns](#endorsed-orchestration-patterns)
3. [Anti-Patterns (Forbidden)](#anti-patterns-forbidden)
4. [How the Trust Kernel Fits](#how-the-trust-kernel-fits)
5. [Authoring Rules](#authoring-rules)
6. [Composition Decision Flow](#composition-decision-flow)

---

## The Three Layers

| Layer | Job | File location | Example |
|---|---|---|---|
| **Skill** | Workflow + steps + exit criteria — *the how* | `skills/<name>/SKILL.md` | `evolve-tdd`, `evolve-audit` |
| **Persona** | One role, one perspective, one output format — *the who* | `agents/<role>.md` | `evolve-scout`, `plan-reviewer` |
| **Command** | User-facing entry point — *the when* (the orchestration layer) | `.claude-plugin/commands/<name>.md` | `/scout`, `/audit`, `/loop` |

**The governing rule** (inherited from `addyosmani/agent-skills`): the user (or a slash command) is the orchestrator. **Personas do not invoke other personas.** Skills are mandatory hops inside a persona's workflow.

This is not just a convention — Claude Code enforces it at runtime: *"subagents cannot spawn other subagents."*

---

## Endorsed Orchestration Patterns

| # | Pattern | When to use | Example in evolve-loop |
|---|---|---|---|
| 1 | **Direct invocation** | One persona, one perspective, one artifact | `/audit` runs auditor |
| 2 | **Single-persona slash command** | Repeating direct invocation with saved setup | `/scout`, `/build` |
| 3 | **Parallel fan-out + merge** | Independent sub-tasks that share an input but produce different perspectives | `/scout` fans out to scout-codebase + scout-research + scout-evals (Sprint 1.1) |
| 4 | **Sequential pipeline as user-driven slash commands** | Lifecycle with dependencies + human checkpoints | `/scout → /plan-review → /tdd → /build → /audit → /ship → /retro` |
| 5 | **Auto-orchestrated macro** (the `/loop` exception) | Autonomous mode: full lifecycle without human checkpoints | `/loop` runs the entire sequence; trust kernel enforces phase ordering at script layer |

Pattern 5 is **specific to evolve-loop** because the trust kernel (sandbox, ledger SHA, phase-gate) substitutes for the human checkpoints addyosmani's framework relies on. Without our kernel, auto-orchestration would be Anti-pattern C below.

---

## Anti-Patterns (Forbidden)

| # | Name | Why it fails |
|---|---|---|
| A | **Router persona** ("decide which persona to call") | Pure routing layer with no domain value; replicates work that slash commands + intent mapping already do |
| B | **Persona-calls-persona** | Defeats single-perspective design; failure modes multiply; platform-blocked at runtime |
| C | **Sequential orchestrator that paraphrases** | Loses human checkpoints; accumulated drift via summarization; doubles tokens |
| D | **Deep persona trees** | Each layer adds latency and tokens with no decision value |

evolve-loop's `/loop` is *not* Anti-pattern C because the trust kernel binds artifacts SHA-by-SHA — there is no paraphrasing at handoff, only deterministic ledger-verified passing.

---

## How the Trust Kernel Fits

The trust kernel operates at **script layer**, *below* skills/personas/commands:

```
┌────────────────────────────────────────────────────┐
│  Commands (.claude-plugin/commands/*.md)           │  USER-FACING
├────────────────────────────────────────────────────┤
│  Personas (agents/*.md)                            │  ROLES
├────────────────────────────────────────────────────┤
│  Skills (skills/*/SKILL.md)                        │  WORKFLOWS
├────────────────────────────────────────────────────┤
│  Trust Kernel:                                     │  PROTECTED
│   - scripts/dispatch/subagent-run.sh (only LLM entry)       │
│   - scripts/lifecycle/phase-gate.sh (artifact verifier)      │
│   - scripts/guards/phase-gate-precondition.sh      │
│   - scripts/lifecycle/ship.sh (atomic ship)                  │
│   - OS sandbox (sandbox-exec / bwrap)              │
│   - SHA256 ledger binding                          │
└────────────────────────────────────────────────────┘
```

A new skill, persona, or command **cannot weaken the kernel** because:
1. Every LLM invocation routes through `subagent-run.sh` (CLAUDE.md rule 5)
2. `subagent-run.sh` enforces per-profile sandbox + permission allowlist
3. Each artifact is bound to the cycle via SHA256 + git tree state in the ledger
4. `phase-gate-precondition.sh` is a PreToolUse hook — no skill or command can bypass it

This means Sprint 3's tri-layer refactor only touches the user-facing API; the kernel is unchanged.

---

## Authoring Rules

### A new Skill must:

- Live at `skills/<skill-name>/SKILL.md`
- Have YAML frontmatter with `name` and `description`
- Define a workflow (steps + exit criteria) — not invoke a persona
- Be invokable via Claude Code's `Skill` tool

### A new Persona must:

- Live at `agents/<role>.md` (or `agents/evolve-<role>.md` for evolve-loop personas)
- Have YAML frontmatter with `name`, `description`, `tools` (least-privilege)
- Define one role with one perspective and one output format
- End with a **Composition** block:
  ```
  ## Composition
  - Invoke directly when: <condition>
  - Invoke via: <command(s)>
  - Do not invoke from another persona.
  ```

### A new Command must:

- Live at `.claude-plugin/commands/<command-name>.md`
- Have YAML frontmatter with `description`
- Compose personas and skills (the orchestration layer)
- For Pattern 3 (fan-out): issue multiple `Agent` tool calls in **one** assistant turn (sequential turns serialize)

### Forbidden:

- A persona that invokes another persona (any depth)
- A "router" persona whose only job is to decide which persona to call
- A skill that bypasses `subagent-run.sh` to directly call `claude -p` / `gemini` / `codex`

---

## Composition Decision Flow

```
Is the work one perspective on one artifact?
├── Yes → Direct persona invocation. Stop.
└── No  → Will the same composition repeat?
         ├── No  → Direct invocation, ad hoc. Stop.
         └── Yes → Are sub-tasks independent (no shared mutable state, no ordering)?
                  ├── No  → Sequential slash commands run by user (Pattern 4),
                  │         OR auto-orchestrated macro under trust kernel (Pattern 5).
                  └── Yes → Parallel fan-out with merge (Pattern 3).
                           Implement via subagent-run.sh dispatch-parallel +
                           profile.parallel_subtasks + aggregator.sh.
```

---

## Prior art

- **addyosmani/agent-skills** — `references/orchestration-patterns.md` is the source of patterns 1–5 and anti-patterns A–D.
- **garrytan/gstack** — `/autoplan` showed multi-lens plan review (CEO + Eng + Design + DX); informed Sprint 2's `plan-reviewer`.
- **Claude Code platform docs** — *"subagents cannot spawn other subagents"* enforces Anti-pattern B at runtime.
