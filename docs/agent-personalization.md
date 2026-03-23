# Agent Personalization

How agents adapt behavior to user preferences and project conventions without retraining. Based on the PPP framework (arXiv:2511.02208) and the Personalized Agents Survey (arXiv:2602.22680).

---

## PPP: Productivity + Proactivity + Personalization

PPP demonstrates that jointly optimizing three objectives via multi-objective RL produces +21.6 average improvement over single-objective baselines. The key insight: accuracy-only training produces agents that complete tasks but misalign with user intent.

### Three Objectives Mapped to Evolve-Loop

| PPP Objective | Description | Evolve-Loop Equivalent |
|--------------|-------------|----------------------|
| **Productivity** | Task completion success | Ship rate, eval pass rate |
| **Proactivity** | Asking clarifying questions at the right moment | Scout deferring ambiguous tasks, Operator HALT on uncertainty |
| **Personalization** | Adapting to diverse user preferences | Instinct system learning project-specific patterns |

### Proactivity in the Evolve-Loop

Proactivity is underexplored in the evolve-loop. PPP shows that strategic clarifying questions improve task success even when they add interaction steps. The equivalent:
- **Scout proactivity:** When a goal is vague, the Scout should propose a narrower interpretation rather than attempting broad coverage
- **Builder proactivity:** When an approach is uncertain (AUQ uncertainty > threshold), the Builder should flag it in the build-report rather than proceeding confidently

---

## Four-Capability Taxonomy (arXiv:2602.22680)

The Personalized Agents Survey identifies 4 capabilities that enable personalization:

| Capability | Description | Evolve-Loop Mechanism |
|-----------|-------------|----------------------|
| **Profile modeling** | Build user/project model from interactions | `project-digest.md`, `state.json` context |
| **Memory** | Remember preferences across sessions | Instinct system (personal + global scopes) |
| **Planning** | Adapt task decomposition to preferences | Scout reading `strategy` and operator briefs |
| **Action execution** | Customize output style and approach | Builder reading instinct patterns |

### Instinct System as Personalization Engine

The instinct system is already the evolve-loop's primary personalization mechanism:
- **Project-specific instincts** (`personal/`) capture this project's conventions
- **Global instincts** (`~/.evolve/instincts/personal/`) capture cross-project preferences
- **Instinct graduation** promotes recurring patterns from episodic to semantic memory

The gap: instincts currently capture *what works* but not *what the user prefers*. PPP suggests adding a preference dimension: "user prefers bundled PRs over many small ones" is a personalization instinct, not a technical instinct.

---

## Anti-Patterns

| Anti-Pattern | Risk | Mitigation |
|-------------|------|------------|
| Over-personalization | Agent optimizes for user habits, ignoring best practices | Instinct confidence scoring — low-confidence preferences don't override high-confidence conventions |
| Stale preferences | User preferences change but old instincts persist | Temporal decay (existing mechanism in instinct lifecycle) |
| Preference hallucination | Agent assumes preferences not supported by evidence | Require 2+ cycle confirmations before promoting a preference instinct |

---

## Research References

- PPP Framework (arXiv:2511.02208): multi-objective RL for Productivity + Proactivity + Personalization
- Personalized Agents Survey (arXiv:2602.22680): 4-capability taxonomy for personalized LLM agents

See [research-paper-index.md](research-paper-index.md) for the full citation index.
