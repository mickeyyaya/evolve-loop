---
score_cap:
  - criterion: "Every ComposePrompt phase edge (scout, triage, tdd, build, audit, intent, build-planner, swarm-plan, debugger) is declared in one phaseio edge config; no edge names itself or audit; an unknown phase is undeclared"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1938_001_' ./acs/cycle1938 | grep -q -- '--- PASS: TestC1938_001_'"
  - criterion: "Handoffs.DigestFor renders only the declared upstream sections (typed and generic); an empty or undeclared edge renders nothing"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1938_002_' ./acs/cycle1938 | grep -q -- '--- PASS: TestC1938_002_'"
  - criterion: "DigestFor stays within the package-default 1024-rune cap and an oversized scalar never evicts another declared section"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1938_003_' ./acs/cycle1938 | grep -q -- '--- PASS: TestC1938_003_'"
  - criterion: "The real triage ComposePrompt renders exactly one digest block equal to DigestFor(triage), with no undeclared data, and none for an inactive PhaseInput"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1938_004_' ./acs/cycle1938 | grep -q -- '--- PASS: TestC1938_004_'"
  - criterion: "The real build ComposePrompt renders exactly one digest block equal to DigestFor(build), with no undeclared data, and none for an inactive PhaseInput"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1938_005_' ./acs/cycle1938 | grep -q -- '--- PASS: TestC1938_005_'"
  - criterion: "The audit prompt carries no upstream digest and no builder self-report"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1938_006_' ./acs/cycle1938 | grep -q -- '--- PASS: TestC1938_006_'"
  - criterion: "The unmet remainder of this item is a tracked, claimable, weighted inbox-root item linked by connects_to/deps"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1938_008_' ./acs/cycle1938 | grep -q -- '--- PASS: TestC1938_008_'"
---

# Eval: per-edge handoff digest config and its first consumers

> Pins the cycle-1938 slice of `tokenopt-handoff-digests-per-edge-remainder`. Cycle 1632 landed the
> bounded `UpstreamDigest` and one consumer (tdd). This slice makes the artifact flow of each
> ComposePrompt edge explicit config (`phaseio.EdgeUpstreams`), renders a per-edge bounded digest
> (`Handoffs.DigestFor`), and routes it into the triage and build prompts. The audit edge is
> declared empty so the auditor never reads the builder's self-report. Edges that are declared but
> not yet consumed (intent, build-planner, swarm-plan, debugger) go to a queued remainder item.
> Source incident: cycle 1632 audit H1 (a partial landing retired the parent item with no remainder).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| edge-config | every ComposePrompt edge declared | 8/10 | `TestC1938_001_` |
| section-filter | only declared sections render | 8/10 | `TestC1938_002_` |
| bounded | package-default cap, no eviction | 7/10 | `TestC1938_003_` |
| triage-consumer | one block, equal to DigestFor | 8/10 | `TestC1938_004_` |
| build-consumer | one block, equal to DigestFor | 8/10 | `TestC1938_005_` |
| audit-isolation | no digest into audit | 7/10 | `TestC1938_006_` |
| remainder | queued tracked remainder item | 8/10 | `TestC1938_008_` |
