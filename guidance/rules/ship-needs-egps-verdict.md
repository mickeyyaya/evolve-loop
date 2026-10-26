---
id: ship-needs-egps-verdict
statement: "Ship only when the EGPS verdict has a red count of zero. Never edit the hash-chained ledger by hand."
layer: L0
order: 7
audience: [all]
enforced_by: []
source: "AGENTS.md invariants 5 and 6"
status: active
added: 2026-10-26
evidence: "AGENTS.md invariants 5 and 6; evolve ledger verify checks the hash chain"
---
Why: A ship without a green verdict ships an unverified change. A hand edit breaks the chain that proves the history. Run `evolve ledger verify` to check the chain.
