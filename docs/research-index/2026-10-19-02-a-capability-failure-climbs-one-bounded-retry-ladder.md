## 2026-10-19 — a capability failure climbs one bounded retry ladder (ADR-0142)

| Document | What it records |
|---|---|
| [research/retry-ladder-2026-10.md](research/retry-ladder-2026-10.md) | Research: about half of the build correction rounds of cycles 1800 to 1970 fail again, and no round raises the tier or the effort. The builder pin turns off the raises that exist. It also surveys model cascades, effort guidance and self-repair limits. Findings F2.1 to F2.12 and F3.1 to F3.10. |
| [plans/retry-ladder-2026-10.md](plans/retry-ladder-2026-10.md) | Plan: five designs with scores. The winner is one retry ladder with a deterministic classifier, three rungs and a stop rule. Decisions D1 to D14, components L0 to L8 and four open decisions. |
| [architecture/adr/0142-a-capability-failure-climbs-one-bounded-retry-ladder.md](architecture/adr/0142-a-capability-failure-climbs-one-bounded-retry-ladder.md) | Decision (Proposed): only a capability failure climbs. It takes one effort step and then one tier step. Two tiers that fail on one check stop as a spec failure. |
| [architecture/retry-ladder.md](architecture/retry-ladder.md) | Design, the spec of record: the flow, the classifier signals and the rungs for each family. It also gives the retry paths, the try record, the policy keys and the signals. |
