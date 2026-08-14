# Cycle 1464 Dossier

**Goal:** Work through the todo inbox by weight; pipeline-repair items first.
**Final verdict:** FAIL
**Run ID:** 01M00GR3067PEF40SVE7W46TR5

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| cycle-recorded |  | FAIL |  | cycle completed; ledger walk deferred to future slice |

## Defects

- **audit-fail** (HIGH): cycle did not pass audit; see audit-report.md + acs-verdict.json — fix: address the audit findings recorded for this cycle


## Failure

**Fingerprint:** `audit|infra-error|149b8b250b24` · **Class:** infra-error

- EGPS: red_count=2 (cycle ships only when red_count==0)
- the integration tier (`go test -tags integration`) reported 7 offender(s) — CI's integration-tier test step would FAIL (e.g. TestFleetSoak). Offenders: --- FAIL: TestRun_RoleDigestExcludesForeignAnd


## Carryover

- **address-audit-findings** (high): resolve the audit findings that failed cycle 1464

