## 2026-10-25 — the CLI is the service interface, and the pipeline reads no ambient control flags (ADR-0156)

| Document | What it records |
|---|---|
| [research/llm-isolation-2026-10.md](research/llm-isolation-2026-10.md) §6 | Research: the CLI surface and the control flags, with re-verified file:line evidence (F4.1 to F5.6). Current practice (P4.1 to P4.6), and the audit facts that were wrong. |
| [plans/llm-isolation-2026-10.md](plans/llm-isolation-2026-10.md) §6.1, §6.2, §12 | Plan: the component state of X1, X2 and X6, and the follow-up X6b. Three designs with scores, the decisions D21 to D31 and the components X17a to X19c. |
| [architecture/adr/0156-the-cli-is-the-service-interface-and-the-pipeline-reads-no-ambient-control-flags.md](architecture/adr/0156-the-cli-is-the-service-interface-and-the-pipeline-reads-no-ambient-control-flags.md) | Decision: the invariants I-15 (the CLI is the service contract) and I-16 (no ambient control). |
| [architecture/cli-contract.md](architecture/cli-contract.md) | Spec of record: the envelope, the exit-code table, the availability classes and the core verbs. |
| [architecture/llm-isolation.md](architecture/llm-isolation.md) §26 to §28 | Spec Part III: the design of X17 and X18, and of X19 (ADR-0154 I-17, the CLI-neutral task assignment). |
| [architecture/control-flags.md](architecture/control-flags.md) | Reference: the five classes of the environment readers outside the registry. |
