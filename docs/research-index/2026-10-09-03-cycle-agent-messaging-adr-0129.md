## 2026-10-09 — cycle agent messaging (ADR-0129)

| Document | What it records |
|---|---|
| [research/agent-messaging-2026-10.md](research/agent-messaging-2026-10.md) | Research: agent-to-agent protocols (A2A, MCP, Claude Code teams and messages, AutoGen, MetaGPT, CAMEL) and scope (NATS, MQTT, Kafka). It also covers identity under one uid, delivery to a busy LLM agent, safety and scale. Findings F1.1 to F7.5, an adopt-or-reject table, refinements R1 to R24. |
| [architecture/agent-messaging.md](architecture/agent-messaging.md) | Design, the spec of record: the `msg/1.0` envelope, the address grammar, the identity with the outbox location and the token, and the policy rules. It also covers the flow, the store on the ADR-0127 log, the delivery adapters, the limits, the `evolve msg` verbs, the paths and the threat model. |
| [plans/agent-messaging-2026-10.md](plans/agent-messaging-2026-10.md) | Plan: operator decisions OD1 to OD4, four candidate designs with scores, decisions D1 to D15 and the skeleton. It also lists the wiring seams, and components M0 to M8 with their tests. |
| [architecture/adr/0129-cycle-scoped-agent-messaging.md](architecture/adr/0129-cycle-scoped-agent-messaging.md) | Decision: the agents of one cycle send messages through a broker of that cycle, with a general protocol and a scoped policy. It amends ADR-0127 and ADR-0064. |
