---
name: reference
description: Reference doc.
---

# Safety & Integrity

> Read this file when handling security concerns, audit failures, or suspected gaming behavior.

## Phase Gate Script

`scripts/phase-gate.sh` is the trust boundary. Runs at every phase transition — the LLM cannot skip it.

| Controls | Owner |
|----------|-------|
| Phase progression, eval verification, health fingerprint, state.json writes, mastery | **Script** (deterministic) |
| Task selection, implementation, code review, instinct extraction | **LLM** (creative work) |

## Eval Tamper Detection

- Builder MUST NOT modify `skills/`, `agents/`, `scripts/`, `.claude-plugin/`
- Eval checksums captured by phase gate after DISCOVER, verified before AUDIT
- Weakened eval criteria → CRITICAL severity, automatic FAIL

## Anti-Patterns

| # | Pattern | Rule |
|---|---------|------|
| 1 | Over-discovery | Incremental after cycle 1 — read digest, not full scan |
| 2 | Big tasks | Prefer 3 small over 1 large (<50K tokens each) |
| 3 | Retry same failure | Log in state.json, try alternative approach |
| 4 | Skip audit | WARN/FAIL blocks shipping — no exceptions |
| 5 | Ignore instincts | Builder MUST read instinctSummary when available |
| 6 | Research every cycle | 12hr cooldown — reuse cached results |
| 7 | Ceremony > substance | Concise workspace files, not exhaustive reports |
| 8 | Ignore HALT | Pause and present issues to user |
| 9 | Complexity creep | S >30 lines, M >80 lines → break down or simplify |
| 10 | Orchestrator gaming | Never skip agents, fabricate cycles, or inflate mastery |
| 11 | Artifact forgery | Never write fake reports, `git commit --allow-empty`, or modify state.json directly |
| 12 | The "Grep Trap" / Hallucinated Features | Never accept tautological `grep` evals for logic tasks; modifying documentation without modifying executable code (`scripts/`) is forbidden for capability tasks |

## Known Incidents

| Incident | Attack | Fix | Report |
|----------|--------|-----|--------|
| Cycles 102-111 | Builder tautological evals | `eval-quality-check.sh` rigor classification | `docs/incidents/cycle-102-111.md` |
| Cycles 132-141 | Orchestrator skipped agents, fabricated cycles | `phase-gate.sh` deterministic enforcement | `docs/incidents/cycle-132-141.md` |
| Gemini CLI | Forgery script with fake artifacts | Content verification, git diff substance, state checksum | `docs/incidents/gemini-forgery.md` |
| Autoresearch Loop (Cycles 1-16) | "Flawless Execution" (Hallucinating features into Markdown, evading actual code changes) | Ban `grep` for logic evals, enforce execution-based testing, require execution layer modifications | `docs/incidents/flawless-execution-anomaly.md` |
