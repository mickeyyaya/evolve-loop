# Island Model Evolution

For advanced use cases, the evolve-loop can maintain multiple independent "islands" — parallel configurations evolving separately with periodic migration of best-performing traits.

## Concept

Instead of a single linear improvement path, maintain 3-5 independent evolve-loop configurations. Each island has its own:
- Strategy preset
- Instinct set
- Token budget
- Agent prompt variations

Periodically (every 5 cycles), the best-performing traits migrate between islands.

## How It Works

### Setup
Create island directories under `.evolve/islands/`:
```
.evolve/islands/
  island-1/    # balanced strategy, standard budgets
  island-2/    # innovate strategy, higher budgets
  island-3/    # harden strategy, strict budgets
```

Each island contains its own `state.json`, `instincts/`, and `evalHistory`.

### Independent Evolution
Each island evolves independently using the standard 5-phase cycle. Islands can be run:
- **Sequentially** — one island per session (most practical)
- **In parallel** — using separate worktrees (advanced)

### Migration (every 5 cycles)
After 5 cycles, compare island performance using delta metrics:

1. **Rank islands** by composite fitness score:
   ```
   fitness = 0.4 * successRate + 0.3 * (1 - avgAuditIterations/3) + 0.2 * instinctQuality + 0.1 * novelty
   ```

2. **Migrate top traits** from the best island to others:
   - High-confidence instincts (>0.8)
   - Successful genes
   - Plan templates with high reuse counts

3. **Preserve diversity** — never migrate more than 30% of an island's traits from another island. Diversity is the point.

### When to Use Islands

- Large codebases with distinct subsystems (frontend, backend, infra)
- Experimenting with different strategies simultaneously
- When single-strategy evolution has stagnated

### Practical Usage

```bash
# Run island 1 for 3 cycles
/evolve-loop 3 --island 1

# Run island 2 for 3 cycles
/evolve-loop 3 innovate --island 2

# Trigger migration between islands
/evolve-loop --migrate
```

## Limitations

- Higher total cost (3-5x for 3-5 islands)
- Requires more disk space for parallel state
- Migration conflicts possible (contradictory instincts)
- Best suited for projects with 20+ cycles of history

See [self-learning.md](self-learning.md) § Meta-Cycle Review for the single-configuration self-improvement mechanisms that islands build upon.
