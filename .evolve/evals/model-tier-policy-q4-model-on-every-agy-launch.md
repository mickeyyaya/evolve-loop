---
score_cap:
  - criterion: "A dispatch launch of agy with no tier carries the fast model of the agy manifest"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2139_00[12]_' ./acs/cycle2139"
  - criterion: "A launch whose tier token does not resolve carries the fast model, read from the manifest map"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2139_00[34]_' ./acs/cycle2139"
  - criterion: "Resolved tiers, raw models and the other CLIs keep their argv"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2139_00[56]_' ./acs/cycle2139"
  - criterion: "The agy -p doctor probe carries the fast model"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2139_007_' ./acs/cycle2139"
  - criterion: "No non-test Go literal names an agy model, and the old no-model agy tests pin the fast model"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2139_00[89]_' ./acs/cycle2139"
  - criterion: "An empty tier stays model-less, so the CLI-health canary launches agy on its own default"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2139_01[01]_' ./acs/cycle2139"
  - criterion: "The unresolved fallback is manifest data, not agy binary-name sniffing in the realizer"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2139_01[23]_' ./acs/cycle2139"
---

# Eval: Every agy launch passes a valid --model (tier policy Q4)

> Pins component Q4 (decision D10) of docs/plans/model-tier-policy-2026-10.md. A launch of agy with no tier, or with a tier token that does not resolve, must carry the fast model of the agy manifest. Without it, agy uses its saved default and spends the Argon pool (plan fact U2). The real no-tier path reaches the bridge as the token `auto`, so `auto` is an unresolved token for agy. The model comes from model_tier_map, never from a Go literal. The empty tier is not a dispatch: it is the CLI-health canary (go/cmd/evolve/cli_health_canary.go), which must launch a benched CLI on its own default. The fallback is declared in the model_tier ParamSpec of the manifest. Source incident: inbox item model-tier-policy-q4-model-on-every-agy-launch, cycle 2139; audit round 4 findings H1 (canary RED at ship REPO_CONTRACT_GATE) and M1 (binary-name sniffing).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| no-tier | untiered agy launch carries fast, through the production launch line | 4/10 | `go test -tags acs -run TestC2139_00[12]_ ./acs/cycle2139` |
| unresolved | unresolved token carries fast from the manifest map | 5/10 | `go test -tags acs -run TestC2139_00[34]_ ./acs/cycle2139` |
| preservation | resolved tiers, raw models, other CLIs unchanged | 6/10 | `go test -tags acs -run TestC2139_00[56]_ ./acs/cycle2139` |
| probe | agy -p probe carries fast | 6/10 | `go test -tags acs -run TestC2139_007_ ./acs/cycle2139` |
| no-literal | no agy model literal in non-test Go | 5/10 | `go test -tags acs -run TestC2139_00[89]_ ./acs/cycle2139` |
| canary | empty tier launches agy with no --model; the cmd/evolve canary contract is GREEN | 3/10 | `go test -tags acs -run TestC2139_01[01]_ ./acs/cycle2139` |
| manifest-data | fallback comes from ParamSpec data, not from the agy binary name | 5/10 | `go test -tags acs -run TestC2139_01[23]_ ./acs/cycle2139` |
