---
id: commit-through-ship
statement: "Commit and push only through evolve ship. The ship gate denies a bare git commit and a bare git push."
layer: L0
order: 5
audience: [all]
enforced_by: [go/internal/guards/ship.go]
source: "AGENTS.md invariant 3"
status: active
added: 2026-10-26
evidence: "AGENTS.md invariant 3; go/internal/guards/ship.go denies a bare git commit and git push"
---
Why: Ship binds each commit to its attestation and to the hash-chained ledger.
