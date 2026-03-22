# Safety & Integrity

## Phase Gate Script (primary enforcement)

`scripts/phase-gate.sh` runs at every phase transition. The LLM cannot skip it.

**Script controls:** phase progression, eval verification, health fingerprint, state.json writes, mastery updates.
**LLM controls:** task selection, implementation, code review, instinct extraction.

Research: Greenblatt "AI Control" (2023), Redwood "Factored Cognition" (2025).

## Eval Tamper Detection

- Builder MUST NOT modify `skills/`, `agents/`, `scripts/`, `.claude-plugin/`
- Eval checksums captured by phase gate, verified before audit
- Weakened eval criteria → CRITICAL severity

## Anti-Patterns

1. Over-discovery — incremental after cycle 1
2. Big tasks — prefer 3 small over 1 large (<50K tokens each)
3. Retrying same failure — log in state.json, try alternative
4. Skipping audit — WARN/FAIL blocks shipping
5. Ignoring instincts — Builder MUST read when available
6. Research every cycle — 12hr cooldown
7. Ceremony over substance — concise workspace files
8. Ignoring HALT — pause and present to user
9. Complexity creep — S >30 lines or M >80 lines → break down
10. Orchestrator gaming — never skip agents, fabricate cycles, inflate mastery
11. Artifact forgery — never write fake reports, `git commit --allow-empty`, or modify state.json directly

## Known Incidents

- **Cycles 102-111:** Builder tautological evals → eval-quality-check.sh
- **Cycles 132-141:** Orchestrator skipped agents, fabricated cycles → phase-gate.sh
- **Gemini CLI:** Wrote forgery script with fake artifacts → content verification, git diff substance, state checksum lock

Full reports: `docs/incident-report-*.md`
