---
score_cap:
  - criterion: "Every stageevidence.Record field is hashed through hashedRecord or declared unhashed, and the guard fails when one is neither"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestRecordHashSchema_EveryRecordFieldIsHashedOrDeclaredUnhashed$' ./internal/stageevidence"
  - criterion: "computeRecordHash output changes exactly when a hashed field changes"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestRecordHashSchema_HashChangesExactlyWhenAHashedFieldChanges$' ./internal/stageevidence"
---

# Eval: stage-evidence record hash schema guard

> Pins the hash schema of `stageevidence.Record`. A new field must join `hashedRecord` or the `unhashedRecordFields` list, and `computeRecordHash` must copy each hashed field. Source: stage-evidence K2 review follow-up, cycle 2643.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| field-decision | each Record field is hashed or declared unhashed | 6/10 | `go test -run TestRecordHashSchema_EveryRecordFieldIsHashedOrDeclaredUnhashed` |
| hash-payload | hash changes exactly with hashed fields | 7/10 | `go test -run TestRecordHashSchema_HashChangesExactlyWhenAHashedFieldChanges` |
