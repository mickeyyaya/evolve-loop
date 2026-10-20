## 2026-10-09 — a retry carries the evidence of the dispatch that it replaces (ADR-0130)

| Document | What it records |
|---|---|
| [research/attempt-postmortem-2026-10.md](research/attempt-postmortem-2026-10.md) | Research: the cycle 1853 chain, the records that the repository writes today, and how agent harnesses carry failure context into a retry. It covers Reflexion, SWE-agent, OpenHands, Claude Code, Aider, LangGraph and same-input retry. Findings F1.1 to F3.10, refinements R1 to R9. |
| [plans/attempt-postmortem-2026-10.md](plans/attempt-postmortem-2026-10.md) | Plan: four candidate designs with scores, decisions D1 to D18, components P1 to P5, the wiring lane W1 and the repeat guard G1 (R3). |
| [architecture/adr/0130-a-retry-carries-the-evidence-of-the-attempt-it-replaces.md](architecture/adr/0130-a-retry-carries-the-evidence-of-the-attempt-it-replaces.md) | Decision: each abnormal end of a phase dispatch leaves a postmortem record. The next dispatch or a resume gets a section from the bridge with a do-not-repeat rule. |
