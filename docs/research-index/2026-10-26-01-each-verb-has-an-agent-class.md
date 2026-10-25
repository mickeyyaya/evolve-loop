## 2026-10-26 — each verb has an agent class, and a cell reads its classes from one verb (ADR-0156 amendment, X17e)

| Document | What it records |
|---|---|
| [architecture/cli-contract.md](architecture/cli-contract.md) §4.1 to §4.5, §5 | Spec of record: the agent classes `A`, `R` and `D`, the verb `agent capabilities --json`, the request kinds, the credential check, the signals and three quality readings. The column "agent" of §5 is checked on `origin/main` at `5c010d7a4`. |
| [architecture/adr/0156-the-cli-is-the-service-interface-and-the-pipeline-reads-no-ambient-control-flags.md](architecture/adr/0156-the-cli-is-the-service-interface-and-the-pipeline-reads-no-ambient-control-flags.md) | Decision: rules 17 to 22 amend I-15 with the agent class of each verb. |
| [architecture/llm-isolation.md](architecture/llm-isolation.md) §26, §28 | Spec: the field "tool surface" of the neutral TaskAssignment, the CLI-neutral persona text and the component X17e. |
| [plans/llm-isolation-2026-10.md](plans/llm-isolation-2026-10.md) §12.6 | Plan: the component X17e and its dependencies on the X4 items of #1126. |
