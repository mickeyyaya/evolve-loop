---
score_cap:
  - criterion: "The cross-artifact invariant aggregate evaluates all four classes and stays advisory"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 ./internal/coherence/..."
  - criterion: "The aggregate is recorded from the production cycle-close path, not only from tests"
    max_if_missing: 7
    evidence: "grep -q 'recordCrossArtifactInvariants(cycle, cs.WorkspacePath, cs.ActiveWorktree)' go/internal/core/cyclerun.go"
  - criterion: "The §6 experience record for item rank 5 exists per the inbox record's own DOCS-per-3.8 clause"
    max_if_missing: 6
    evidence: "grep -qE '^### 6\\.[0-9]+ .*[Cc]ross-artifact' docs/research/deliverable-alignment-2026-08/README.md"
  - criterion: "The portfolio and layer-model rows no longer describe the landed stack as new or partial"
    max_if_missing: 5
    evidence: "! grep -q 'cross-artifact invariant stack partial' docs/research/deliverable-alignment-2026-08/README.md"
---

# Eval: Cross-artifact metamorphic invariant stack (weak-verifier aggregation)

> Pins both halves of the `crossartifact-invariant-stack` acceptance contract.
> The CODE half — four weak deterministic verifiers (verdict agreement, test-count
> agreement, referenced-path existence, provenance/phase order) aggregated into one
> advisory report in `go/internal/coherence/crossartifact.go` and recorded from
> `go/internal/core/cyclerun.go:241` — landed and is green. The DOCS half, which the
> inbox record's own `fix` field names ("DOCS per 3.8 into
> docs/research/deliverable-alignment-2026-08/README.md"), never landed.
>
> Source incident: because the doc deliverable its acceptance contract requires was
> never written, the item's acceptance stayed unmet and `consumeCommittedItems` never
> retired it. The identical record sat unconsumed in `.evolve/inbox/processing/cycle-<N>/`
> for **every cycle from 1601 through 1679** — roughly twenty re-claims of work that was
> already built. This eval exists so the doc half can never again be the silent half.
>
> Advisory is deliberate, not provisional: per the record's own SWE-agent rule, each
> invariant ships advisory until its false-positive rate is evidenced ~0 (the 1054/1060
> breaker lesson). The "absence is indeterminate, never violated" property is what keeps
> that rate at zero, so it is pinned as a behavioural predicate rather than prose.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| aggregate-behaviour | All four invariant classes evaluate, and the report declares itself advisory | 8/10 | `go test -count=1 ./internal/coherence/...` |
| production-reachability | The aggregate is driven from the real cycle-close path, not only from a test | 7/10 | `grep recordCrossArtifactInvariants go/internal/core/cyclerun.go` |
| experience-record | §6.x entry for item rank 5 exists, per the record's DOCS-per-3.8 clause | 6/10 | `grep -E '^### 6\.[0-9]+ .*[Cc]ross-artifact' …/README.md` |
| staleness | Portfolio/L3 rows no longer call the landed stack "NEW" or "partial" | 5/10 | `! grep 'cross-artifact invariant stack partial' …/README.md` |

## Acceptance Criteria (code-graded)

Each check below RUNS the system and exits 0 only when the behaviour holds. The
`score_cap` half above caps a future audit score when evidence goes missing; this
half is what a grader executes. Every command is narrowed with `-run` to named
tests: `./internal/core` is a known 40s+ suite and is never swept whole here
(the cycle-1173/1175/1178 false-red shape).

### AC1: the aggregate evaluates all four invariant classes and declares itself advisory [code]
```bash
cd go && go test -count=1 -v -run '^TestCrossArtifactInvariants_(CoherentWorkspaceIsAllOK|ReportShapeIsDeterministic)$' ./internal/coherence | grep -c '^--- PASS: TestCrossArtifactInvariants_' | grep -qx 2
```
Expected: exit 0

### AC2: each invariant class is independently falsifiable, and names both sides of the disagreement [code]
```bash
cd go && go test -count=1 -v -run '^TestCrossArtifactInvariants_(VerdictDisagreementIsViolatedWithBothSides|TestCountDisagreementNamesClaimedAndCounted|MissingReferencedPathIsViolatedAndNamed|PhaseOrderViolationsAreReported)$' ./internal/coherence | grep -c '^--- PASS: TestCrossArtifactInvariants_' | grep -qx 4
```
Expected: exit 0

### AC3 (negative): absent or malformed artifacts are INDETERMINATE, never violated [code]
```bash
cd go && go test -count=1 -v -run '^TestCrossArtifactInvariants_(AbsentArtifactsAreIndeterminateNotOK|MalformedArtifactsAreIndeterminateNotOK|EscapingReferencedPathIsViolatedNotResolved)$' ./internal/coherence | grep -c '^--- PASS: TestCrossArtifactInvariants_' | grep -qx 3
```
Expected: exit 0 — this is the property that keeps the advisory's false-positive rate ~0 (the 1054/1060 breaker lesson). If absence ever reads as "violated", this check reds.

### AC4: the aggregate is reached from the real cycle-close path, and never blocks the cycle [code]
```bash
cd go && go test -count=1 -v -run '^TestFinalizeCycle_(EmitsAdvisoryCrossArtifactInvariantsArtifact|CrossArtifactViolationsNeverBlockTheCycle|CrossArtifactBindsTheLaneWorktreeNotTheProjectRoot|VerdictIncoherenceFloorSurvivesTheAdvisory)$' ./internal/core | grep -c '^--- PASS: TestFinalizeCycle_' | grep -qx 4
```
Expected: exit 0 — a seam whose only caller is a test is dead code; this drives the production `finalizeCycle` and asserts the ADR-0072 verdict-incoherence floor still halts beside the advisory.

### AC5: the experience record for item rank 5 landed and the stale rows are gone [code]
```bash
grep -qE '^### 6\.[0-9]+ .*[Cc]ross-artifact' docs/research/deliverable-alignment-2026-08/README.md && ! grep -q 'cross-artifact invariant stack partial' docs/research/deliverable-alignment-2026-08/README.md
```
Expected: exit 0 — the DOCS-per-3.8 half whose twenty-cycle absence is this eval's source incident.
