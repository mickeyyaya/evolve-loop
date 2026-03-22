> Read this file when running the meta-cycle (every 5 cycles). Covers split-role critique, agent effectiveness review, prompt evolution, skill synthesis, mutation testing, and topology review.

## Contents
- [Collecting Metrics](#collecting-metrics) — last 5 cycles data
- [Split-Role Critique](#split-role-critique) — Efficiency, Correctness, Novelty critics
- [Agent Effectiveness Review](#evaluating-agent-effectiveness) — per-agent assessment
- [Proposing Improvements](#proposing-improvements) — meta-review.md format
- [Automated Prompt Evolution](#automated-prompt-evolution) — TextGrad-style optimization
- [Skill Synthesis Check](#skill-synthesis-check) — instinct-to-gene/skill graduation
- [Mutation Testing](#self-generated-evaluation-mutation-testing) — eval quality validation
- [Workflow Topology Review](#workflow-topology-review) — phase ordering optimization
- [Applying Changes](#applying-remaining-changes) — config updates, digest regeneration

# Evolve Loop — Phase 6: META-CYCLE

Runs ONLY when `cycle % 5 === 0`. Evaluates the evolve-loop's own effectiveness and proposes improvements to agent prompts, strategies, and pipeline topology.

---

## Collecting Metrics

Gather from the last 5 cycles in `evalHistory` and `ledger.jsonl`:
- Tasks shipped vs attempted (success rate)
- Average audit iterations per task (Builder efficiency)
- Stagnation pattern count
- Instinct confidence trend

## Split-Role Critique

Use three specialized perspectives to avoid blind spots:

| Critic | Focus | Key Question |
|--------|-------|-------------|
| **Efficiency** | Cost, token usage, task sizing, model routing | "Are we spending tokens wisely? Could tasks be smaller?" |
| **Correctness** | Eval pass rates, audit verdicts, regression trends | "Are we shipping quality code? Are evals catching issues?" |
| **Novelty** | Instinct diversity, task variety, stagnation patterns | "Are we learning new things? Or repeating the same work?" |

Each critic reviews independently and produces 1-3 findings. Synthesize into meta-review, resolving conflicts: correctness > efficiency > novelty.

## Evaluating Agent Effectiveness

| Agent | Assessment Questions |
|-------|---------------------|
| Scout | Right-sized tasks? Are they shipping? |
| Builder | How many attempts per task? Self-verify pass rate? |
| Auditor | WARN/FAIL verdicts resolved or accumulating? |
| Operator | Recommendations being followed? |

## Proposing Improvements

Write `meta-review.md` to workspace:

```markdown
# Meta-Cycle Review — Cycles {N-4} to {N}

## Pipeline Metrics
- Success rate: X/Y tasks (Z%)
- Avg audit iterations: N
- Stagnation patterns: N active
- Instinct trend: growing/stable/stale

## Agent Effectiveness
| Agent | Assessment | Suggested Change |
|-------|-----------|-----------------|
| Scout | ... | ... |
| Builder | ... | ... |
| Auditor | ... | ... |
| Operator | ... | ... |

## Recommended Changes
1. <specific change to agent prompt, strategy, or process>
```

## Automated Prompt Evolution

Based on meta-review findings, refine agent prompts via critique-synthesize loop:

1. **Critique:** Identify specific prompt weaknesses from cycle outcomes
2. **Synthesize:** Propose targeted edits (additions, rewording, examples). Small and targeted — never rewrite entire definitions.
3. **Validate:** Check proposed edit doesn't contradict instincts or policies
4. **Apply:** Edit agent file. Log before/after and rationale in meta-review.
5. **Track:** Append `prompt-evolution` ledger entry

**TextGrad-style optimization:** For each edit, generate a "textual gradient":
- What the current prompt produces (observed)
- What it should produce (desired)
- The specific text change (the "gradient")
- Expected impact on process rewards

| Safety Constraint | Detail |
|-------------------|--------|
| Scope | Only non-structural sections (guidance, examples, strategy) |
| Limit | Max 2 prompt edits per meta-cycle |
| Reversibility | All committed; revert with `git revert` |
| Auto-revert | If worse performance in next meta-cycle |

## Skill Synthesis Check

After prompt evolution, before mutation testing. Closes the loop between learning and capability expansion.

**Step 1: Identify candidates** — query `instinctSummary` for clusters of 3+ instincts meeting ALL:
- Same `category` (all procedural, semantic, or episodic)
- Related patterns (overlapping keywords, shared target file types)
- All confidence >= 0.8
- None already `graduated: true`

**Step 2: Determine synthesis target:**

| Option | When | Output |
|--------|------|--------|
| **Gene** | Instincts describe repeatable fix/implementation pattern | `.evolve/genes/<pattern-name>.md` with Selector, Steps, Validation, Source Instincts |
| **Skill Fragment** | Instincts describe agent behavior improvements | New agent prompt section (max 20 lines), gated through TextGrad validation |

**Step 3: Record** in `state.json.synthesizedTools`:
```json
{"name": "<pattern-name>", "path": ".evolve/genes/<pattern-name>.md", "purpose": "<description>",
 "sourceInstincts": ["inst-012", "inst-017"], "cycle": "<N>", "useCount": 0, "type": "gene|skill-fragment"}
```

**Step 4: Update source instincts** — mark `graduated: true`. Stop decaying but no longer individually injected.

| Safety Constraint | Detail |
|-------------------|--------|
| Limit | Max 1 synthesis per meta-cycle |
| Validation | Must pass eval-quality-check rigor filter |
| Reversibility | Subject to same auto-revert as prompt evolution |

## Self-Generated Evaluation (mutation testing)

Test eval quality by generating mutations:
1. For each task in the last 5 cycles, generate 2-3 small mutations
2. Run existing eval graders against mutated code
3. Track **mutation kill rate** (caught / generated)

```markdown
## Mutation Testing Results
- Mutations generated: <N>
- Mutations killed: <N>
- Kill rate: <percentage>
- Weak evals identified: <list>
- Proposed improvements: <list>
```

Target: >80%. Below 60% triggers eval improvement as priority task.

## Workflow Topology Review

Evaluate current phase ordering and agent configuration:

| Analysis | Question |
|----------|----------|
| Phase skip | Were any phases redundant this meta-cycle? |
| Phase merge | Could two phases combine? |
| Phase addition | Is there a gap? |
| Parallel opportunities | Could independent tasks build in parallel? |

```markdown
## Topology Recommendations
- **Current:** DISCOVER -> BUILD -> AUDIT -> SHIP -> LEARN
- **Proposed:** DISCOVER -> BUILD(parallel) -> AUDIT(light) -> SHIP -> LEARN
- **Rationale:** <why>
```

Topology changes are proposals only — require human approval. Never auto-apply.

## Applying Remaining Changes

- Update default strategy, token budgets, or configuration based on meta-review
- Archive `meta-review.md` to history
- Regenerate `.evolve/project-digest.md` (shared location) to capture structural changes
