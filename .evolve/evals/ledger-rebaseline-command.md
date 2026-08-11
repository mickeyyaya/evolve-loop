---
score_cap:
  - criterion: "evolve ledger rebaseline seals a densely damaged prefix in ONE call and leaves verify --deep green"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run 'TestRebaseline_SealsDamagedPrefix' ./internal/adapters/ledger/"
  - criterion: "rebaseline is non-destructive: pre-rebaseline bytes remain a byte-identical prefix of ledger.jsonl"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run 'TestRebaseline_PreservesDamagedPrefixBytes' ./internal/adapters/ledger/"
  - criterion: "rebaseline is operator-gated and refuses an empty chain rather than fabricating a record"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestRebaseline_Refuses' ./internal/adapters/ledger/"
---

# Eval: evolve ledger rebaseline seals a damaged prefix in one operation

> Repairing a densely damaged ledger with `evolve ledger anchor` costs one
> operator invocation per break. The runtime-plane ledger needed 55 sequential
> anchors to go green; the console-plane ledger (~180+ breaks from pre-CA.1
> fleet-concurrency damage, broken since line 78729) was abandoned and LEFT
> BROKEN BY DESIGN because no single-operation repair existed. Source incident:
> inbox item `ledger-fleet-concurrency-chain` root-cause (c), cycle-1433.
>
> This eval pins the three properties that make `rebaseline` a repair tool rather
> than a chain-integrity bypass: it works in ONE call, it destroys nothing (the
> damaged prefix stays byte-identical on disk, per ADR-0048's preservation
> requirement), and it refuses to act without an operator sign-off or on an empty
> chain — a command that seals whatever it is pointed at whenever it is invoked
> would silence Verify rather than repair it.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| one-call-repair | A multi-break fixture goes RED → GREEN under `verify --deep` after ONE rebaseline | 8/10 | `go test -run TestRebaseline_SealsDamagedPrefix ./internal/adapters/ledger/` |
| non-destructive | Pre-rebaseline bytes remain a byte-identical prefix (no truncation, no rewrite) | 8/10 | `go test -run TestRebaseline_PreservesDamagedPrefixBytes ./internal/adapters/ledger/` |
| operator-gate | Missing operator note and empty-chain invocations are refused, writing nothing | 7/10 | `go test -run TestRebaseline_Refuses ./internal/adapters/ledger/` |
