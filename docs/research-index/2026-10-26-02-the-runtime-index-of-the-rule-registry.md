## 2026-10-26 — the rule registry gets a runtime index that each CLI family reads on demand (ADR-0145 amendment)

| Document | What it records |
|---|---|
| [research/guidance-layers-2026-10.md](research/guidance-layers-2026-10.md) §5 | Research: the instruction file of Claude Code, Gemini CLI, Antigravity, Codex and Ollama, the `AGENTS.md` standard and load on demand (S12 to S18). The X10 `AGENTS.md` is over the Codex cap of 32 KiB (F5.2). |
| [plans/guidance-layers-2026-10.md](plans/guidance-layers-2026-10.md) §9 | Plan: the extraction inventory of each rule source and of the 143 memory rules. Four scored designs, decisions D12 to D19, components G8 to G10 and the first L0 batch. |
| [architecture/guidance-layers.md](architecture/guidance-layers.md) §10 | Spec: `guidance/index.md`, `guidance/index.json`, the L0 region of `AGENTS.md` and the verb `evolve guidance index` with its exit codes. Family facts stay in the driver layer. |
| [architecture/adr/0145-each-rule-has-one-home-and-loads-by-audience-path-action-or-failure.md](architecture/adr/0145-each-rule-has-one-home-and-loads-by-audience-path-action-or-failure.md) | Decision: points 8 to 10 add the runtime index. |
