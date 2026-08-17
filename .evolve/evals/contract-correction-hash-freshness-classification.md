---
score_cap:
  - criterion: "contractArtifactDetermined allowlists exactly the six artifact-determined violation codes"
    max_if_missing: 7
    evidence: "go -C go test -count=1 -run '^TestContractArtifactDetermined_Table$' ./internal/core"
  - criterion: "contractArtifactDetermined fails closed: stray_in_worktree and unknown codes return false"
    max_if_missing: 8
    evidence: "go -C go test -count=1 -run '^TestContractArtifactDetermined_StrayInWorktreeIsFalse$' ./internal/core"
  - criterion: "the allowlist stays pinned to the real deliverable code vocabulary (exactly 6 of 9)"
    max_if_missing: 6
    evidence: "go -C go test -count=1 -run '^TestContractArtifactDetermined_CodesMatchDeliverableVocabulary$' ./internal/core"
---

# Eval: contract-correction hash-freshness classification

> Pins the correction ladder's artifact-determined violation-code classifier,
> `contractArtifactDetermined(code string) bool` in
> `go/internal/core/contract_escalation.go`. The classifier answers exactly one
> question: does repairing this violation NECESSARILY change the bytes of the
> watched deliverable artifact? Only when the answer is yes is an unchanged
> artifact hash sound evidence that "no new work was done". Six codes qualify —
> `missing_artifact`, `empty_artifact`, `missing_section`, `bad_verdict`,
> `missing_key`, `missing_challenge_token` — because their repair IS an edit to
> the artifact.
>
> `deliverable.CodeStrayInWorktree` is the counter-example the whole primitive
> exists for: it is repaired by DELETING a stray worktree copy, leaving the
> watched artifact byte-identical. A hash-equality short-circuit that does not
> consult this classifier converts a repairable contract block into a
> deterministic ladder abort.
>
> Source incident: cycle-1508 audit FAIL, defects H1/L1 (`inst-L1508a`) — a
> proposed hash-of-artifact-bytes freshness classifier that skipped re-consulting
> the gate for location-class violations. Re-scoped at cycle-1510 to the
> classifier alone (no speculative consumer).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| allowlist-positives | All six artifact-determined codes return true | 7/10 | `go test -run TestContractArtifactDetermined_Table ./internal/core` |
| fail-closed-negatives | `stray_in_worktree` and unknown codes return false — an allowlist, never a denylist | 8/10 | `go test -run TestContractArtifactDetermined_StrayInWorktreeIsFalse ./internal/core` |
| vocabulary-drift | Exactly 6 of the 9 real `deliverable` codes are accepted; the two vocabularies cannot drift silently because codes cross the package boundary as plain strings (import-cycle constraint) | 6/10 | `go test -run TestContractArtifactDetermined_CodesMatchDeliverableVocabulary ./internal/core` |
