---
name: evolve-intent
description: Pre-Scout intent capture phase. Structures vague user goals into intent.md before any subagent budget is spent. Opt-in via EVOLVE_REQUIRE_INTENT=1.
---

# evolve-intent

> Sprint 4 composable skill (v8.19.0+). Wraps the new Intent phase that runs between Calibrate and Research when `EVOLVE_REQUIRE_INTENT=1`. Single-persona, autonomy-preserving — no human checkpoint mid-cycle.

## When to invoke

- **Autonomous**: orchestrator advances to phase=intent when `cycle-state.intent_required==true` (set at cycle init from `EVOLVE_REQUIRE_INTENT=1`)
- **User-driven**: `/intent` slash command, before `/loop`, to lock in structured intent
- **Re-run**: re-invoke to replace prior intent.md within the same cycle (kernel accepts latest ledger entry)

## When NOT to invoke

- After Scout has already run for this cycle (intent must precede research)
- For pure-execution cycles where the goal is fully specified (e.g., "fix typo at file.md:42") — but in practice, even those benefit from the explicit `non_goals` and `acceptance_checks`
- When `EVOLVE_REQUIRE_INTENT` is unset and the user has not explicitly invoked `/intent` — default flow skips this phase

## Workflow

| Step | Action | Exit criteria |
|---|---|---|
| 1 | Read user goal + workspace + priorIntent (if re-run) | Inputs gathered |
| 2 | Classify input via AwN framework (IMKI/IMR/IwE/IBTC/CLEAR) | Single class chosen |
| 3 | If awn_class == IBTC: emit short scope-rejection intent.md | gate_intent_to_research will short-circuit cycle |
| 4 | Otherwise: produce 8-field structured intent.md | Frontmatter complete + body written |
| 5 | Surface ≥1 challenged_premise — assumption to actively question | premise/challenge/alternative populated |
| 6 | Write `$workspace/intent.md` | File present, parseable |
| 7 | Subagent runner writes ledger entry binding intent.md SHA to cycle | `kind: "agent_subprocess"`, `role: "intent"` ledger entry |

## Autonomy invariant

This skill MUST NOT block on human approval. The intent persona produces intent.md; `gate_intent_to_research` verifies structure (≥1 challenged_premise, valid awn_class, SHA matches ledger); cycle proceeds. There is no `accept-intent` operator command — re-running `/intent` replaces the prior file, and the kernel uses the latest ledger entry. This is the difference between an autonomy-preserving filter and a checkpoint.

## Cycle-binding

Like every other agent, intent.md gets a ledger entry with `(artifact_sha256, git_head, tree_state_sha, challenge_token)`. ship.sh's downstream tree-state check works against the same SHA-binding the auditor uses.

## Composition

Invoked by:
- `/intent` slash command (user-driven)
- `evolve-orchestrator` macro when `cycle-state.intent_required==true`

Cannot be:
- Invoked by another persona (Anti-pattern B per `docs/architecture/tri-layer.md`)
- Fanned out (single perspective; no `parallel_subtasks` in profile for v0.1)
- Substituted via `EVOLVE_BYPASS_PHASE_GATE=1` (CRITICAL violation per CLAUDE.md)

## Reference

- `agents/evolve-intent.md` (persona)
- `.evolve/profiles/intent.json` (permission profile)
- `.claude-plugin/commands/intent.md` (slash command)
- `scripts/lifecycle/phase-gate.sh` (`gate_calibrate_to_intent`, `gate_intent_to_research`)
- `scripts/guards/phase-gate-precondition.sh` (scout-blocked-without-intent enforcement)
- `.evolve/research/intent-capture-patterns.md` (5-source research grounding)
- `docs/architecture/intent-phase.md` (full architecture)
