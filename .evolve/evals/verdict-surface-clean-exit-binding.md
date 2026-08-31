---
score_cap:
  - criterion: "A fully-valid deliverable with green audit+ACS self-heals a recorded negative verdict to PASS, never halting"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run TestC1590_001_ValidDeliverableReconcilesNotHalt -tags acs ./acs/cycle1590/"
  - criterion: "A malformed PASS-sentinel deliverable with green audit+ACS still halts as forgery — never launders"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -run TestC1590_002_MalformedDeliverableStillHalts -tags acs ./acs/cycle1590/"
  - criterion: "A missing or malformed ACS verdict never manufactures a reconcile, even with a fully-valid deliverable"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run TestC1590_003_MissingACSNeverReconciles -tags acs ./acs/cycle1590/"
  - criterion: "The runtime finalization call site binds BOTH audit- and ship-phase substantive-error evidence, and the FULL deliverable.Verify chain (not the cheap sentinel), before recording or halting"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run TestC1590_005_CallSiteBindsSubstantiveErrorAndFullVerify -tags acs ./acs/cycle1590/"
  - criterion: "The end-to-end forged-verdict halt path stays armed"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run TestC1590_006_ForgedVerdictHaltsLive -tags acs ./acs/cycle1590/"
---

# Eval: Bind clean-exit finalization to authoritative verdict evidence

> Pins the ADR-0072 verdict-coherence floor's clean-exit self-heal contract at
> its RUNTIME call site (`internal/core/system_failure.go`'s
> `detectVerdictIncoherence`), not just the pure `coherence.CheckVerdictCoherence`
> comparison. Source incident: the P0 pipeline-blocker halt at cycle 1582 (3
> consecutive failed cycles, mixed failure identities, `.evolve/inbox/
> pipeline-defect-pipeline-blocker-cycle1582.json`) — the recorded cycle
> verdict was negative while `audit-report.md` and `acs-verdict.json` were
> both green, and the scout's leading hypothesis (confidence 0.82) was that a
> finalization caller could supply incomplete `VerdictInputs`. This eval
> permanently pins BOTH directions of the anti-laundering boundary
> `coherence.go` documents: a genuinely benign clean-exit-late-write race
> (green artifacts + a fully-verified deliverable) must self-heal to PASS without
> halting, while a malformed report merely tagged with a PASS sentinel, or a
> missing/non-PASS ACS artifact, must never be laundered into a reconcile —
> and that the ship-phase's `ShipFailReasons`, not just the audit phase's
> `AuditFailReasons`, feed `SubstantiveError` so a diagnosed ship-gate
> downgrade is never mistaken for pipeline forgery (the cycles-930/931/932
> false-HALT family this floor already fixed once).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| self-heal-happy | valid deliverable + green artifacts reconciles, no halt | 8/10 | `TestC1590_001_ValidDeliverableReconcilesNotHalt` |
| anti-laundering-negative | malformed report still halts as forgery | 9/10 | `TestC1590_002_MalformedDeliverableStillHalts` |
| anti-laundering-edge | missing/malformed ACS never manufactures a reconcile | 7/10 | `TestC1590_003_MissingACSNeverReconciles` |
| call-site-binding | runtime binds ship+audit substantive-error and full Verify | 8/10 | `TestC1590_005_CallSiteBindsSubstantiveErrorAndFullVerify` |
| forged-halt-armed | end-to-end forged-verdict halt path stays live | 6/10 | `TestC1590_006_ForgedVerdictHaltsLive` |
