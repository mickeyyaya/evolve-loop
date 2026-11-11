---
score_cap:
  - criterion: "failure-dossier.json names each postmortem file of the cycle, sorted"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestFailureDossierPostmortems_NamesEachPostmortemFileSorted$' ./internal/core"
  - criterion: "no postmortem omits the field; malformed names, directories and links are ignored"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^(TestFailureDossierPostmortems_NoPostmortemOmitsTheField|TestFailureDossierPostmortems_IgnoresMalformedNamesDirectoriesAndLinks)$' ./internal/core"
  - criterion: "the audit-fail and retro-floor decision callers write the postmortem paths to the artifact"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^(TestFailureDossierPostmortems_AuditFailDecisionWritesThePathsToTheArtifact|TestFailureDossierPostmortems_RetroFloorDecisionWritesThePathsToTheArtifact)$' ./internal/core"
---

# Eval: Attempt postmortem W2, the failure dossier records the postmortem paths

> Pins plan W2 of docs/plans/attempt-postmortem-2026-10.md. `failure-dossier.json`, written by `go/internal/core/failure_dossier.go`, lists the workspace-relative name of each `<phase>-attempt-<n>-postmortem.json` file. Source: cycle 2249, inbox item attempt-postmortem-w2-dossier-paths.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| positive-sorted | each postmortem file is named, in sorted order | 8/10 | `go test -run TestFailureDossierPostmortems_NamesEachPostmortemFileSorted ./internal/core` |
| omit-and-ignore | empty omits the field; bad entries are ignored | 7/10 | `go test -run TestFailureDossierPostmortems_NoPostmortemOmitsTheField ./internal/core` |
| production-callers | both decision callers write the paths | 8/10 | `go test -run TestFailureDossierPostmortems_AuditFailDecisionWritesThePathsToTheArtifact ./internal/core` |
