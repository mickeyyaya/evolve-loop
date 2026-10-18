---
score_cap:
  - criterion: "The real build-planner ComposePrompt renders exactly one digest block equal to DigestFor(build-planner) with only scout and triage data, none for an inactive PhaseInput, and an inactive prompt byte-identical to BaseCycleContext"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1942_001_' ./acs/cycle1942 | grep -q -- '--- PASS: TestC1942_001_'"
  - criterion: "The real swarm-plan ComposePrompt renders exactly one digest block equal to DigestFor(swarm-plan) with only scout and triage data, none for an inactive PhaseInput, and an inactive prompt byte-identical to BaseCycleContext"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1942_002_' ./acs/cycle1942 | grep -q -- '--- PASS: TestC1942_002_'"
  - criterion: "The real debugger ComposePrompt renders exactly one digest block equal to DigestFor(debugger) with only triage and build data, none for an inactive PhaseInput, and keeps its Ship Failure Envelope"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1942_003_' ./acs/cycle1942 | grep -q -- '--- PASS: TestC1942_003_'"
  - criterion: "The intent edge stays declared empty and the intent prompt carries no digest block and no upstream data"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1942_004_' ./acs/cycle1942 | grep -q -- '--- PASS: TestC1942_004_'"
---

# Eval: per-edge handoff digest in the intent, build-planner, swarm-plan and debugger prompts

> Pins the cycle-1942 close of `tokenopt-handoff-digests-remaining-consumers`. Cycle 1938 declared
> every ComposePrompt edge in `phaseio.EdgeUpstreams` and wired triage and build through
> `runner.UpstreamDigestBlock`. This eval pins the four remaining consumers. Each block carries only
> the sections that its edge declares, and an inactive PhaseInput keeps the prompt unchanged.
> The intent edge is empty, so its block is always absent (cycle-1942 spec-verify finding 1).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| build-planner-consumer | one block, scout+triage only | 8/10 | `TestC1942_001_` |
| swarm-plan-consumer | one block, scout+triage only | 8/10 | `TestC1942_002_` |
| debugger-consumer | one block, triage+build only | 8/10 | `TestC1942_003_` |
| intent-empty-edge | no block, no upstream data | 6/10 | `TestC1942_004_` |
