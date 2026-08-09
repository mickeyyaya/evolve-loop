---
score_cap:
  - criterion: "The blocking diagnostic for an unparseable defect-dispositions.json names the expected schema inline — the dispositions/id/status/evidence/reason vocabulary plus the two legal statuses FIXED and DEFERRED — alongside the underlying parse error"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -v -run '^TestClassify_DispositionUnparseableErrorNamesSchema$' ./internal/phases/audit | grep -q '^--- PASS: TestClassify_DispositionUnparseableErrorNamesSchema'"
  - criterion: "The absent-file branch keeps its own distinct disposition-preflight: MISSING marker and is never relabelled as a parse failure"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -v -run '^TestClassify_DispositionMissingDiagnosticNotRelabelledUnparseable$' ./internal/phases/audit | grep -q '^--- PASS: TestClassify_DispositionMissingDiagnosticNotRelabelledUnparseable'"
---

# Eval: Unparseable disposition file states the schema inline

> When `readDispositions` rejects a malformed `defect-dispositions.json` it
> emits the raw `encoding/json` error — "cannot unmarshal number into Go struct
> field .dispositions.evidence of type string"
> (`go/internal/phases/audit/defect_ledger.go:667-670`). The agent that must
> re-author the file on the next dispatch does not read Go type errors, so the
> diagnostic names the failure without naming the remedy and the next attempt is
> another guess. Source incident: inbox
> `defect-disposition-contract-unsatisfiable` part (c); the guessing it causes is
> the cycle-1397/1399/1400 chain.
>
> The negative criterion is load-bearing. An absent file and an unreadable file
> are different findings with different operator actions — author it vs fix it —
> and the absent branch already has its own named marker
> (`disposition-preflight: MISSING`). Smearing the parse-failure text across both
> would send the operator after the wrong remedy, so it caps nearly as high as
> the feature itself.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| schema-inline | rejection carries the field vocabulary + FIXED/DEFERRED | 6/10 | `go test -run TestClassify_DispositionUnparseableErrorNamesSchema` |
| branch-distinctness | MISSING branch keeps its marker, never reported unparseable | 5/10 | `go test -run TestClassify_DispositionMissingDiagnosticNotRelabelledUnparseable` |
