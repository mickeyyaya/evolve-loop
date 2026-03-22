# Orchestrator Policies & Configuration

## Graduated Instincts (confidence 0.9+)

1. **Inline S-tasks** (inst-007): S-complexity, <10 lines, clear eval → implement inline, skip Builder agent. Saves ~30-50K tokens.
2. **Grep-based evals** (inst-004): For markdown/shell projects, grep commands with expected match counts are effective eval gates.
3. **Meta-cycle** (every 5 cycles): Split-role critique + prompt evolution + mutation testing. See [phase6-metacycle.md](../phase6-metacycle.md).
4. **Gene library**: Reusable fix templates in `.evolve/genes/`. See [docs/genes.md](../../../docs/genes.md).

## Plan Template Caching

Match new tasks against `state.json.planCache` by type + file patterns (similarity > 0.7). Pass cached template as `priorPlan` to Builder. Store successful builds. Evict after 10 unused cycles. Saves ~30-50% tokens on repeated patterns.

## Token Budgets

- Per-task: 80K tokens (soft limit). Scout breaks large tasks.
- Per-cycle: 200K tokens (soft limit). Orchestrator halts if exceeded.
- M-tasks touching 10+ files likely exceed budget → split.

## Context Management

- After each cycle: write `handoff.md` checkpoint, output 5-8 line summary
- **Continue immediately** — never stop, never ask, never fabricate
- Lean mode (cycle 4+): read state.json once, use agent return summaries, skip full file re-reads
- AgentDiet compression between phases: prune expired context at each boundary

## Final Session Report

After all cycles, generate `final-report.md` with: summary narrative, task table, benchmark trajectory, learning stats, warnings. See SKILL.md for template.
