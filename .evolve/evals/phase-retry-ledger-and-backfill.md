---
score_cap:
  - criterion: "an ErrArtifactTimeout retry appends a ledger entry kind=phase_retry, role=<phase>, exit_code=81"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestPhaseRetryLedgerEntry$' -v ./internal/core/ 2>&1 | grep -q 'PASS: TestPhaseRetryLedgerEntry'"
  - criterion: "backfillArtifact recovers stdout.log content >=200 chars after the phase header (true) and rejects <200 (false)"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestBackfillArtifact(RecoversAt200Chars|RejectsBelow200Chars)$' -v ./internal/core/ 2>&1 | grep -c 'PASS: TestBackfillArtifact' | grep -qx 2"
  - criterion: "with EVOLVE_BACKFILL_ENABLED unset, retry-exhaustion aborts as before (opt-in; default path untouched)"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestBackfillDisabledByDefault$' -v ./internal/core/ 2>&1 | grep -q 'PASS: TestBackfillDisabledByDefault'"
---

# Eval: phase_retry ledger entries + artifact backfill

> Pins two self-heal robustness features added in cycle 164:
> (1) every `ErrArtifactTimeout` retry now appends a `kind="phase_retry"` ledger
> entry (forensic visibility — the gap cycle-162's audit flagged: retries were
> stderr-only), and (2) an opt-in `backfillArtifact` that, on retry exhaustion,
> salvages a phase's report from its `stdout.log` when >=200 chars follow the
> phase header (the cycle 154-160 agy "agent produced content but the Write tool
> timed out" pattern). The 200-char gate is the load-bearing guard against
> promoting empty/truncated output, so the negative (<200 rejected) test carries
> equal weight to the happy path.
>
> Source incident: cycles 154-160 (agy ArtifactTimeout, exit=81) + cycle 162
> (retries forensically invisible in the ledger).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| retry-ledger | phase_retry entry kind/role/exit_code=81 | 7/10 | `go test -run TestPhaseRetryLedgerEntry -v \| grep PASS` |
| backfill-gate | recover(>=200)=true AND reject(<200)=false | 7/10 | both `TestBackfillArtifact*` PASS (count==2) |
| default-off | unset env => abort, backfill not consulted | 6/10 | `go test -run TestBackfillDisabledByDefault -v \| grep PASS` |
