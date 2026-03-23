# Instinct System

The evolve-loop learns from each cycle by extracting **instincts** — specific, actionable patterns discovered during development. Instincts prevent the loop from repeating mistakes and reinforce successful approaches.

## How It Works

During Phase 5 (LEARN), the orchestrator analyzes the cycle's artifacts — what was built, what passed/failed audit, what approaches worked — and extracts instincts with deep reasoning. Each instinct captures a single pattern with a confidence score.

Instincts are stored as YAML files in `.evolve/instincts/personal/` and read by the Scout and Builder at the start of each cycle.

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

## Instinct Lifecycle Gates

Three distinct gates govern instinct advancement. Each serves a different purpose and has different thresholds — do not confuse them:

| Gate | Purpose | Confidence | Cycle Confirmations | Additional Requirements |
|------|---------|------------|--------------------|-----------------------|
| **Graduation** | Instinct → mandatory Builder guidance | >= 0.75 | 3+ distinct cycles | No contradictions in `failedApproaches` |
| **Global Promotion** | Project → cross-project sharing | >= 0.8 | 2+ cycles | Loop age 5+ cycles; must be generalizable |
| **Trust Governance** | External → accepted instinct | >= 0.8 | 3 confirmations | Provenance check; no eval/prompt refs |

**Why different thresholds:** Graduation is stricter on cycle count (3+) because graduated instincts bypass Builder deliberation — a wrong one directly causes failures. Promotion has a lower cycle bar (2+) because promoted instincts are still evaluated before application, but requires higher confidence (0.8) and loop maturity (5+ cycles). Trust governance is strictest on provenance because external instincts carry injection risk (arXiv:2602.12430: 26.1% community skills have vulnerabilities).

See [phase5-learn.md](../skills/evolve-loop/phase5-learn.md) § Instinct Graduation for the canonical executable specification.

## Global Promotion

After 5+ cycles of loop maturity, instincts with confidence >= 0.8 can be promoted to global scope (`~/.evolve/instincts/personal/`), making them available across all projects. The "5+ cycles" refers to the loop's total age, not the instinct's age — this prevents promoting instincts from an immature loop.

**Promotion criteria:**
- Confidence >= 0.8
- Confirmed across 2+ cycles
- Not project-specific (must be generalizable)

**Promotion process:**
1. Copy the instinct YAML entry to `~/.evolve/instincts/personal/<instinct-id>.yaml`
2. Add `promotedFrom: "<project-name>/cycle-<N>"` field
3. Keep the original in the project's instincts directory (source of truth)
4. The global copy is read by all projects' evolve-loop instances

## Graduation

High-confidence instincts that have been repeatedly confirmed graduate to **mandatory guidance** status. Graduated instincts are applied automatically by the Builder without the usual "should I apply this?" evaluation.

### Graduation Threshold

An instinct graduates when **all three conditions** are met:

| Condition | Requirement |
|-----------|-------------|
| Confidence | >= 0.75 |
| Cross-cycle confirmation | Cited in `instinctsApplied` by Scout or Builder in 3+ distinct cycles |
| No contradictions | Not contradicted by any entry in `state.json failedApproaches` |

Note: Graduation (3+ cycles, >= 0.75) is distinct from global promotion (2+ cycles, >= 0.8). See § Instinct Lifecycle Gates above.

### Operational Effects

- The instinct's `graduated: true` flag is set in `instinctSummary`
- Builder treats graduated instincts as **mandatory** — applies them directly without deliberation
- Scout lists graduated instincts first in context, giving them attention priority
- Each subsequent citation boosts confidence by +0.1 (capped at 1.0)
- Graduated instincts are candidates for global promotion to `~/.evolve/instincts/personal/`

### Reversal

Graduation can be reverted when evidence contradicts the instinct:

- **Trigger:** 2+ consecutive build failures where the graduated instinct was applied
- **Action:** Set `graduated: false`, reduce confidence by 0.2
- **Escalation:** If confidence drops below 0.5 after reversal, the instinct is archived with `archivedReason: "reversal"`
- **Logging:** Reversal is recorded in the ledger as `type: "instinct-reversal"`

See [phase5-learn.md](../skills/evolve-loop/phase5-learn.md) § Instinct Graduation for the full specification.

