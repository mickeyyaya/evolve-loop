# Instinct System

The evolve-loop learns from each cycle by extracting **instincts** — specific, actionable patterns discovered during development. Instincts prevent the loop from repeating mistakes and reinforce successful approaches.

## How It Works

During Phase 5 (LEARN), the orchestrator analyzes the cycle's artifacts — what was built, what passed/failed audit, what approaches worked — and extracts instincts with deep reasoning. Each instinct captures a single pattern with a confidence score.

Instincts are stored as YAML files in `.claude/evolve/instincts/personal/` and read by the Scout and Builder at the start of each cycle.

## Schema

```yaml
- id: inst-001
  pattern: "short-identifier"
  description: "Specific, actionable description of what to do or avoid. Include context about why this matters and when it applies."
  confidence: 0.7      # 0.5 (new) to 1.0 (proven)
  source: "cycle-N/task-slug"
  type: "anti-pattern"  # see Types section for full list
  category: "episodic"  # episodic, semantic, or procedural
```

### Fields

| Field | Description |
|-------|-------------|
| `id` | Unique identifier (inst-NNN) |
| `pattern` | Short kebab-case name for the pattern |
| `description` | What to do/avoid and why. Must be specific enough to act on. |
| `confidence` | Starts at 0.5-0.6 for new instincts. Increases when confirmed across cycles. |
| `source` | Which cycle and task produced this instinct |
| `type` | Category: `anti-pattern`, `successful-pattern`, `convention`, `architecture`, `process` |

### Types

Instincts are organized into three memory categories for targeted retrieval:

#### Episodic (what happened)
- **anti-pattern** — something that failed or caused problems; avoid this
- **successful-pattern** — an approach that worked well; repeat this

#### Semantic (domain knowledge)
- **convention** — a naming/format/structure rule to follow consistently
- **architecture** — a structural decision about the system
- **domain** — codebase-specific knowledge (e.g., "this API uses camelCase for JSON keys")

#### Procedural (how-to)
- **process** — a workflow optimization for the loop itself
- **technique** — a specific implementation technique that works in this codebase (e.g., "use barrel exports in index.ts")

### Memory Categories

Each instinct belongs to one of three memory categories. Agents use categories for targeted retrieval:

| Category | Contains | When to Query |
|----------|----------|---------------|
| **Episodic** | Past experiences — what worked, what failed | Before selecting approaches, to avoid repeating failures |
| **Semantic** | Domain knowledge — conventions, architecture, codebase facts | Before implementing, to follow existing patterns |
| **Procedural** | How-to knowledge — techniques, process optimizations | During implementation, for specific techniques |

Agents should query the relevant category based on their current phase:
- **Scout** → episodic (what failed before) + semantic (conventions to maintain)
- **Builder** → semantic (existing patterns) + procedural (how to implement)
- **Auditor** → semantic (conventions to enforce) + episodic (past issues to check for)

## Confidence Scoring

| Score | Meaning |
|-------|---------|
| 0.5 | New, single observation |
| 0.6 | Likely correct, seen once with clear evidence |
| 0.7 | Confirmed by outcome (task passed audit) |
| 0.8 | Confirmed across 2+ cycles |
| 0.9 | Proven reliable, no contradictions |
| 1.0 | Fundamental truth, always applies |

Confidence increases when an instinct is confirmed in a later cycle (e.g., the pattern still holds, the anti-pattern was correctly avoided). Confidence decreases if a pattern is contradicted.

## Promotion

After 5+ cycles, instincts with confidence >= 0.8 can be promoted to global scope (`~/.claude/homunculus/instincts/personal/`), making them available across all projects.

## File Organization

```
.claude/evolve/instincts/
  personal/
    cycle-1-instincts.yaml    # instincts from cycle 1
    cycle-2-instincts.yaml    # instincts from cycle 2
    ...
```

Each cycle appends a new file. Instinct updates (confidence changes) reference the original ID with a `-update` suffix.

## How Agents Use Instincts

- **Scout** reads all instincts before scanning. Applies relevant patterns to avoid re-discovering known issues and to focus on areas where past patterns suggest problems.
- **Builder** reads instincts before implementing. Follows successful patterns and avoids anti-patterns.
- **Orchestrator** reads instincts during LEARN phase to update confidence scores and extract new ones.

## Viewing Instincts

To inspect current instincts:

```bash
cat .claude/evolve/instincts/personal/*.yaml
```

To see instinct count and history:

```bash
cat .claude/evolve/state.json | python3 -c "import sys,json; d=json.load(sys.stdin); print(f'Instincts: {d[\"instinctCount\"]}')"
```
