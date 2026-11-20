---
score_cap:
  - criterion: "A console-confirmed premise resets the premise_drift window to the later of created_at and premise_verified_at"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC2546_001_ConsoleVerificationResetsTheDriftWindow$' ./acs/cycle2546"
  - criterion: "A stale drop's cited sha or file is verified on the host and recorded as evidence_verified in the routed item"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC2546_002_StaleDropEvidenceIsCheckedOnTheHost$' ./acs/cycle2546"
  - criterion: "The same id stale-dropped twice emits the INBOX_STALE_DROP_REPEATED escalation and stays operator-owned"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC2546_004_SecondStaleDropOfTheSameIdEscalates$|^TestC2546_005_EscalationNeedsTheSameIdStaleDroppedTwice$' ./acs/cycle2546"
  - criterion: "An absent or invalid drop must cite the scout-report lines that name the id; the host checks the citation"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC2546_006_AbsentDropMustCiteTheScoutLinesThatNameTheId$' ./acs/cycle2546"
---

# Eval: Premise drift anchor and host-verified drop evidence

> Pins four contracts of premise-verification-anchor: the drift window resets at
> the console's premise_verified_at, a stale drop's cited evidence is checked on
> the host (evidence_verified), a second stale drop of one id escalates through
> a coded signal, and an absent or invalid drop must cite the scout-report lines
> that name the id. Source incident: cycle 1571 audit H1 (zero Triage reads
> while the id appeared seven times in the Scout report) and the F40 review.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| drift-reset | window starts at the later of filing and verification | 5/10 | `TestC2546_001` |
| host-verified-evidence | evidence_verified true or false per cited sha and file | 5/10 | `TestC2546_002` |
| repeat-escalation | second stale drop of one id emits INBOX_STALE_DROP_REPEATED | 6/10 | `TestC2546_004`, `TestC2546_005` |
| cited-scout-lines | absent or invalid drop cites the id's scout-report lines | 6/10 | `TestC2546_006` |
