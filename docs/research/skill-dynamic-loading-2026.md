# Skill Dynamic Loading & Context Discipline — 2026-05 Research Dossier

> Archive note: Persistent reference for cycle-24+ Scouts.
> Conducted 2026-05-12 (Cycle 24). All citations verified via WebSearch/WebFetch.
> Cross-checked against token-reduction-roadmap.md — no duplicates of P1-P8/P-NEW-1-6.

## Source 11: SkillReducer
arXiv:2603.29919 | CS > Software Engineering | March 31, 2026
URL: https://arxiv.org/abs/2603.29919
Key: 48% description compression + 39% body compression + 2.8% quality improvement.
Mechanism: adversarial delta debugging (routing) + taxonomy-driven progressive disclosure (body).
Root causes: missing routing descriptions, non-actionable content, oversized reference files.
Relevance: phases.md (28,911 bytes) is 14.5x the recommended activation-layer size.

## Source 12: AgentDiet
arXiv:2509.23586 | FSE 2026 | Submitted 2025-09-28
URL: https://arxiv.org/abs/2509.23586
Key: 39.9-59.7% input token reduction, 21.1-35.9% total cost reduction, no performance loss.
Mechanism: inference-time removal of expired/redundant/useless trajectory information.
Relevance: failedApproaches[] (6,455 bytes, 14 entries) is "expired" for Builder — all audit-domain.

## Source 13: OpenHands Context Condensation
Blog: https://openhands.dev/blog/openhands-context-condensensation-for-more-efficient-ai-agents
Paper: arXiv:2511.03690 | November 2025
Key: 50% per-turn cost reduction; quadratic→linear context scaling; 54% vs 53% SWE-bench parity.
Mechanism: LLMSummarizingCondenser — replaces history with summaries at threshold.
Relevance: orchestrator accumulates ~50KB in one cycle (phase reports); P-NEW-9 applies this.

## Source 14: addyosmani/agent-skills
GitHub: https://github.com/addyosmani/agent-skills | October 2025
Key: Discovery layer ~80 tokens/skill; activation ~2,000 tokens; execution on-demand.
Ecosystem: 40,285 skills listed within 20 days of standard release (Bosch/CMU study Jan 2026).
Relevance: evolve-loop skills are 10-15x the recommended activation-layer size.

## Source 15: SoK: Agentic Skills
arXiv:2602.20867 | CS > Security | February 24, 2026
URL: https://arxiv.org/abs/2602.20867
Key: 7 design patterns; ClawHavoc campaign (~1,200 malicious skills, exfiltrating API keys).
Relevance: anti-gaming context for moving logic to skill files — evolve-loop defenses sufficient.

## Evolve-Loop Measurements (Cycle 24 baseline)
- phases.md: 28,911 bytes (28KB, 550 lines)
- SKILL.md: 19,163 bytes (19KB)
- online-researcher.md: 18,642 bytes (18KB)
- benchmark-eval.md: 21,858 bytes (21KB)
- phase6-learn.md: 25,840 bytes (25KB)
- failedApproaches[]: 6,455 bytes (14 entries; 10=code-audit-warn, 4=unknown-classification)
- instinctSummary[]: 847 bytes (5 entries — small, not primary target)

## Top-3 Candidates (new, not in existing P1-P8/P-NEW-1-6)
P-NEW-7: SkillReducer-style Layer-3 split for phases.md + large skills — saves 16KB/load, $0.10-0.40/cycle
P-NEW-8: AgentDiet classification filter for failedApproaches — saves ~6KB Builder context/cycle, $0.03-0.10/cycle
P-NEW-9: OpenHands-style phase-report summarization in orchestrator — saves ~40KB orchestrator context, $0.10-0.30/cycle