## Memory Operations

Three distinct memory operations run at different cadences. They are complementary, not alternatives:

| Operation | Frequency | Window | Action |
|-----------|-----------|--------|--------|
| **Dormant flagging** | Continuous (per-cycle Scout check) | 3+ cycles uncited | Soft signal: Scout notes dormant instincts in report for task selection |
| **Consolidation + temporal decay** | Every 3 cycles (or instinctCount > 20) | Last 5 cycles unreferenced | Cluster, decay (-0.1 confidence/pass), archive below 0.3 |
| **Forgetting (zero-use discard)** | Every 10 cycles | Last 10 cycles with 0 citations | Move to `archived/` after causal review |

**Why a 5-cycle decay window (not 3):** A new instinct created in cycle N needs at least 2 consolidation passes before decay kicks in. This matches the 2+ cycle confirmation needed for promotion — an instinct that would be promoted should not be simultaneously decayed.

### Consolidation (every 3 cycles)

Every 3 cycles (or when instinct count exceeds 20), the orchestrator consolidates memory:

1. **Cluster:** Merge instincts with >85% semantic similarity into higher-level abstractions
2. **Archive:** Superseded instincts move to `archived/` (never deleted)
3. **Temporal decay:** Instincts unreferenced in the last 5 cycles lose 0.1 confidence per pass; below 0.3 → archived as stale
4. **Entropy gating:** New instincts >90% similar to existing ones update the existing instinct instead of creating duplicates

This prevents unbounded memory growth and keeps the instinct set relevant and compact.

## File Organization

```
.evolve/instincts/
  personal/
    cycle-1-instincts.yaml    # instincts from cycle 1
    cycle-2-instincts.yaml    # instincts from cycle 2
    ...
  archived/
    merged-instincts.yaml     # superseded instincts (provenance preserved)
    stale-instincts.yaml      # decayed below 0.3 confidence
```

Each cycle appends a new file. Instinct updates (confidence changes) reference the original ID with a `-update` suffix.

For an annotated example, see [examples/instinct-example.yaml](../examples/instinct-example.yaml).

## How Agents Use Instincts

- **Scout** reads all instincts before scanning. Applies relevant patterns to avoid re-discovering known issues and to focus on areas where past patterns suggest problems.
- **Builder** reads instincts before implementing. Follows successful patterns and avoids anti-patterns.
- **Orchestrator** reads instincts during LEARN phase to update confidence scores and extract new ones.

## Instinct Forgetting and Consolidation (Memory Research-Inspired)

Research on agent memory operations (arXiv:2505.00675, arXiv:2603.07670) identifies six core memory operations. The evolve-loop's instinct store implements Store, Retrieve, and Update — but lacks **Forgetting** (strategic discard) and robust **Consolidation** (continual compression). Over 100+ cycles, zero-use instincts accumulate and degrade retrieval quality.

**Forgetting protocol (every 10 cycles):**

| Step | Action | Threshold |
|------|--------|-----------|
| Usage scan | Count instinct citations in ledger for last 10 cycles | — |
| Candidate selection | Instincts with 0 citations in last 10 cycles | `usageFrequency == 0` |
| Staleness check | Candidate confidence already < 0.4 | Automatic discard |
| Causal review | Check if instinct was learned from a still-relevant failure | Retain if causal link active |
| Discard | Move to `archived/` with `archivedReason: "zero-use-discard"` | Provenance preserved |
| Merge candidates | Instincts with >85% similarity but different confidence | Merge, keep higher confidence |

**Graduation interaction:** Graduated instincts (confidence >= 0.75, cited 3+ cycles) are exempt from forgetting — they have proven their value. Non-graduated instincts with zero use are the primary discard targets.

**Anti-pattern:** Discarding instincts that are rarely used but prevent critical failures. The causal review step catches this: if an instinct's `source` cycle had a CRITICAL audit finding, the instinct is retained regardless of usage frequency.

## Viewing Instincts

To inspect current instincts:

```bash
cat .evolve/instincts/personal/*.yaml
```

To see instinct count and history:

```bash
cat .evolve/state.json | python3 -c "import sys,json; d=json.load(sys.stdin); print(f'Instincts: {d[\"instinctCount\"]}')"
```
