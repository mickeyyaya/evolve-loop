## 2026-11-27 — Deliverable verification: a static check and an authentic check for every deliverable

| Document | What it records |
|---|---|
| [research/deliverable-verification-2026-11.md](research/deliverable-verification-2026-11.md) | A matrix of 15 deliverable classes with their static checks and their authenticity gaps. It holds 11 local findings with `path:line` and counts. It holds 8 outside findings on FAIL_TO_PASS, mutation in review and LLM-test oracles. It adds 9 adopt-or-reject decisions, 6 refinements and 6 decision entries. |
| [plans/deliverable-verification-2026-11.md](plans/deliverable-verification-2026-11.md) | The plan. It scores three candidate designs and picks a verification registry with a meta-guard, then staged authenticity gates. It lists 12 decisions, the components V0 to V6, the execution order, the targets and the operator questions about gates that only warn. |
| [architecture/adr/0172-every-deliverable-has-a-registered-static-check-and-an-authentic-check.md](architecture/adr/0172-every-deliverable-has-a-registered-static-check-and-an-authentic-check.md) | ADR-0172, Proposed. It gives 19 decision rules, the confirmation by the meta-guard, the good and bad consequences, and the rejected options. |
| [architecture/deliverable-verification.md](architecture/deliverable-verification.md) | The spec of record. It defines the classes, the registry schema and the five rules of the meta-guard. It gives the designs of V1 to V6 with their stage, the exemption marker, a worked example of one PR and a table of guards. |
