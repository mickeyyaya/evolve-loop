---
score_cap:
  - criterion: "The persona/profile pairing gate stays green when an UNTRACKED runtime-minted profile stub is present on the tree"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -run '^TestRepoPersonaProfilePairing$' ./internal/phasecoherence"
  - criterion: "The PR #421 tracked-vs-untracked fixture regression (unpaired_tracked_test.go) still exists and passes"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestTrackedProfiles_' ./internal/phasecoherence"
  - criterion: "The cd49274beab2 pipeline-blocker inbox item is consumed with a durable record naming the incident fingerprint"
    max_if_missing: 6
    evidence: "grep -rl cd49274beab2 .evolve/inbox/consumed/ >/dev/null"
  - criterion: "The consumed record's verification block cites the merged fix commit 7a42d30b"
    max_if_missing: 5
    evidence: "grep -rl 7a42d30b .evolve/inbox/consumed/ >/dev/null && git merge-base --is-ancestor 7a42d30b HEAD"
---

# Eval: verify-pipeline-blocker-fix-421

> Pins the terminal step of the cd49274beab2 halt/fix/verify chain. The
> `ship|gate-block|cd49274beab2` fingerprint recurred 3× in one batch (cycles
> 1402/1403/1405), tripping the ADR-0072 identical-fingerprint ceiling and
> halting the whole batch: `TestRepoPersonaProfilePairing` Direction B was
> binding the untracked runtime-minted stub
> `.evolve/profiles/defect-disposition-ledger.json` on the live plane, so every
> audit-green lane ship was blocked by the ship-time repo-contract pack. PR #421
> (commit `7a42d30b`) fixed it by binding only git-TRACKED profiles via
> `trackedProfiles()` (git ls-files, stderr-surfaced errors, loud empty-set
> fallback to strict bind-all). This eval keeps the gate honest after the item
> is consumed: the first two caps re-prove the runtime behavior on every future
> cycle, and the last two keep the verification evidence — not merely the merge
> — in the durable audit trail so a forensics sweep never has to re-derive
> whether #421 was actually verified live. Source incident: cycle 1406 halt
> (item `pipeline-defect-pipeline-blocker`, filed 2026-08-09T20:51:57Z);
> verification cycle 1410.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| untracked-mint-tolerated | Pairing gate green with an untracked minted profile stub present | 9/10 | `go test -run '^TestRepoPersonaProfilePairing$' ./internal/phasecoherence` |
| fixture-regression-alive | PR #421 tracked/untracked fixtures still execute and pass | 8/10 | `go test -run '^TestTrackedProfiles_' ./internal/phasecoherence` |
| incident-consumed | Consumed record exists and names fingerprint cd49274beab2 | 6/10 | `grep -rl cd49274beab2 .evolve/inbox/consumed/` |
| evidence-cites-merge | Verification cites 7a42d30b and it is an ancestor of HEAD | 5/10 | `grep -rl 7a42d30b .evolve/inbox/consumed/ && git merge-base --is-ancestor 7a42d30b HEAD` |
