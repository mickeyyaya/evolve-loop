# Knowledge Distillation for Agents

> Reference document for transferring knowledge between agent generations.
> Apply distillation techniques to compress reasoning traces, build patterns,
> and audit findings into reusable, compact formats that accelerate future cycles.

## Table of Contents

1. [Distillation Techniques](#distillation-techniques)
2. [Knowledge Transfer Pipeline](#knowledge-transfer-pipeline)
3. [Mapping to Evolve-Loop](#mapping-to-evolve-loop)
4. [Implementation Patterns](#implementation-patterns)
5. [Prior Art](#prior-art)
6. [Anti-Patterns](#anti-patterns)

---

## Distillation Techniques

| Technique | Description | Input | Output | Best For |
|-----------|-------------|-------|--------|----------|
| **Reasoning Trace Distillation** | Extract decision rationale from verbose chain-of-thought into compact rules | Full CoT traces from Scout/Builder/Auditor | Concise decision heuristics (instincts) | Converting exploratory reasoning into reusable judgment |
| **Multi-Teacher Distillation** | Merge knowledge from multiple specialist agents into a single generalist representation | Outputs from Scout, Builder, and Auditor on the same task | Unified knowledge entry combining search, build, and review perspectives | Cross-role knowledge that no single agent holds |
| **Self-Distillation** | Agent compresses its own outputs into a more efficient form after task completion | Agent's full session trace | Compressed session summary retaining key decisions and outcomes | Reducing context cost for future retrieval |
| **Trajectory Compression** | Reduce multi-step execution logs to essential state transitions | Full tool-call sequences and intermediate results | Minimal action sequence reproducing the same outcome | Plan cache entries and replayable build recipes |
| **Auto-Distillation from Build Traces** | Automatically extract reusable patterns from successful Builder executions | build-report.md artifacts across multiple cycles | Gene templates, code snippets, and parameterized procedures | Scaling procedural knowledge without manual curation |

---

## Knowledge Transfer Pipeline

Define each stage of the pipeline from raw traces to transferable knowledge.

```
Raw Traces ──> Extract Patterns ──> Compress ──> Encode ──> Transfer
  (Stage 1)       (Stage 2)        (Stage 3)    (Stage 4)   (Stage 5)
```

| Stage | Action | Input | Output | Compression Ratio |
|-------|--------|-------|--------|-------------------|
| 1. Collect | Gather raw artifacts from completed cycles | scout-report, build-report, audit-report, experiments.jsonl | Timestamped artifact corpus | 1:1 (no compression) |
| 2. Extract | Identify recurring patterns, successful strategies, and failure modes | Raw artifact corpus | Pattern candidates with frequency and confidence scores | ~5:1 |
| 3. Compress | Remove redundancy, merge overlapping patterns, drop low-confidence entries | Pattern candidates | Deduplicated pattern set with supporting evidence | ~10:1 |
| 4. Encode | Convert patterns into reusable formats (instincts, genes, cache entries) | Compressed pattern set | Structured knowledge artifacts in standard formats | ~20:1 |
| 5. Transfer | Inject encoded knowledge into next-generation agent context | Encoded knowledge artifacts | Updated instincts.md, genes.md, plan-cache, project-digest | Variable (context-budget-dependent) |

### Stage Ownership

| Stage | Primary Owner | Validation Gate |
|-------|---------------|-----------------|
| Collect | Orchestrator (automatic after each cycle) | Artifacts exist and pass schema validation |
| Extract | Scout (pattern recognition phase) | Minimum 3 supporting examples per pattern |
| Compress | Auditor (deduplication and quality check) | No information loss on critical patterns |
| Encode | Builder (template creation) | Encoded artifact passes lint and format checks |
| Transfer | Orchestrator (context injection) | Target agent confirms knowledge is accessible |

---

## Mapping to Evolve-Loop

Map each evolve-loop primitive to its distillation role.

| Evolve-Loop Primitive | Distillation Role | Knowledge Type | Update Trigger | Example |
|-----------------------|-------------------|----------------|----------------|---------|
| **Instincts** | Distilled judgment heuristics from reasoning traces | Semantic (compressed decision rules) | When a pattern reaches confidence threshold (>0.80) | "Validate imports before running tests" extracted from 12 Builder failures |
| **Genes** | Procedural distillation of successful build strategies | Procedural (parameterized templates) | When a Builder approach succeeds 3+ times with >90% audit pass | `parallel-test-runner` gene distilled from cycles 45-52 |
| **Plan Cache** | Compressed experience from prior task executions | Trajectory (minimal action sequences) | After successful cycle completion | Cached plan for "add new API endpoint" reduces Scout time by 60% |
| **Project Digest** | Context distillation of codebase state | Semantic (compressed codebase summary) | On significant codebase changes (>500 lines delta) | Digest update after major refactor, consumed by all agents |
| **Experiments JSONL** | Raw episodic knowledge awaiting distillation | Episodic (full experiment records) | Every cycle (append-only) | Raw record of cycle 147 task selection and outcomes |

### Knowledge Flow Between Agents

| Source Agent | Distilled Output | Consuming Agent | Transfer Format |
|-------------|------------------|-----------------|-----------------|
| Scout | Task selection heuristics, search strategies | Future Scout instances | Instinct entries in instincts.md |
| Builder | Code generation templates, fix patterns | Future Builder instances | Gene entries in genes.md |
| Auditor | Quality rules, common defect categories | Future Auditor instances | Audit rubric entries |
| Auditor | Actionable fix suggestions | Builder (same cycle) | Structured feedback in audit-report.md |
| Builder | Build context and rationale | Auditor (same cycle) | Structured summary in build-report.md |

---

## Implementation Patterns

### Distill Builder Approaches into Reusable Templates

| Step | Action | Tool/Artifact | Success Criteria |
|------|--------|---------------|------------------|
| 1 | Identify Builder runs with audit score >= 9/10 | experiments.jsonl query | Minimum 3 qualifying runs |
| 2 | Extract common code structure and tool-call sequences | Diff analysis across qualifying build-reports | Shared pattern covers >= 80% of steps |
| 3 | Parameterize variable elements (file paths, names, config) | Template with `{{placeholders}}` | Template validates against all source runs |
| 4 | Encode as gene with metadata (trigger condition, expected outcome) | genes.md entry | Gene passes format lint |
| 5 | Validate by running Builder with gene on a held-out task | Controlled experiment cycle | Audit score >= 8/10 on new task |

### Compress Audit Findings into Actionable Rules

| Step | Action | Tool/Artifact | Success Criteria |
|------|--------|---------------|------------------|
| 1 | Collect audit failures across last N cycles | audit-report.md corpus | Minimum 10 failure instances |
| 2 | Cluster failures by root cause category | Pattern extraction | Each cluster has >= 3 instances |
| 3 | Draft rule: condition, check, remediation | Instinct candidate | Rule is falsifiable and actionable |
| 4 | Validate rule against historical data (would it have caught past failures?) | Backtest against audit corpus | Precision >= 0.85, recall >= 0.70 |
| 5 | Encode as instinct with confidence score | instincts.md entry | Instinct passes schema validation |

### Trajectory Compression for Plan Cache

| Step | Action | Compression Target |
|------|--------|--------------------|
| 1 | Record full tool-call sequence from successful Builder run | Raw trajectory (100%) |
| 2 | Remove no-op steps and redundant reads | ~70% of original |
| 3 | Merge consecutive edits to the same file | ~50% of original |
| 4 | Abstract file-specific paths to parameterized references | ~40% of original |
| 5 | Store as plan-cache entry with trigger conditions | ~30% of original |

---

## Prior Art

| Source | Key Contribution | Relevance to Evolve-Loop |
|--------|-----------------|--------------------------|
| **Hinton, Valla, & Dean (2015)** "Distilling the Knowledge in a Neural Network" | Introduced knowledge distillation via soft targets from teacher to student | Foundation for transferring agent knowledge between generations; instinct confidence scores mirror soft-target distributions |
| **arXiv:2504.14772** "A Survey on Knowledge Distillation" | Comprehensive taxonomy of distillation methods: response-based, feature-based, relation-based | Provides framework for classifying evolve-loop distillation techniques by type |
| **arXiv:2603.13017** "Structured Knowledge Distillation" | Distill structured outputs (sequences, graphs) rather than single predictions | Applicable to distilling multi-step Builder trajectories and structured audit reports |
| **AgentDiet (trajectory compression)** | Compress agent execution traces while preserving task performance | Direct application to plan-cache compression and context budget management |
| **MemGPT / Letta** | Tiered memory with automatic consolidation between episodic and semantic stores | Analogous to evolve-loop's three-tier memory model (episodic artifacts, semantic instincts, procedural genes) |
| **Karpathy autoresearch pattern** | Self-directed research loops with automatic knowledge accumulation | Inspires the auto-distillation approach where agents extract their own reusable knowledge |

---

## Anti-Patterns

| Anti-Pattern | Description | Symptom | Mitigation |
|-------------|-------------|---------|------------|
| **Lossy Distillation** | Compress too aggressively, losing critical details like edge cases or failure conditions | Builder repeats mistakes that were previously caught by Auditor | Set minimum detail thresholds per knowledge type; require edge-case preservation in compression rules |
| **Teacher Bias Amplification** | Distilled knowledge inherits and reinforces biases from source agents | Instincts consistently favor one approach even when alternatives are better | Use multi-teacher distillation; require diversity in source patterns; add contrarian review step |
| **Outdated Knowledge Propagation** | Transfer knowledge from old cycles without validating against current codebase state | Builder applies deprecated patterns; Auditor enforces obsolete rules | Attach expiry metadata to all distilled knowledge; validate against current project-digest before transfer |
| **Distillation Without Validation** | Encode patterns as instincts or genes without backtesting against historical data | Low-quality instincts that fire incorrectly; genes that produce failing code | Require backtest pass rate >= 0.85 before promoting any pattern to production knowledge |
| **Premature Distillation** | Extract patterns from too few examples, creating brittle or overfit knowledge | Knowledge fails on novel tasks outside narrow training distribution | Enforce minimum example count (3+ for genes, 5+ for instincts) before distillation |
| **Context Pollution** | Transfer too much distilled knowledge, overwhelming agent context windows | Agent performance degrades due to context overload; key instructions buried | Budget distilled knowledge per agent role; apply relevance filtering at transfer stage |
