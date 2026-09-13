---
score_cap:
  - criterion: "A landing-lost cycle's committed dossier carries the signal's structured category AND evidence text, driven through the REAL writeCycleDossier path — both production callers (cycleRun.completeCycle and cycleRun.abnormalEpilogue) over cycle-1535's real ship artifacts; JSON key system_failure with category + verbatim evidence, Markdown naming both"
    max_if_missing: 3
    evidence: "cd go && go test -count=1 -run '^(TestDossierSystemFailure_LostLandingReachesTheCommittedDossier|TestDossierSystemFailure_AbnormalEpilogueThreadsTheSignal)$' ./internal/core"
  - criterion: "The landed sibling with the same transient ship error (cycle-1536, ship-binding present) carries no landing-lost evidence, and an ordinary PASS dossier stays byte-clean against the pre-change golden (testdata/dossierparams/cycle-4243.golden.*)"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^(TestDossierSystemFailure_LandedSiblingCarriesNone|TestDossierSystemFailure_OrdinaryPassStaysByteClean)$' ./internal/core"
  - criterion: "acs/cycle1544 predicates 004-005 restored and green under the acs tag"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -v -run '^(TestC1544_004_LostLandingEvidenceReachesTheCommittedDossier|TestC1544_005_LandedSiblingAndOrdinaryPassCarryNoEvidence)$' ./acs/cycle1544 2>&1 | grep -c -- '--- PASS: TestC1544_00[45]_' | grep -qx 2"
  - criterion: "The dossier Go struct and schemas/cycle-dossier.schema.json stay in lock-step for the new field (bidirectional drift guard), and the FAIL-path golden is unchanged by a nil signal"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestSchema_NoDrift$' ./internal/dossier && go test -count=1 -run '^TestWriteCycleDossier_ParamsStructPreservesFixedInputBytes$' ./internal/core"
---

# Eval: Lost-landing evidence reaches the committed dossier

> Pins the inbox item `2026-08-23T12-01-00Z-lost-ship-dossier-evidence.json`
> (high, weight 0.82; re-filed after the cycle-1546 console salvage excised
> the draft together with the witness half that carried HIGH defects). The
> producer already exists: `finalizeCycle` stamps the landing-lost
> `SystemFailureSignal` onto `CycleResult` and downgrades the verdict to WARN
> (`go/internal/core/lost_landing_floor.go`, PR #482). What was missing is the
> SINK: `writeCycleDossier` received only the outcome string, so the committed
> `knowledge-base/cycles/cycle-N.{json,md}` carried a WARN indistinguishable
> from any other WARN — an operator could not see WHY a cycle was downgraded
> without diffing `ship-error.json` against `ship-binding.json` in gitignored
> runtime, per cycle, by hand (wave-20260822a-verify: cycle-1535 lost its
> landing to a peer's conflict and closed PASS; cycle-1536 hit the same
> GIT_FLEET_REBASE_NEEDED and landed). RED authored in cycle 1663 against the
> vendored 1535/1536 artifacts (`go/internal/core/testdata/lostlanding`).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| evidence-durability | landing-lost category + verbatim evidence in the committed pair, via BOTH real callers | 3/10 | `go test -run 'TestDossierSystemFailure_LostLanding…\|…AbnormalEpilogue…' ./internal/core` |
| false-alarm-immunity | landed sibling carries none; ordinary PASS byte-identical to the pre-change golden | 5/10 | `go test -run 'TestDossierSystemFailure_LandedSibling…\|…OrdinaryPass…' ./internal/core` |
| historical-suite-restored | cycle1544 004-005 print their own PASS lines under `-tags acs` | 6/10 | `go test -tags acs -run 'TestC1544_00[45]_' ./acs/cycle1544` |
| schema-lockstep | struct ⇄ schema drift guard green; FAIL golden unchanged | 6/10 | `go test -run TestSchema_NoDrift ./internal/dossier` + the 4242 golden |

The wire key is `system_failure` (generic — mirrors `CycleResult.SystemFailure`
and the signal's own JSON tags, so the second signal producer,
`detectVerdictIncoherence`, is not precluded later). Evidence is carried
verbatim: the floor formats one bounded line whose tail is the operator
instruction, so a `FailureRecord`-style byte cap would cut the actionable part.
