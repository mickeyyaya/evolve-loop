# Comment history: `acs/cycle809`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle809/predicates_test.go:3` — above `package cycle809`

```text
// Package cycle809 materializes the cycle-809 acceptance criteria for this fleet
// lane's sole committed defect, ciparity-integration-tier-race-parity (triage
// top_n). Per R9.3 no predicate binds to any deferred/dropped item
// (integration-tier-timeout-headroom-watch, integration-tier-coverprofile-parity
// are out of scope).
//
// Defect: integrationTierCheckDefault (go/internal/phases/audit/ciparity.go:205)
// runs `go test -count=1 -tags integration <pkgs>`, but the CI step it mirrors
// (.github/workflows/go.yml:59) runs `go test -race -count=1 -tags integration`.
// `-race` is present in CI, absent from the gate → a real data race in a touched
// package passes audit clean and goes CI-red on the very step this gate exists to
// pre-empt (warnship_apicover_ci_gap disease, one flag short of parity).
//
// Every predicate EXECUTES the system under test as a subprocess (`go test` of a
// named behavioral unit test) and requires an explicit `--- PASS: <name>` marker
// — exit 0 alone would also cover the "0 tests matched" case (a renamed/removed
// test), which must fail the predicate, not pass it. No source-grep predicate
// over logic files, and specifically no grep for the literal string "-race" (the
// cycle-85 degenerate-predicate failure mode, which a no-op would satisfy).
//
// AC map (1:1, from scout-report.md Acceptance Criteria Summary):
//
//	AC1 gate's `go test` invocation includes -race (proven by EFFECT: it now
//	    catches a real data race, invisible without -race)
//	      → C809_001 audit.TestIntegrationTierGate_Race
//	AC2 the regression test proves detection of a REAL race, not flag-string
//	    presence (the same fixture PASSES under plain -tags integration)
//	      → C809_002 audit.TestIntegrationTierGate_RaceFixtureIsRaceOnly
//	AC3 the pre-existing integration-tier gate tests remain GREEN (no drift)
//	      → C809_003 runs the three cycle-806 gate tests, each must PASS
//	AC4 go vet / -race / apicover -enforce clean on the touched package
//	      → manual+checklist (Auditor; the audit phase's own CI-parity gates run
//	        exactly these — see test-report.md)
//
// Adversarial axes: positive (AC1 gate catches the race under -race), negative
// (AC2 fixture passes WITHOUT -race, proving race-only detection), semantic (AC3
// existing offenders-FAIL / no-op / tag-membership behaviors are distinct, not
// one restated).
```

### `go/acs/cycle809/predicates_test.go:83` — above `func TestC809_003_existing_gate_tests_remain_green(t *testing.T) {`

```text
// AC3 (regression / no-drift) — the three cycle-806 integration-tier gate tests
// must stay GREEN after -race is added: offenders still FAIL audit, the default
// gate still no-ops without a go module, and NewDefault still wires a gate that
// truly runs `-tags integration`.
```
