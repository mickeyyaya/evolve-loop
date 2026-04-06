# Worked Examples — Inspirer Pipeline

> 3 end-to-end examples showing the full FRAME → DIVERGE → RESEARCH → SCORE → CONVERGE → DELIVER pipeline.

---

## Example 1: Code Architecture

**Input:** `/inspirer "How should we handle real-time sync in a serverless app?" --depth STANDARD`

### Stage 1: FRAME
```json
{
  "topic": "Real-time sync in serverless architecture",
  "domain": "code-architecture",
  "constraints": ["serverless (no persistent connections)", "cost-sensitive (pay-per-invocation)"]
}
```

### Stage 2: DIVERGE (4 lenses selected)
| Lens | Divergent Question |
|------|-------------------|
| **Inversion** | "What architectures deliberately avoid real-time sync and still succeed?" |
| **Ecosystem** | "What managed services handle real-time sync without custom serverless code?" |
| **10x Scale** | "What real-time sync approaches handle 10M+ concurrent users on serverless?" |
| **Failure Mode** | "How do serverless real-time systems fail silently under load?" |

### Stage 3: RESEARCH (6 queries executed)
| Query | Source | Finding |
|-------|--------|---------|
| "real-time alternatives serverless 2025" | AWS blog | EventBridge + polling with long-lived connections via API Gateway WebSocket |
| "managed real-time sync services" | Liveblocks docs | Liveblocks, Ably, Pusher handle sync as a service — zero serverless code |
| "serverless WebSocket scaling 10M" | AWS re:Invent 2025 | API Gateway WebSocket + DynamoDB streams scales to 10M connections |
| "serverless real-time failure modes" | Dashbird blog | Cold start latency causes 2-5s sync gaps; stale state on reconnection |

### Stage 4: SCORE (4 Inspiration Cards)

| ID | Title | Lens | Feasibility | Impact | Novelty | Composite | Verdict |
|----|-------|------|------------|--------|---------|-----------|---------|
| insp-001 | Use managed sync service (Liveblocks/Ably) | Ecosystem | 0.9 | 0.7 | 0.4 | 0.67 | KEEP |
| insp-002 | Optimistic UI with eventual sync | Inversion | 0.8 | 0.8 | 0.6 | 0.73 | KEEP |
| insp-003 | API Gateway WebSocket + DynamoDB streams | 10x Scale | 0.7 | 0.8 | 0.5 | 0.67 | KEEP |
| insp-004 | Heartbeat-based stale detection | Failure Mode | 0.9 | 0.6 | 0.3 | 0.60 | KEEP |

### Stage 5: CONVERGE
All 4 cards kept. Ranked: insp-002 (0.73), insp-001 (0.67), insp-003 (0.67), insp-004 (0.60).

### Stage 6: DELIVER (brief format)
| # | Idea | Lens | Score | One-Liner | Next Step |
|---|------|------|-------|-----------|-----------|
| 1 | Optimistic UI + eventual sync | Inversion | 0.73 | Skip real-time entirely — show optimistic state, reconcile async | Prototype with React useOptimistic + SQS |
| 2 | Managed sync service | Ecosystem | 0.67 | Let Liveblocks handle sync; focus on business logic | Evaluate Liveblocks free tier for POC |
| 3 | WebSocket + DynamoDB streams | 10x Scale | 0.67 | AWS-native, scales to 10M, pay-per-message | Spike API Gateway WebSocket with 3 Lambda handlers |
| 4 | Heartbeat stale detection | Failure Mode | 0.60 | Detect and recover from stale state on reconnect | Add last-sync timestamp to client state |

---

## Example 2: Product Strategy

**Input:** `/inspirer "What features should we add to increase user retention?" --depth DEEP`

### Stage 1: FRAME
```json
{
  "topic": "Feature ideas to increase user retention",
  "domain": "product-strategy",
  "constraints": ["SaaS product", "small team (5 engineers)"]
}
```

### Stage 2: DIVERGE (5 lenses)
| Lens | Divergent Question |
|------|-------------------|
| **Audience Shift** | "What would a non-technical user need to stay engaged?" |
| **User-Adjacent** | "After onboarding, what's the #1 reason users churn in the first week?" |
| **Constraint Flip** | "What if we had unlimited engineering — what retention feature would we build first?" |
| **Time Travel** | "In 6 months, what will churned users say they wished the product had?" |
| **Analogy** | "How do gaming platforms retain users — what can SaaS learn from games?" |

### Stage 4: SCORE (top 3 shown)

| ID | Title | Lens | F | I | N | Composite | Verdict |
|----|-------|------|---|---|---|-----------|---------|
| insp-001 | Streak-based engagement rewards | Analogy (gaming) | 0.8 | 0.8 | 0.7 | 0.77 | KEEP |
| insp-002 | Proactive "getting stuck" detection | User-Adjacent | 0.7 | 0.9 | 0.6 | 0.73 | KEEP |
| insp-003 | Weekly value-delivered email digest | Time Travel | 0.9 | 0.7 | 0.4 | 0.67 | KEEP |

---

## Example 3: Technical Research (evolve format)

**Input:** `/inspirer "Multi-agent coordination patterns" --depth QUICK --format evolve`

### Stage 1: FRAME
```json
{
  "topic": "Multi-agent coordination patterns",
  "domain": "technical-research",
  "constraints": []
}
```

### Stage 2: DIVERGE (3 lenses)
| Lens | Divergent Question |
|------|-------------------|
| **Analogy** | "How do biological swarms coordinate without central control?" |
| **Ecosystem** | "What multi-agent frameworks exist that we don't know about?" |
| **First Principles** | "What is the minimum coordination protocol for N agents?" |

### Stage 6: DELIVER (evolve format)
```json
{
  "conceptCandidates": [
    {
      "id": "insp-001",
      "title": "Stigmergic coordination via shared artifacts",
      "targetFiles": ["agents/agent-templates.md", "skills/evolve-loop/memory-protocol.md"],
      "complexity": "M",
      "feasibility": 0.7,
      "impact": 0.7,
      "novelty": 0.8,
      "composite": 0.73,
      "source": "inspirer",
      "lens": "Analogy",
      "researchBacking": ["arXiv:2504.xxxxx — swarm intelligence in LLM agents"]
    },
    {
      "id": "insp-002",
      "title": "Adopt CrewAI hierarchical delegation",
      "targetFiles": ["skills/evolve-loop/phases.md"],
      "complexity": "S",
      "feasibility": 0.8,
      "impact": 0.6,
      "novelty": 0.5,
      "composite": 0.63,
      "source": "inspirer",
      "lens": "Ecosystem",
      "researchBacking": ["crewai.com docs — hierarchical process"]
    }
  ]
}
```
