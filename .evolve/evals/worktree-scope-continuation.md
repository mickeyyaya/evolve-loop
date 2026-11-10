---
score_cap:
  - criterion: "A present continuation manifest yields an ancestor true or false line measured against the project root HEAD"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./internal/core -run 'TestContinuationAncestor_SalvageOnProjectHeadReportsTrue|TestContinuationAncestor_SalvageOffProjectHeadReportsFalse'"
  - criterion: "An absent or unreadable manifest yields nothing and runs no git"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./internal/core -run 'TestContinuationAncestor_AbsentManifestYieldsNothingAndRunsNoGit|TestContinuationAncestor_UnreadableManifestYieldsNothing'"
  - criterion: "An unresolvable or malformed salvage sha reports unknown, never false, and a malformed sha never reaches git"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 ./internal/core -run 'TestContinuationAncestor_UnknownSHAReportsUnknownNotFalse|TestContinuationAncestor_ExitCodeMapsToVerdict|TestContinuationAncestor_MalformedSHAIsUnknownAndNeverReachesGit'"
  - criterion: "The probe asks the injected git runner exactly merge-base --is-ancestor sha HEAD"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./internal/core -run TestContinuationAncestor_ProbeAsksGitExactlyMergeBaseIsAncestorAgainstHead"
  - criterion: "Adoption serves the line to the Build request from the project root HEAD, and a fresh cycle serves none"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 ./internal/core -run 'TestContinuationAncestor_AdoptionServesTheLineFromTheProjectRootHead|TestContinuationAncestor_FreshCycleCarriesNoLine'"
  - criterion: "The Build prompt renders the served line and is byte-identical without it"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./internal/phases/build -run TestComposePrompt_ContinuationAncestor"
---

# Eval: continuation salvage ancestry probe (worktree-scope-continuation, Q5)

> Pins the probe that reports whether the salvage sha of a continuation is already an ancestor of the project root HEAD. Source: inbox prescription d3dc4fbcea83f461917acdbf494e9e1f0 (first seen cycle 1513), selected in cycle 2270. A false line tells the agent the salvage is still unshipped; an unknown line tells it git could not answer.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| true-false | Present manifest gives true or false against the project HEAD | 6/10 | `go test ./internal/core -run TestContinuationAncestor_Salvage` |
| absent | Absent or unreadable manifest gives nothing | 6/10 | `go test ./internal/core -run TestContinuationAncestor_AbsentManifest` |
| unknown | Unresolvable or malformed sha gives unknown, never false | 7/10 | `go test ./internal/core -run TestContinuationAncestor_UnknownSHA` |
| injected-runner | Exact argv through the injected runner | 6/10 | `go test ./internal/core -run TestContinuationAncestor_ProbeAsksGit` |
| caller-proof | Adoption reaches the Build request; fresh cycle does not | 8/10 | `go test ./internal/core -run TestContinuationAncestor_Adoption` |
| render | Build prompt shows the line, byte-identical without it | 6/10 | `go test ./internal/phases/build -run TestComposePrompt_ContinuationAncestor` |
