# Persistent Memory Architecture

<!-- challenge: f1b06434ebbac97c -->

The evolve-loop is a multi-cycle system. Without persistent memory, each cycle re-discovers the same patterns, re-makes the same mistakes, and pays full token cost for knowledge that was learned (and forgotten) the cycle before. This document describes how the loop retains and applies knowledge across sessions.

Research basis: Mem0 (arXiv:2504.19413) reports 26% quality improvement, 91% reduced latency, and 90%+ token savings by maintaining structured persistent memory rather than re-loading full conversation history. Meta-Policy Reflexion (arXiv:2509.03990) demonstrates that predicate-form rules outperform prose summaries for cross-cycle retrieval in agentic systems.

---

## 1. Why Persistent Memory

Every evolve-loop cycle generates hard-won knowledge: which file patterns are brittle, which eval-grader forms are reliable, which task sizes are appropriate for the codebase. Without a memory system, that knowledge evaporates at cycle end and must be re-earned through trial and error.

Persistent memory serves three concrete purposes:

- **Error prevention** — anti-patterns discovered in cycle N are checked before implementation in cycle N+1
- **Efficiency** — reusable plan templates eliminate redundant reasoning on recurring task types
- **Quality compounding** — each cycle's learning raises the floor for the next, producing measurable quality improvement over time (see Mem0 arXiv:2504.19413 benchmark results)

The alternative — loading full cycle history as raw context — is both expensive and noisy. Structured memory selects the relevant signal and discards the rest.

---

## 2. Memory Types

The evolve-loop maintains four categories of persistent memory, each serving a distinct retrieval need:

### Instincts (Patterns)

**Path:** `.evolve/instincts/personal/`

YAML-encoded patterns extracted during Phase 5 (LEARN). Each instinct captures a single observation with a confidence score (0.5 for new, up to 1.0 for proven). Instincts are typed as anti-pattern, successful-pattern, convention, architecture, or process.

Agents read the compact `instinctSummary` from `state.json` rather than parsing individual YAML files, keeping token cost proportional to the number of active instincts, not the total historical count.

### Genes (Fix Templates)

**Path:** `.evolve/genes/`

Pre-synthesized fix templates for recurring error classes. A gene capsule pairs a selector (the error condition) with a verified fix sequence. When the Builder encounters a known error class, it checks gene selectors before designing a solution from scratch — eliminating both reasoning cost and the risk of re-inventing a flawed fix.

### Plan Cache (Reusable Plans)

**Path:** `state.json → planCache`

JSON-encoded plan templates for task types that recur across cycles (e.g., "add a new doc", "fix a relative path reference", "extend a YAML schema"). When the Scout selects a task matching a cached plan type, the Builder loads the cached plan and adapts it rather than reasoning from first principles.

### Project Context (Architecture Knowledge)

**Path:** `state.json → projectContext`, `docs/architecture.md`

Stable facts about the repository: directory layout, file ownership, test commands, domain type, blast-radius hotspots, and integration points. This layer answers "what is the shape of this system" so agents don't need to re-scan the codebase each cycle.

---

## 3. Memory Lifecycle

Memory in the evolve-loop follows a four-phase lifecycle: creation, retrieval, decay, and consolidation.

### Creation (Learn Phase)

At the end of each cycle (Phase 5 — LEARN), the orchestrator analyzes cycle artifacts and extracts new instincts. A new instinct starts with confidence 0.5. Gene capsules are synthesized when the same error class recurs across two or more cycles. Plan cache entries are written after a successful build that used a clearly generalizable approach.

### Retrieval (Discover and Build Phases)

The Scout reads `instinctSummary` before scanning to avoid re-discovering known issues. The Builder reads instincts and gene selectors before implementation to apply proven fixes and avoid documented anti-patterns. Plan cache is consulted at task-start before any design reasoning begins.

Retrieval is always filtered: agents load the compact summary form, not raw history. This is the mechanism behind the 91% latency reduction in the Mem0 (arXiv:2504.19413) benchmark — structured retrieval versus full-context replay.

### Decay (Unused Memories Expire)

Instincts that are not cited in `instinctsApplied` across three consecutive cycles lose 0.1 confidence per pass. Instincts below confidence 0.3 are archived (never deleted — provenance is preserved). This prevents stale knowledge from polluting retrieval and keeps the active set focused on patterns that remain relevant to the current codebase state.

