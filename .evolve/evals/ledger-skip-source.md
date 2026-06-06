---
score_cap:
  - criterion: "LedgerEntry carries Source (json source,omitempty) and recordRoutingDecision stamps router on phase_skipped"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run TestLedgerEntrySource ./internal/core/"
  - criterion: "Entries without Source serialize without a source key (hash-chain byte stability)"
    max_if_missing: 3
    evidence: "cd go && go test -count=1 -run TestLedgerEntrySource_RoundTrip ./internal/core/"
---

# Eval: phase_skipped skip-source attribution

> Pins the skip-source attribution contract from cycle 230 (PRIORITY 0
> carryover sub-mode): `phase_skipped` ledger entries must say WHO decided the
> skip (`psmas|router|content`) so audit predicates can distinguish routing
> declines from content-gate failures. The omitempty criterion is
> load-bearing: a non-omitempty field would shift serialized bytes of
> historical entries and cascade SHA256 hash-chain breaks. Source incident:
> cycle 229 built this in its worktree but never shipped (regression-gate
> FAIL); re-pinned on main in cycle 230.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| router-stamp | Source field + router attribution on routing skips | 4/10 | `go test -run TestLedgerEntrySource ./internal/core/` |
| hash-chain-stability | omitempty keeps historical bytes unchanged | 3/10 | `go test -run TestLedgerEntrySource_RoundTrip ./internal/core/` |
