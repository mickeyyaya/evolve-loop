---
score_cap:
  - criterion: "The phase runner derives its dispatched instruction body from the role-tagged SSOT source, not the whole document"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1460_001_DigestMaterializationInRunnerUsesRoleScopedDigest ./acs/cycle1460"
  - criterion: "Untagged prose and other-role blocks never reach the dispatched digest"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1460_002_RoleScopedDigestExcludesUntaggedAndOtherRoleContent ./acs/cycle1460"
  - criterion: "An unterminated digest marker fails before bridge.Launch, and a no-match role never receives the full source as its digest"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1460_003_DigestInjectionMalformedFailsBeforeLaunchAndNoMatchNeverYieldsFullSource ./acs/cycle1460"
---

# Eval: Materialize role-scoped instructions in the live phase runner

> Pins the production wiring of `go/internal/digest` into `BaseRunner.Run`.
> Cycle 1391 shipped `digest.ProjectDigest` and the `systemprompt` `digest_file`
> fallback, but cycle-1460 scouting found **no production caller**: no
> `ProjectDigest` call site, no `digest_file` in any live profile, and no
> `digest:role=` marker in any real instruction source. A tested-but-orphaned
> projector cannot reduce a single dispatched byte, and nothing stopped the
> wiring from regressing back to a pass-through. This eval makes the three
> load-bearing guarantees permanent: the dispatched body is the role
> projection; cross-role and untagged content are excluded; malformed input
> fails closed *before* the bridge is launched and a no-match role is given an
> empty digest rather than a silent fallback to the full source.
>
> Source incident: cycle 1460 (scout finding "Pipeline wiring — HIGH").

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| live-materialization | Dispatched body is the role-scoped projection, proven through the real `BaseRunner.Run` path | 7/10 | `go test -tags acs -run TestC1460_001_...` |
| cross-role-isolation | No untagged prose, no other-role block leaks into the digest | 8/10 | `go test -tags acs -run TestC1460_002_...` |
| fail-closed-and-no-fallback | Unterminated marker aborts before `bridge.Launch`; no-match role gets an empty digest, never the full source | 6/10 | `go test -tags acs -run TestC1460_003_...` |