Gene capsules that have not been triggered in five cycles are marked `stale: true` and excluded from the active selector index.

### Consolidation (Related Memories Merge)

Every three cycles, the orchestrator runs a consolidation pass:

1. Cluster instincts with >85% semantic similarity
2. Merge clustered instincts into a single higher-confidence entry
3. Archive superseded entries with a `mergedInto` reference
4. Compress the `instinctSummary` to a fixed-size representation

Consolidation bounds memory growth and raises signal quality — two instincts saying the same thing in different words become one instinct with higher confidence.

---

## 4. Structured vs Unstructured Memory

Early evolve-loop cycles used prose notes (builder-notes.md, cycle summaries) as the primary cross-cycle memory surface. Prose is human-readable but degrades retrieval quality: LLMs reading prose summaries extract different details on each pass, and relevant patterns are frequently missed when buried in narrative.

Meta-Policy Reflexion (arXiv:2509.03990) formalizes this: predicate-form rules ("IF file is in docs/ AND task type is add-doc THEN use append-only pattern") outperform prose summaries for structured knowledge retrieval because they are unambiguous, scannable, and directly actionable.

The evolve-loop's instinct system applies this principle. Each instinct is a predicate, not a paragraph:
- `pattern`: a short kebab-case identifier (the predicate name)
- `description`: a single, specific, actionable statement
- `type`: the knowledge category (enables targeted query by phase)
- `confidence`: numeric, enabling threshold-based filtering

Agents apply instincts as rules, not as inspiration. A Builder reading `anti-pattern: absolute-path-in-eval-grader` applies the fix template immediately — no inference needed.

---

## 5. Integration Points

Each evolve-loop phase reads or writes memory at defined points:

| Phase | Role | Memory Operation |
|-------|------|-----------------|
| Phase 1 — SCOUT | Scout | Read: instinctSummary, projectContext, fileExplorationMap. Write: scout-report.md |
| Phase 2 — BUILD | Builder | Read: instinctSummary, genes (selectors), planCache. Write: build-report.md, builder-notes.md |
| Phase 3 — AUDIT | Auditor | Read: instinctSummary, build-report.md. Write: audit-report.md |
| Phase 4 — OPERATE | Operator | Read: all workspace files, state.json. Write: state.json (metrics, taskArms), operator-log.md |
| Phase 5 — LEARN | Orchestrator | Read: all cycle artifacts. Write: instincts (YAML), planCache, state.json (instinctSummary, confidence updates) |

Memory writes are immutable: new data is appended or written to new files; existing records are not overwritten. This ensures provenance is always traceable.

---

## 6. Quality Guardrails

Persistent memory introduces a failure mode: incorrect knowledge, once written, persists and misleads future cycles. Three guardrails prevent this:

### Provenance Tracking

Every instinct records its source cycle and task slug (`source: "cycle-N/task-slug"`). Every gene capsule records its originating error, the fix applied, and the cycle in which the fix was verified. Provenance enables root-cause analysis when a memory-guided decision leads to a failure.

### Confidence Thresholds

Memory is tiered by confidence. A new observation (0.5) is advisory — the Builder considers it but does not treat it as mandatory. A confirmed pattern (0.8+) is applied automatically. A graduated instinct (confidence >= 0.75, cited across 3+ cycles) becomes mandatory guidance. Thresholds prevent premature institutionalization of uncertain observations.

### Decay Policies

Memory that is not confirmed decays. Instinct confidence decreases 0.1 per unconsolidated pass without citation. Gene capsules are marked stale after five uncited cycles. This ensures the active memory set reflects current codebase reality, not historical accidents. Stale or archived memories are retained in `.evolve/instincts/archived/` for audit purposes but excluded from active retrieval.

---

## See Also

- [memory-hierarchy.md](memory-hierarchy.md) — six-layer storage architecture underlying persistent memory
- [instincts.md](instincts.md) — instinct schema, confidence scoring, and graduation thresholds
- [genes.md](genes.md) — gene capsule format and selector matching protocol
- [self-learning.md](self-learning.md) — feedback loops that populate memory across cycles
