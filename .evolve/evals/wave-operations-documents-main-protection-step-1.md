---
score_cap:
  - criterion: "wave-operations.md states the live protection of main: step 1 applied 2026-11-24 with the five settings, the gh api read-back, and step 2 waiting for I9"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2636_001|TestC2636_002|TestC2636_003' ./acs/cycle2636"
  - criterion: "wave-operations.md names the enforce_admins=false consequence for the console, the lanes and the train"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2636_004' ./acs/cycle2636"
  - criterion: "The new protection section passes the STE lint and the rest of the document stays intact"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2636_005|TestC2636_006' ./acs/cycle2636"
---

# Eval: wave-operations.md documents main protection step 1

> Pins the operator-facing statement of the live branch protection of main in
> `docs/operations/wave-operations.md`: what step 1 set on 2026-11-24, how to
> read it back, why step 2 waits for I9, and what `enforce_admins=false` means
> for the admin identities the console, the lanes and the train use.
> Source item: wave-operations-documents-main-protection-step-1 (cycle 2636).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| live-state | Step 1 settings, read-back and step 2 gate are in the doc | 5/10 | `go test -tags acs -run 'TestC2636_001|TestC2636_002|TestC2636_003' ./acs/cycle2636` |
| admin-consequence | enforce_admins=false consequence is named | 5/10 | `go test -tags acs -run TestC2636_004 ./acs/cycle2636` |
| ste-and-preservation | Section is STE-clean, existing sections kept | 7/10 | `go test -tags acs -run 'TestC2636_005|TestC2636_006' ./acs/cycle2636` |
