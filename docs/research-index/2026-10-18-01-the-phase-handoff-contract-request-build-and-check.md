## 2026-10-18 — the phase handoff contract: request, build and check between every phase pair (ADR-0141)

| Document | What it records |
|---|---|
| [research/phase-handoff-contract-2026-10.md](research/phase-handoff-contract-2026-10.md) | Research: cycles 1946 and 1950 failed on their request, not on their code. Three of five phase pairs have no check of the output against the ask, and the audit chain is shadow. It also surveys self-checks, independent judges and stop rules. It also covers blocked requests and attestation chains. Findings F1.1 to F1.14 and P2.1 to P2.17, refinements R1 to R10. |
| [plans/phase-handoff-contract-2026-10.md](plans/phase-handoff-contract-2026-10.md) | Plan: four approaches with weighted scores. The winner is one handoff contract: a request, a ledger, a deterministic check and then a judge, with three exits. Decisions D1 to D12 and components H1 to H12 with red tests. |
| [architecture/adr/0141-every-phase-handoff-is-a-request-build-check-loop-with-three-exits.md](architecture/adr/0141-every-phase-handoff-is-a-request-build-check-loop-with-three-exits.md) | Decision (Accepted): every phase pair runs a request, build and check loop. The exits are pass, cap and blocked. Only deterministic proof blocks. |
| [architecture/phase-handoff-contract.md](architecture/phase-handoff-contract.md) | Design, the spec of record: the pairs and their requests, the ledger schema and the two layers of the check. It also gives the exits, the chain in the audit and a worked example on cycle 1950. |
