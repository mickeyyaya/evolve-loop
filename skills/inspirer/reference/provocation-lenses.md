# Provocation Lenses — Full Reference

> Detailed descriptions, usage guidance, and domain affinity matrix for all 12 provocation lenses used by the `/inspirer` skill.

## Contents
- [Lens Catalog](#lens-catalog)
- [Domain Affinity Matrix](#domain-affinity-matrix)
- [Lens Selection Algorithm](#lens-selection-algorithm)

---

## Lens Catalog

### 1. Inversion

| Attribute | Detail |
|-----------|--------|
| **Question** | "What if we did the exact opposite of the current approach?" |
| **When to use** | Assumptions feel entrenched; "we've always done it this way" |
| **Example** | Topic: "How to improve API response time?" → Inversion: "What if we made the API intentionally slower but more predictable?" → Research reveals: rate-limited APIs with guaranteed SLAs outperform bursty fast APIs for user satisfaction |
| **Domain affinity** | code-architecture (HIGH), process-improvement (HIGH), product-strategy (MEDIUM) |

### 2. Analogy

| Attribute | Detail |
|-----------|--------|
| **Question** | "What would this look like borrowed from {adjacent domain}?" |
| **When to use** | Problem feels domain-specific; cross-pollination likely |
| **Example** | Topic: "How to prioritize tech debt?" → Analogy from medicine: "Triage model — critical (fix now), urgent (this sprint), elective (backlog)" → Research reveals: CodeScene's code health triage reduces unplanned work by 30% |
| **Domain affinity** | technical-research (HIGH), product-strategy (HIGH), general (HIGH) |

### 3. 10x Scale

| Attribute | Detail |
|-----------|--------|
| **Question** | "What breaks at 10x the current load/complexity?" |
| **When to use** | Architecture decisions, scaling planning, bottleneck hunting |
| **Example** | Topic: "Database design for user events" → 10x: "What if we had 10M events/day instead of 1M?" → Research reveals: event sourcing with CQRS handles 10x better than traditional CRUD |
| **Domain affinity** | code-architecture (HIGH), technical-research (HIGH), process-improvement (MEDIUM) |

### 4. Removal

| Attribute | Detail |
|-----------|--------|
| **Question** | "What if we deleted this entirely — what would we lose and gain?" |
| **When to use** | Feature bloat, complexity creep, simplification needed |
| **Example** | Topic: "Our CI pipeline takes 45 minutes" → Removal: "What if we deleted the E2E tests entirely?" → Research reveals: trunk-based development with feature flags + canary deploys replaces 80% of E2E test value |
| **Domain affinity** | code-architecture (HIGH), process-improvement (HIGH), product-strategy (MEDIUM) |

### 5. User-Adjacent

| Attribute | Detail |
|-----------|--------|
| **Question** | "What problem will the user hit NEXT after this is solved?" |
| **When to use** | Feature planning, UX design, retention strategy |
| **Example** | Topic: "Add search to our app" → User-Adjacent: "After finding results, what will users need next?" → Research reveals: 67% of search users immediately want to filter, sort, and compare results |
| **Domain affinity** | product-strategy (HIGH), general (HIGH), process-improvement (MEDIUM) |

### 6. First Principles

| Attribute | Detail |
|-----------|--------|
| **Question** | "Why does this exist? What fundamental constraint requires it?" |
| **When to use** | Inherited complexity, legacy systems, "nobody knows why we do this" |
| **Example** | Topic: "Our deploy process has 12 manual steps" → First Principles: "What constraint originally required each step?" → Research reveals: 8 of 12 steps compensate for a single missing integration test |
| **Domain affinity** | code-architecture (HIGH), process-improvement (HIGH), technical-research (MEDIUM) |

### 7. Composition

| Attribute | Detail |
|-----------|--------|
| **Question** | "What if we combined two unrelated things into one?" |
| **When to use** | DRY opportunities, emergent capabilities, feature integration |
| **Example** | Topic: "We have separate logging and metrics systems" → Composition: "What if logs and metrics were the same thing?" → Research reveals: structured logging with metric extraction (OpenTelemetry) unifies observability |
| **Domain affinity** | code-architecture (HIGH), product-strategy (MEDIUM), technical-research (MEDIUM) |

### 8. Failure Mode

| Attribute | Detail |
|-----------|--------|
| **Question** | "How would this fail silently? What's the worst undetected failure?" |
| **When to use** | Reliability planning, security review, observability gaps |
| **Example** | Topic: "Our caching layer" → Failure Mode: "How would stale cache poison production silently?" → Research reveals: cache stampede + stale reads cause 23% of outages in distributed systems (Uber 2024) |
| **Domain affinity** | code-architecture (HIGH), technical-research (HIGH), process-improvement (MEDIUM) |

### 9. Ecosystem

| Attribute | Detail |
|-----------|--------|
| **Question** | "What tool/library/pattern exists externally that makes this obsolete?" |
| **When to use** | Build-vs-buy decisions, reinvention risk, tool evaluation |
| **Example** | Topic: "Build a custom job scheduler" → Ecosystem: "What existing schedulers would eliminate this work?" → Research reveals: Temporal, Inngest, and Trigger.dev handle 95% of job scheduling needs |
| **Domain affinity** | technical-research (HIGH), code-architecture (HIGH), general (MEDIUM) |

### 10. Time Travel

| Attribute | Detail |
|-----------|--------|
| **Question** | "What will we wish we had built 3 months from now?" |
| **When to use** | Architecture decisions, roadmap planning, forward-looking design |
| **Example** | Topic: "Starting a new microservice" → Time Travel: "In 3 months, what will we regret not adding?" → Research reveals: teams consistently regret skipping structured logging, health checks, and circuit breakers |
| **Domain affinity** | code-architecture (HIGH), product-strategy (HIGH), process-improvement (MEDIUM) |

### 11. Constraint Flip

| Attribute | Detail |
|-----------|--------|
| **Question** | "What if the biggest constraint were removed entirely?" |
| **When to use** | Business constraints assumed fixed, budget/timeline/technology limits |
| **Example** | Topic: "We can't afford a dedicated ML team" → Constraint Flip: "What if ML expertise were free?" → Research reveals: AutoML platforms (Vertex AI, SageMaker) reduce ML team need by 70% for standard use cases |
| **Domain affinity** | product-strategy (HIGH), general (HIGH), process-improvement (HIGH) |

### 12. Audience Shift

| Attribute | Detail |
|-----------|--------|
| **Question** | "What if the primary user were someone completely different?" |
| **When to use** | UX rethinking, market expansion, empathy gap discovery |
| **Example** | Topic: "Our developer tool is hard to learn" → Audience Shift: "What if the user were a designer, not a developer?" → Research reveals: no-code interfaces capture 40% of internal tool builders (Retool 2025 survey) |
| **Domain affinity** | product-strategy (HIGH), general (HIGH), technical-research (MEDIUM) |

---

## Domain Affinity Matrix

Lenses ranked by effectiveness per domain. **Bold** = top picks for that domain.

| Domain | Top Lenses (ranked) |
|--------|-------------------|
| `code-architecture` | **Inversion**, **First Principles**, **10x Scale**, **Failure Mode**, Composition, Removal |
| `product-strategy` | **Audience Shift**, **User-Adjacent**, **Constraint Flip**, **Time Travel**, Analogy |
| `technical-research` | **Analogy**, **Ecosystem**, **10x Scale**, Failure Mode, First Principles |
| `process-improvement` | **First Principles**, **Removal**, **Constraint Flip**, Inversion, Time Travel |
| `general` | **Analogy**, **Constraint Flip**, **Audience Shift**, User-Adjacent, Composition |

---

## Lens Selection Algorithm

```
function selectLenses(domain, count, previousLenses):
  pool = ALL_12_LENSES
  
  # 1. Pick 1 random lens (ensures diversity)
  randomLens = pool[RANDOM % len(pool)]
  selected = [randomLens]
  pool.remove(randomLens)
  
  # 2. Pick remaining from domain affinity ranking
  affinityRanked = DOMAIN_AFFINITY[domain]  # sorted by relevance
  for lens in affinityRanked:
    if lens not in selected and lens not in previousLenses[-3:]:
      selected.append(lens)
    if len(selected) >= count:
      break
  
  # 3. Fill any remaining slots from unused pool
  while len(selected) < count:
    selected.append(pool.pop(0))
  
  return selected
```

**Diversity rule:** Avoid lenses used in the previous 3 invocations (if invoked from evolve-loop with persistent state). For standalone use, no history constraint applies.
