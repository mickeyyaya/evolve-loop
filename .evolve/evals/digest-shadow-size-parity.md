---
score_cap:
  - criterion: "Every dispatch records full-versus-digest byte counts and a parity verdict, emitted through the runner's diagnostics sink"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1460_004_DigestShadowRecordsByteCountsAndParityVerdict ./acs/cycle1460"
  - criterion: "Empty, malformed, and non-reducing projections preserve the live prompt and cannot record a saving"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1460_005_DigestShadowUnsafeProjectionsPreserveLivePromptAndCannotClaimSaving ./acs/cycle1460"
  - criterion: "No agent profile enables a pre-generated digest before a recorded telemetry baseline"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC1460_006_DigestShadowNoProfileDefaultFlipBeforeBaseline ./acs/cycle1460"
---

# Eval: Shadow size and parity evidence for the digest rollout

> Pins the rollout-safety half of `tokenopt-role-scoped-instruction-digests`.
> The inbox item
> (`.evolve/inbox/2026-07-07T09-30-00Z-tokenopt-role-scoped-instruction-digests.json:20`)
> gates any default flip on an A/B quality check, but that gate was prose-only:
> nothing recorded how many bytes a projection actually saved, and nothing
> stopped a broken or empty projection from being counted as a win. This eval
> makes the falsifiable version permanent — `ShadowRecord` carries exact
> `FullBytes`/`DigestBytes` and a `Parity` verdict that is **false** for every
> unsafe shape (no-match, malformed, matched-but-not-strictly-smaller), the
> live prompt is preserved unchanged in each of those cases, and the telemetry
> line is emitted for every dispatch including the fail-closed one so a broken
> marker is counted rather than dropped.
>
> `Parity == true` is a mechanical size-and-well-formedness claim only; it is
> explicitly NOT proof of behavioral equivalence, and no profile may set
> `digest_file` until a real telemetry baseline exists.
>
> Source incident: cycle 1460 (scout finding "Rollout safety — HIGH").

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| shadow-evidence | Byte counts + parity verdict recorded and emitted by the live dispatch path | 7/10 | `go test -tags acs -run TestC1460_004_...` |
| no-false-saving | Empty / malformed / non-reducing projections keep the live prompt and report `parity=false` | 8/10 | `go test -tags acs -run TestC1460_005_...` |
| no-premature-flip | No profile sets `digest_file` before the baseline is recorded | 5/10 | `go test -tags acs -run TestC1460_006_...` |
