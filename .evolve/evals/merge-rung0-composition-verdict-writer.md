---
score_cap:
  - criterion: "A clean composition (unchanged patch-id, green composed-tree gates) carries the audit forward: the writer appends a composition-verdict entry and verifyAuditBinding accepts it"
    max_if_missing: 3
    evidence: "cd go && go test -tags integration -count=1 -run 'TestTrivialRebaseWriter_CarriesAuditForward$' ./internal/phases/ship/"
  - criterion: "Patch-id drift, a real rebase conflict (clean abort, no partial repo state), and any red composed-tree gate each decline with ZERO ledger writes, falling back to full re-audit"
    max_if_missing: 4
    evidence: "cd go && go test -tags integration -count=1 -run 'TestTrivialRebaseWriter_(PatchIdDriftFallsBackToReaudit|ConflictedRebaseAborts|ComposedGatesMustAllBeGreen)$' ./internal/phases/ship/"
  - criterion: "The writer's entry is kernel-recomputable by the UNMODIFIED ledger verifier: persisted diff artifacts re-derive the recorded patch_id, and tampering an artifact breaks the chain"
    max_if_missing: 4
    evidence: "cd go && go test -tags integration -count=1 -run 'TestTrivialRebaseWriter_KernelVerifiesWriterEntry$' ./internal/phases/ship/"
  - criterion: "The existing ledger-side kernel-verify regression tests pass without any verifier modification"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestVerify_CompositionVerdict' ./internal/adapters/ledger/"
---

# Eval: Composition-verdict writer — rung-0 trivial-rebase carry-forward

> Pins the WRITER half of merge ladder RUNG 0 (campaign
> merge-efficiency-2026-07). Cycle-786 shipped the reader
> (`tryTrivialRebaseCarryForward`) and the kernel verifier
> (`verifyCompositionLine`), but nothing ever writes a `composition-verdict`
> ledger entry, so the fast path is dead code and every moved-HEAD ship still
> hard-fails into a full re-audit (source incident: cycle-789 scout — audit
> ran ~10x over 8 cycles). This eval keeps the writer load-bearing: on
> `AUDIT_BINDING_HEAD_MOVED` it must attempt the composition, prove the
> patch-id unchanged, run the FULL composed-tree gate set
> (`ciparity.RequiredComposedGates`, ADR-0069), persist both diff artifacts,
> and append an entry the unmodified reader and `evolve ledger verify` both
> accept — deterministic, kernel-recomputable, zero LLM tokens
> (knowledge-base/research/merge-concurrency-2026 finding #1: verdicts follow
> the CHANGE, gates follow the TREE).

## Acceptance Criteria

### C1 — Clean composition carries the audit forward [code]

```bash
cd go && go test -tags integration -count=1 -v -run 'TestTrivialRebaseWriter_CarriesAuditForward$' ./internal/phases/ship/
```

Expected: `--- PASS: TestTrivialRebaseWriter_CarriesAuditForward`

### C2 — Drift / conflict / red-gate rejections decline with zero writes [code]

```bash
cd go && go test -tags integration -count=1 -v -run 'TestTrivialRebaseWriter_(PatchIdDriftFallsBackToReaudit|ConflictedRebaseAborts|ComposedGatesMustAllBeGreen)$' ./internal/phases/ship/
```

Expected: all three report `--- PASS`

### C3 — Writer entry is kernel-recomputable by the unmodified verifier [code]

```bash
cd go && go test -tags integration -count=1 -v -run 'TestTrivialRebaseWriter_KernelVerifiesWriterEntry$' ./internal/phases/ship/
```

Expected: `--- PASS: TestTrivialRebaseWriter_KernelVerifiesWriterEntry`

### C4 — Ledger-side kernel regression suite untouched and green [code]

```bash
cd go && go test -count=1 -v -run 'TestVerify_CompositionVerdict' ./internal/adapters/ledger/
```

Expected: `--- PASS: TestVerify_CompositionVerdictKind_KernelRecompute` and `--- PASS: TestVerify_CompositionVerdict_MissingArtifactBreaksChain`

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| happy-path carry-forward | writer entry accepted by reader, no auditor re-dispatch | 3/10 | `go test -run TestTrivialRebaseWriter_CarriesAuditForward` |
| rejection trio (anti-no-op) | drift / conflict / red gate → decline, zero writes, clean abort | 4/10 | `go test -run TestTrivialRebaseWriter_(PatchIdDrift…\|Conflicted…\|ComposedGates…)` |
| kernel recompute end-to-end | writer artifacts satisfy unmodified `ledger verify`; tamper breaks chain | 4/10 | `go test -run TestTrivialRebaseWriter_KernelVerifiesWriterEntry` |
| verifier untouched | ledger-side kernel regression suite green with zero verifier edits | 5/10 | `go test -run TestVerify_CompositionVerdict ./internal/adapters/ledger/` |
