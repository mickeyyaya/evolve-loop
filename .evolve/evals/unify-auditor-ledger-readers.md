---
score_cap:
  - criterion: "internal/auditledger.LatestAuditorEntry binds this run's newest kind=agent_subprocess/role=auditor row (latest-any when runID is empty), skips alien lines, and parses rows as JSON"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1698_001_' ./acs/cycle1698"
  - criterion: "Every miss (foreign-run-only, unstamped, wrong kind, no auditor row, empty ledger) is errors.Is(ErrNoAuditorForRun), never fs.ErrNotExist, and the foreign refusal names the refused run id and git_head"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1698_002_' ./acs/cycle1698"
  - criterion: "An absent ledger stays errors.Is(fs.ErrNotExist) and an unreadable one is neither a miss nor an absence; releasepreflight.Run with no ledger stays the advisory NONE"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1698_003_' ./acs/cycle1698"
  - criterion: "internal/auditledger is a leaf: it depends on no consumer, no internal/core, no cmd package"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1698_004_' ./acs/cycle1698"
  - criterion: "ship, cmd/evolve and releasepreflight all import the helper and none re-declares the auditor row (a role+kind json struct or a raw-line field regex)"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1698_00[56]_' ./acs/cycle1698"
  - criterion: "releasepreflight.Run binds exactly the auditor row the helper binds, for a whitespace-formatted row and next to an auditor-role row of another kind"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1698_007_' ./acs/cycle1698"
  - criterion: "redteamcheck is untouched and does not depend on the run-scoped helper"
    max_if_missing: 7
    evidence: "cd go && test -n \"$(go list -deps ./internal/redteamcheck)\" && test -z \"$(go list -deps ./internal/redteamcheck | grep 'internal/auditledger$')\""
  - criterion: "The stale TODO(merge-concurrency-2026) no longer appears in the Go tree outside go/acs"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1698_009_' ./acs/cycle1698"
  - criterion: "internal/auditledger is enrolled in go/.apicover-enforce with every export documented, named by a package test and executed"
    max_if_missing: 6
    evidence: "cd go && APICOVER_PKGS=./internal/auditledger make apicover-enforce"
  - criterion: "The helper's and the three consumers' own package suites stay green"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -tags integration ./internal/auditledger ./internal/releasepreflight && go test -count=1 -tags integration -run Audit ./internal/phases/ship"
---

# Eval: Unify the auditor-ledger readers behind one helper with a typed sentinel

> Three consumers read the same auditor ledger row on their own:
> `ship.findLatestAudit` (internal/phases/ship/audit.go), `latestAuditEntry`
> (cmd/evolve/cmd_composition_wiring.go) and `releasepreflight.checkRecentAudit`
> (a raw-line regex walk). Each re-declares the row and its own scoping policy,
> and cycle-1571's H3 fail-open hole had to be closed in two of them separately.
> Cycle 1698 folds them onto `internal/auditledger`: one backward scan, one row
> identity (kind=agent_subprocess, role=auditor), one run-scope predicate and a
> typed `ErrNoAuditorForRun` sentinel carrying the refused foreign entry, which
> each consumer maps onto its own error vocabulary. `redteamcheck` stays out
> (cycle-scoped over all roles, no run scoping). Source: inbox item
> 2026-08-27T06-00-00Z-unify-auditor-ledger-readers, console premise audit
> 2026-09-26, cycle 1698.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| run-scoped binding | helper binds this run's newest auditor row | 6/10 | `TestC1698_001_` |
| typed miss sentinel | every miss is `ErrNoAuditorForRun`, foreign refusal names the entry | 7/10 | `TestC1698_002_` |
| absent-ledger edge | absent/unreadable ledger stays distinguishable; preflight stays advisory | 6/10 | `TestC1698_003_` |
| leaf package | no dependency on a consumer or core | 5/10 | `TestC1698_004_` |
| single row schema | consumers import the helper, re-declare no row | 5/10 | `TestC1698_005_`, `TestC1698_006_` |
| production-caller agreement | releasepreflight.Run binds the helper's row | 6/10 | `TestC1698_007_` |
| scope fence | redteamcheck untouched | 7/10 | `go list -deps ./internal/redteamcheck` |
| stale TODO | marker gone from go/ | 4/10 | `TestC1698_009_` |
| apicover graduation | enrolled, named, covered | 6/10 | `make apicover-enforce` |
| no regression | touched package suites green | 8/10 | `go test` per package |
