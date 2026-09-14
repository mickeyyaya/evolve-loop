---
score_cap:
  - criterion: "Ungated `evolve continuation release <scope-id>` refuses with guidance naming both authority paths, leaves the binding intact, and records nothing"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1684_001$' ./acs/cycle1684 | grep -q -- '--- PASS: TestC1684_001'"
  - criterion: "`-operator` authorizes the release: the binding is removed and a durable record answers who/when/why"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1684_002$' ./acs/cycle1684 | grep -q -- '--- PASS: TestC1684_002'"
  - criterion: "EVOLVE_OPERATOR_CONFIRM=1 authorizes the release and a set-but-negative value (0) does NOT — the env gate reads the value, not mere presence"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1684_003$' ./acs/cycle1684 | grep -q -- '--- PASS: TestC1684_003'"
  - criterion: "A scope whose cycle holds a LIVE lease refuses even for an authorized operator unless -force is passed"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1684_004$' ./acs/cycle1684 | grep -q -- '--- PASS: TestC1684_004'"
  - criterion: "-force overrides a live lease and the record says so; a lease staler than runlease.DefaultTTL does not block at all"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1684_005$' ./acs/cycle1684 | grep -q -- '--- PASS: TestC1684_005'"
  - criterion: "The authority check lives in continuation.RequireOperatorAuthority, refuses unauthorized in-process callers, and is the helper the CLI itself calls"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1684_006$' ./acs/cycle1684 | grep -q -- '--- PASS: TestC1684_006'"
  - criterion: "The cycle's required explanation document's `## Limitations` describes the tree it is SHA-bound to: it does not claim cycle 1515's predicate was left unadjudicated when this same diff re-authored it"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1684_008' ./acs/cycle1684 | grep -q -- '--- PASS: TestC1684_008_ExplanationLimitationsMatchTheBoundTree'"
---

# Eval: `evolve continuation release` operator-authority gate

> `evolve continuation release <scope-id>` shipped in cycle 1515 as console's
> replacement for hand-editing `.evolve/continuation-registry.json` under its flock
> sidecar — and it shipped with no authority gate whatsoever: no `-operator` flag,
> no `EVOLVE_OPERATOR_CONFIRM` env fallback, no live-lease check, and a hardcoded
> `"operator-release"` reason string that names no actual caller. Its sibling
> sensitive surface `evolve reset-sha` has had `--operator` since ADR-0065. The
> phase guard (`go/internal/guards/phase.go`) denies in-process `Agent`/`Task`
> dispatch during a cycle but says nothing about a Bash invocation of the `evolve`
> binary, so any Bash-capable process — an in-cycle agent included — could drop a
> LIVE scope's continuation binding. That binding is the lineage the defect-ledger
> gate treats as anti-tamper evidence (ADR-0085/0089, cycle-1285), which makes
> silent erasure of it the whole problem: the registry's "ORCHESTRATOR-side only"
> authority invariant was widened by omission, not by decision.
>
> This eval pins the gate that closes it. Authority must be explicit (flag or an
> affirmative env value, never mere presence); a lane that is still ALIVE must not
> have its binding dropped out from under it without `-force`; a lane that is DEAD
> must not have its scope bricked forever by a stale `.lease`; and every release
> must leave a durable record answering who authorized it, when, and why — so that
> a lineage erasure is itself evidenced. The check lives in a shared helper
> precisely so a future in-process caller cannot route around it. Source incident:
> the 3-agent alignment audit's ADR-0089 gap #2, filed 2026-08-18, fixed in cycle
> 1684.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| ungated-refuses | No authority ⇒ non-zero, binding intact, nothing recorded | 8/10 | `TestC1684_001` |
| flag-authorizes | `-operator` releases AND records who/when/why | 7/10 | `TestC1684_002` |
| env-value-not-presence | `=1` authorizes, `=0` does not | 6/10 | `TestC1684_003` |
| live-lease-refuses | Authorized is not enough while the lane is alive | 8/10 | `TestC1684_004` |
| force-and-staleness | `-force` overrides and is recorded; a stale lease never blocks | 7/10 | `TestC1684_005` |
| shared-helper | In-process callers cannot bypass the CLI's gate | 7/10 | `TestC1684_006` |
| explanation-accuracy | The bound document's `## Limitations` matches the tree, not a superseded draft of it | 6/10 | `TestC1684_008` |
