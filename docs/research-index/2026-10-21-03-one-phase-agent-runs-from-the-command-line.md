## 2026-10-21 — one phase agent runs from the command line with no cycle (ADR-0152)

| Document | What it records |
|---|---|
| [research/agent-run-standalone-2026-10.md](research/agent-run-standalone-2026-10.md) | Research: the phase command, the spec runner and the ledger form for manual entries. Current practice: Claude Code `-p`, `codex exec`, Gemini CLI, the OpenAI Agents SDK and LangGraph. |
| [plans/agent-run-standalone-2026-10.md](plans/agent-run-standalone-2026-10.md) | Plan: four designs with scores, the decisions D1 to D11 and the components A1 to A5. |
| [architecture/adr/0152-one-phase-agent-runs-from-the-command-line-with-no-cycle-through-the-phase-runner.md](architecture/adr/0152-one-phase-agent-runs-from-the-command-line-with-no-cycle-through-the-phase-runner.md) | Decision: `evolve agent run` through the phase runner, with cycle 0, a temporary workspace and read-only profiles only. |
| [architecture/agent-run.md](architecture/agent-run.md) | Spec of record: the input and output contracts, the no-cycle rules and the profiles. |
