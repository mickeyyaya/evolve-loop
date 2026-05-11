> Three-tier memory consolidation pipeline: convert raw cycle artifacts into durable, retrievable agent knowledge through episodic, semantic, and procedural stages.

## Table of Contents

- [Three-Tier Memory Model](#three-tier-memory-model)
- [Consolidation Pipeline](#consolidation-pipeline)
- [Consolidation Triggers](#consolidation-triggers)
- [Retention and Forgetting Policies](#retention-and-forgetting-policies)
- [Prior Art](#prior-art)
- [Anti-Patterns](#anti-patterns)

---

## Three-Tier Memory Model

Define each tier by storage type, evolve-loop primitive, and concrete example.

| Tier | Memory Type | Description | Evolve-Loop Primitive | Example |
|------|-------------|-------------|-----------------------|---------|
| 1 | Episodic | Raw, timestamped cycle artifacts capturing what happened | scout-report, build-report, audit-report, experiments.jsonl | `cycle-147/scout-report.md` records discovered tasks and rankings |
| 2 | Semantic | Extracted patterns and generalized knowledge distilled from episodes | instincts, failure categories, domain knowledge entries | Instinct: "validate schema before build" (confidence 0.82) |
| 3 | Procedural | Reusable executable skills encoded for direct application | genes, builder templates, plan cache entries | Gene: `parallel-eval-grader` template for concurrent test execution |

### Tier Characteristics

| Property | Episodic | Semantic | Procedural |
|----------|----------|----------|------------|
| Granularity | Per-cycle | Cross-cycle pattern | Reusable skill |
| Lifespan | Short (5-20 cycles) | Medium (decays without citation) | Long (persists while effective) |
| Storage cost | High (full artifacts) | Medium (compressed patterns) | Low (parameterized templates) |
| Retrieval speed | Slow (search required) | Fast (indexed by category) | Fastest (direct invocation) |
| Mutation | Immutable after write | Updated on re-extraction | Versioned on improvement |

---

## Consolidation Pipeline

```
Raw Artifacts ──> Pattern Extraction ──> Semantic Abstraction ──> Procedural Encoding ──> Retrieval Index
   (Tier 1)          (T1 → T2)              (Tier 2)               (T2 → T3)            (Tier 3)
```

### Stage Details

| Stage | Input | Process | Output | Trigger |
|-------|-------|---------|--------|---------|
| 1. Raw Artifacts | Cycle execution | Scout, Builder, Auditor produce reports | Episodic entries in `workspace/` | Every cycle completion |
| 2. Pattern Extraction | 2+ episodic entries with shared signal | Compare artifacts across cycles, identify recurring patterns | Candidate instinct with initial confidence | Pattern appears in 2+ consecutive or 3+ total cycles |
| 3. Semantic Abstraction | Candidate instinct | Validate pattern, assign confidence score, categorize | Instinct entry in `docs/reference/instincts.md` | Extraction confidence >= 0.5 |
| 4. Procedural Encoding | High-confidence instinct | Convert instinct into executable gene or template | Gene file in `docs/reference/genes.md` or plan cache entry | Instinct confidence >= 0.75, cited in 3+ cycles |
| 5. Retrieval Indexing | Procedural entry | Add to retrieval index with tags and search metadata | Indexed gene/template available for Scout discovery | Procedural entry created or updated |

---

## Consolidation Triggers

| Trigger | From | To | Condition | Action |
|---------|------|----|-----------|--------|
| Pattern detection | Episodic | Semantic | Same pattern observed in 2+ cycles | Extract instinct with confidence 0.5 |
| Confidence boost | Semantic | Semantic | Instinct cited in additional cycle | Increment confidence by 0.1 (cap 1.0) |
| Skill graduation | Semantic | Procedural | Instinct confidence >= 0.75 AND cited 3+ times | Encode as gene or builder template |
| Temporal decay | Semantic | Semantic | Instinct not referenced in 5 cycles | Reduce confidence by 0.1 per idle cycle |
| Archive | Semantic | Archive | Confidence drops to 0.3 or below | Move to archive, remove from active retrieval |
| Expunge | Archive | Deleted | Archived entry not recalled in 10 cycles | Permanently remove |
| Reactivation | Archive | Semantic | Archived instinct re-observed in new cycle | Restore with confidence 0.5 |

---

## Retention and Forgetting Policies

### Policy Summary

| Policy | Mechanism | Parameters | Reference |
|--------|-----------|------------|-----------|
| Temporal decay | Reduce confidence of idle instincts | -0.1 per 5 unreferenced cycles | instinct-forgetting-protocol |
| Entropy gating | Block low-information patterns from promotion | Minimum entropy threshold for extraction | Prevents trivial instincts |
| Spaced repetition | Boost retention of frequently recalled memories | Citation count weighted by recency | Ebbinghaus spacing effect |
| Archive threshold | Move low-confidence entries out of active memory | Archive at confidence <= 0.3 | instinct-forgetting-protocol |
| Capacity budget | Limit active instincts to prevent context bloat | Max 50 active instincts, 20 active genes | Context window management |

### Forgetting Decision Flow

```
For each instinct at cycle boundary:
  IF not cited in last 5 cycles:
    confidence -= 0.1
    IF confidence <= 0.3:
      MOVE to archive
  IF in archive AND not recalled in 10 cycles:
    EXPUNGE permanently
  IF cited this cycle:
    confidence = min(confidence + 0.1, 1.0)
    IF was archived:
      REACTIVATE with confidence 0.5
```

---

## Prior Art

| Source | Key Contribution | Relevance to Evolve-Loop |
|--------|-----------------|--------------------------|
| Hierarchical Procedural Memory (arXiv 2512.18950) | Hierarchical skill storage with abstraction levels | Informs three-tier separation and procedural encoding |
| A-MEM | Agentic memory with structured distillation from raw experience | Model for episodic-to-semantic consolidation pipeline |
| MemRL | Self-evolving agents via episodic memory reinforcement | Validates reinforcement-based confidence scoring |
| ICLR 2026 MemAgents Workshop | Benchmarks for agent memory systems, retrieval-augmented retention | Evaluation framework and decay policy baselines |
| Ebbinghaus forgetting curve | Exponential decay of unrehearsed memory over time | Foundation for temporal decay policy (-0.1 per idle window) |
| Spaced repetition (Leitner system) | Optimal review intervals strengthen long-term retention | Informs citation-based confidence boosting mechanism |
| Cognitive load theory (Sweller) | Working memory has fixed capacity; offload to long-term | Justifies capacity budgets and archive policies |

---

## Anti-Patterns

| Anti-Pattern | Description | Symptom | Mitigation |
|--------------|-------------|---------|------------|
| Infinite context stuffing | Load all memories into every prompt without filtering | Token budget exceeded, degraded reasoning quality | Apply capacity budgets and relevance-based retrieval |
| Stale memory poisoning | Retain outdated instincts that no longer reflect reality | Agent applies obsolete patterns, regression in quality | Enforce temporal decay and archive thresholds |
| Premature generalization | Promote pattern to instinct after single occurrence | False instincts that do not replicate, noise in semantic tier | Require 2+ cycle observations before extraction |
| Memory hoarding | Never forget or archive any memory | Context bloat, retrieval noise, slow agent cycles | Enforce forgetting policies and capacity limits |
| Cargo-cult procedural encoding | Convert instinct to gene without validation of effectiveness | Genes that produce no measurable improvement | Require confidence >= 0.75 and 3+ citations before graduation |
| Retrieval bias | Always retrieve same memories due to index skew | Agent stuck in local optimum, ignores new patterns | Periodically refresh retrieval index, add exploration factor |
