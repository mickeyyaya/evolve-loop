---
score_cap:
  - criterion: "tokenusage attribution is anchored to the assembler's literal 'Artifact path: ' marker, proven by a named regression test"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -v -run TestAttributes_MarkerAnchored ./internal/tokenusage | grep -q '^--- PASS: TestAttributes_MarkerAnchored'"
  - criterion: "the S1 concurrent-sessions content-verification contract still holds against the marker-form fixture"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -v -run TestTranscriptScan_ConcurrentSessionsSameDir_OnlyContentVerifiedCounted ./internal/tokenusage | grep -q '^--- PASS: TestTranscriptScan_ConcurrentSessionsSameDir_OnlyContentVerifiedCounted'"
  - criterion: "a prose-only mention of another launch's ArtifactPath does not attribute that transcript's tokens to the cited launch"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1457_00(2|3)' ./acs/cycle1457"
---

# Eval: token-telemetry attribution anchored to the artifact-path marker

> Pins the attribution key of `tokenusage.ScanConfigRoot`. Before cycle-1457,
> `attributes()` (`go/internal/tokenusage/scanner.go:209`) attributed a transcript
> to a launch whenever the launch's `ArtifactPath` appeared *anywhere* in the
> transcript's first user message — a bare `strings.Contains`. Every production
> launch carries the path, but so does any prompt that merely cites it in prose:
> `.evolve/profiles/retrospective.json:118` instructs a retrospective launch to
> "Read `.evolve/runs/cycle-{cycle}/build-report.md` and `audit-report.md`", which
> under the bare-substring rule bills the entire retrospective launch to the
> BUILDER's Window. Both assemblers stamp a literal label —
> `go/internal/subagent/subagent.go:358` (`"Artifact path: %s\n"`) and
> `go/internal/subagent/run.go:442` (`"- Artifact path: %s\n"`) — so anchoring the
> match to `"Artifact path: " + ArtifactPath` closes the vector without narrowing
> any genuine launch out.
>
> This eval outlives the cycle: it caps the audit score of any future cycle that
> reverts the anchor, drops the regression test, or lets the assembler's label
> drift away from the scanner's anchor. Source incident: the token-telemetry
> attribution follow-up filed against cycle-1455's landing
> (`fix/token-telemetry-artifactpath-attribution`), materialised in cycle 1457.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| marker-anchored-match | `attributes()` keys on the literal marker, not a bare path substring, with a named regression test proving it | 7/10 | `go test -v -run TestAttributes_MarkerAnchored ./internal/tokenusage` |
| s1-contract-preserved | The concurrent-sessions content-verification contract survives the marker-form fixture re-cut | 6/10 | `go test -v -run TestTranscriptScan_ConcurrentSessionsSameDir_OnlyContentVerifiedCounted ./internal/tokenusage` |
| no-prose-over-attribution | A prose mention (and any near-miss label) of a foreign `ArtifactPath` yields `SourceNone`, not a stolen token bill | 8/10 | `go test -tags acs -run 'TestC1457_00(2\|3)' ./acs/cycle1457` |
