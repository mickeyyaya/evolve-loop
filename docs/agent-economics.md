> **Agent Economics** — Cost modeling and ROI measurement for agent systems.
> Scope: unit economics, cost amplification, ROI frameworks, budget allocation.
> Audience: engineers building and operating agentic loops.

## Table of Contents

| # | Section | Line |
|---|---------|------|
| 1 | [Unit Economics](#unit-economics) | 14 |
| 2 | [Cost Amplification in Loops](#cost-amplification-in-loops) | 52 |
| 3 | [ROI Framework](#roi-framework) | 88 |
| 4 | [Mapping to Evolve-Loop](#mapping-to-evolve-loop) | 122 |
| 5 | [Budget Allocation Strategies](#budget-allocation-strategies) | 157 |
| 6 | [Prior Art](#prior-art) | 199 |
| 7 | [Anti-Patterns](#anti-patterns) | 230 |

## Unit Economics

### Cost Components

| Component | Unit | Typical Range | Notes |
|-----------|------|---------------|-------|
| Input tokens | per 1M tokens | $0.25 – $15.00 | Varies by model tier |
| Output tokens | per 1M tokens | $1.25 – $75.00 | 3–5x input cost |
| Compute time | per minute | $0.01 – $0.10 | Wall-clock agent runtime |
| Human review | per hour | $50 – $200 | Engineer time reviewing agent output |
| Tool execution | per invocation | $0.001 – $0.05 | File I/O, search, API calls |
| Eval execution | per run | $0.10 – $2.00 | Automated quality checks |

### Cost per Task

| Task Type | Avg Tokens (in/out) | Avg Cost | Avg Duration |
|-----------|---------------------|----------|--------------|
| Single-file edit | 10K / 2K | $0.05 – $0.20 | 30s – 2min |
| Multi-file feature | 50K / 15K | $0.50 – $3.00 | 5min – 15min |
| Full evolve-loop cycle | 200K / 60K | $2.00 – $12.00 | 15min – 45min |
| Shipped feature (multi-cycle) | 500K / 150K | $5.00 – $30.00 | 1hr – 4hr |

### Cost per Cycle Breakdown

| Phase | Token Share | Cost Share | Justification |
|-------|-----------|------------|---------------|
| Scout | 8–12% | 10% | Research and task selection |
| Builder | 45–55% | 50% | Implementation and iteration |
| Auditor | 15–25% | 20% | Review, testing, eval |
| Ship | 3–7% | 5% | Commit, format, push |
| Learn | 10–18% | 15% | Memory consolidation, metrics |

## Cost Amplification in Loops

### Amplification Factors

| Factor | Multiplier | Description |
|--------|-----------|-------------|
| Retry amplification | 1.5x – 4x | Failed attempts consume tokens before succeeding |
| Multi-agent overhead | 1.3x – 2x | Coordination, context passing between agents |
| Eval execution | 1.2x – 1.5x | Running automated checks after each phase |
| Context window reload | 1.1x – 1.8x | Re-reading files across agent boundaries |
| Exploration waste | 1.2x – 3x | Dead-end paths explored before correct solution |

### Loop Cost Formula

```
total_cost = base_cost * retry_factor * agent_overhead * eval_factor

Where:
  base_cost     = tokens_in * price_in + tokens_out * price_out
  retry_factor  = 1 + (retry_rate * avg_retries)
  agent_overhead = 1 + (num_agents - 1) * coordination_cost
  eval_factor   = 1 + (num_evals * eval_cost_ratio)
```

### Cost Escalation by Loop Depth

| Loop Depth | Typical Multiplier | Example |
|------------|-------------------|---------|
| Single pass | 1.0x | One-shot code generation |
| 1 retry loop | 1.5x – 2.0x | Generate → test → fix |
| 2 nested loops | 2.0x – 4.0x | Scout → Build → Audit with retries |
| 3 nested loops | 3.0x – 8.0x | Full evolve-loop with inner retry loops |

### Mitigation Strategies

| Strategy | Cost Reduction | Trade-off |
|----------|---------------|-----------|
| Early exit on failure | 20–40% | May miss fixable issues |
| Model routing (cheap → expensive) | 30–60% | Slight quality risk on routing errors |
| Context pruning between agents | 10–25% | Risk of lost information |
| Caching repeated prompts | 15–30% | Cache invalidation complexity |
| Budget caps per phase | 10–20% | Hard stops may leave work incomplete |

## ROI Framework

### Value Metrics

| Metric | Formula | Target |
|--------|---------|--------|
| Cost per shipped feature | total_spend / features_shipped | < $20 |
| Tasks shipped per dollar | tasks_completed / total_spend | > 0.5 |
| Quality improvement per dollar | (quality_after - quality_before) / spend | Positive trend |
| Developer time saved | manual_estimate - actual_time | > 50% savings |
| Cost per defect prevented | audit_spend / defects_caught | < $5 |

### ROI Calculation

```
ROI = (value_delivered - total_cost) / total_cost * 100%

Where:
  value_delivered = (dev_hours_saved * hourly_rate) + (defects_prevented * defect_cost)
  total_cost      = inference_cost + compute_cost + human_review_cost
```

### ROI Benchmarks

| Scenario | Typical ROI | Break-even Point |
|----------|-------------|------------------|
| Routine code edits | 200–500% | Immediate |
| Multi-file features | 100–300% | 2–3 cycles |
| Complex refactoring | 50–150% | 5–10 cycles |
| Exploratory research | -20% – 80% | Varies widely |

### Tracking Requirements

| Data Point | Collection Method | Frequency |
|------------|-------------------|-----------|
| Token usage per cycle | API response metadata | Every cycle |
| Wall-clock time per phase | Timestamp diff in logs | Every phase |
| Features shipped | Git tag / changelog count | Weekly |
| Defects caught by auditor | Audit report parsing | Every cycle |
| Human intervention rate | Manual log entries | Every cycle |

## Mapping to Evolve-Loop

### Gene-to-Economics Mapping

| Gene | Economic Function | Cost Impact |
|------|-------------------|-------------|
| `tokenBudget` | Hard cost constraint per cycle | Prevent runaway spend |
| `modelTier` | Model routing for cost optimization | 30–60% savings with tiered routing |
| `leanMode` | Reduce token usage in low-value phases | 15–25% cost reduction |
| `processRewards` | Value measurement signal | Quantify ROI per cycle |
| `maxRetries` | Retry amplification cap | Bound worst-case cost |

### tokenBudget as Cost Constraint

| Budget Level | Token Limit | Approx Cost | Use Case |
|--------------|-------------|-------------|----------|
| Minimal | 50K | $0.50 | Simple edits, docs |
| Standard | 200K | $2.00 | Single feature cycle |
| Extended | 500K | $5.00 | Complex multi-file work |
| Maximum | 1M | $10.00 | Architecture-level changes |

### Model Routing as Cost Optimization

| Task Complexity | Recommended Model | Cost per 1M Tokens | Quality Trade-off |
|----------------|-------------------|--------------------|--------------------|
| Trivial (formatting, docs) | Haiku 4.5 | $0.25 / $1.25 | Negligible |
| Standard (single-file edit) | Sonnet 4.6 | $3.00 / $15.00 | None |
| Complex (architecture) | Opus 4.5 | $15.00 / $75.00 | Maximum quality |

### Lean Mode Economics

| Mode | Token Usage | Cost | Quality Impact |
|------|-------------|------|----------------|
| Normal | 100% baseline | 100% | Full quality |
| Lean | 60–70% baseline | 60–70% | Minor reduction in exploration |
| Ultra-lean | 40–50% baseline | 40–50% | Skip optional phases |

### processRewards as Value Signal

| Reward Signal | Economic Interpretation | Action |
|---------------|------------------------|--------|
| High reward, low cost | Excellent ROI — scale up | Increase budget allocation |
| High reward, high cost | Good ROI — optimize | Apply lean mode, route models |
| Low reward, low cost | Acceptable — monitor | No change needed |
| Low reward, high cost | Negative ROI — intervene | Reduce scope, change approach |

## Budget Allocation Strategies

### Default Allocation

| Phase | Budget Share | Rationale |
|-------|-------------|-----------|
| Scout | 10% | Research is cheap; use smaller models |
| Builder | 50% | Core value creation; largest token consumer |
| Auditor | 20% | Quality assurance; catches costly defects |
| Ship | 5% | Mechanical; minimal token usage |
| Learn | 15% | Memory consolidation; prevents re-research |

### Allocation by Task Type

| Task Type | Scout | Builder | Auditor | Ship | Learn |
|-----------|-------|---------|---------|------|-------|
| New feature | 15% | 45% | 20% | 5% | 15% |
| Bug fix | 20% | 40% | 25% | 5% | 10% |
| Refactoring | 10% | 50% | 25% | 5% | 10% |
| Documentation | 5% | 60% | 10% | 5% | 20% |
| Security fix | 15% | 35% | 30% | 5% | 15% |

### Dynamic Reallocation Rules

| Condition | Action | Rationale |
|-----------|--------|-----------|
| Scout exceeds 15% | Cap and transfer to Builder | Prevent analysis paralysis |
| Builder fails twice | Allocate 10% from Learn to Builder | Prioritize completion |
| Auditor finds critical issues | Allocate 10% from Learn to Auditor | Ensure quality |
| Ship phase blocked | Allocate from Builder surplus | Unblock delivery |
| Remaining budget < 20% after Builder | Enter lean mode for remaining phases | Preserve budget for essentials |

### Budget Monitoring Checkpoints

| Checkpoint | Trigger | Action if Over Budget |
|------------|---------|----------------------|
| Post-Scout | > 12% consumed | Switch Scout to Haiku |
| Mid-Builder | > 60% consumed | Enable lean mode |
| Post-Builder | > 75% consumed | Skip optional Auditor checks |
| Post-Auditor | > 90% consumed | Minimal Ship and Learn |
| Post-Ship | > 95% consumed | Write only critical learnings |

## Prior Art

### Anthropic Pricing Models

| Model | Input (per 1M) | Output (per 1M) | Prompt Caching Discount |
|-------|----------------|------------------|------------------------|
| Claude Haiku 4.5 | $0.80 | $4.00 | 90% on cached reads |
| Claude Sonnet 4.6 | $3.00 | $15.00 | 90% on cached reads |
| Claude Opus 4.5 | $15.00 | $75.00 | 90% on cached reads |

### OpenAI Token Economics

| Model | Input (per 1M) | Output (per 1M) | Batch Discount |
|-------|----------------|------------------|----------------|
| GPT-4.1 | $2.00 | $8.00 | 50% |
| GPT-4.1 mini | $0.40 | $1.60 | 50% |
| o3 | $10.00 | $40.00 | 50% |

### Key Research

| Source | Finding | Relevance |
|--------|---------|-----------|
| MIT 2025 GenAI ROI Study | 60% of enterprise GenAI projects fail ROI targets | Measure before scaling |
| Anthropic prompt caching (2024) | 90% cost reduction on repeated context | Cache system prompts aggressively |
| Karpathy autoresearch (2025) | Autonomous loops viable at $0.50–$2.00 per research task | Validates evolve-loop cost model |
| OpenAI batch API (2024) | 50% discount for async workloads | Batch non-urgent evals |

### Cost Trend Observations

| Trend | Direction | Impact on Agent Economics |
|-------|-----------|--------------------------|
| Token prices | Decreasing 50%+ annually | Expanding viable use cases |
| Context windows | Increasing 2–4x annually | Fewer context reloads needed |
| Model capability | Improving per token | Higher ROI at same cost |
| Prompt caching | Expanding availability | Major cost reduction for loops |

## Anti-Patterns

### Cost Anti-Patterns

| Anti-Pattern | Symptom | Fix |
|-------------|---------|-----|
| Ignoring token costs | No cost tracking per cycle | Add token counters to every phase |
| Optimizing only for throughput | High spend, low quality | Track cost-per-quality-unit, not just speed |
| No ROI measurement | Cannot justify agent spend | Implement value metrics from ROI Framework |
| Cost-blind scaling | Linear cost growth, sublinear value | Set budget caps, measure marginal ROI |
| Retry without backoff | Repeating same failed approach | Implement exponential backoff with model escalation |
| Over-provisioning models | Using Opus for trivial tasks | Route by complexity; default to cheapest viable model |
| Ignoring prompt caching | Paying full price for repeated context | Cache system prompts, tool definitions, file contents |
| No phase budgets | One phase consumes entire budget | Enforce per-phase allocation with hard caps |

### Warning Signs

| Signal | Threshold | Response |
|--------|-----------|----------|
| Cost per cycle trending up | > 2x baseline for 3+ cycles | Audit token usage per phase |
| Retry rate above normal | > 30% of cycles need retries | Investigate root cause of failures |
| Human intervention rate rising | > 20% of cycles need human help | Improve agent instructions or tool quality |
| ROI declining | < 100% for 5+ cycles | Pause and reassess task selection |
| Budget exhaustion before Ship | > 50% of cycles | Reduce Builder allocation, enable lean mode |
