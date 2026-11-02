---
score_cap:
  - criterion: "The agy deep probe argv passes the fast model of the agy manifest after --model"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2106_00[12]_' ./acs/cycle2106"
  - criterion: "Bare ollama mints to the ollama-tmux driver that bridge.Resolve returns"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2106_005_' ./acs/cycle2106"
  - criterion: "The doctor argv guard passes on the tree and fails on a fixture with an LLM argv literal"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^(TestDoctorSource_HasNoLLMArgvLiteral|TestDoctorArgvGuard_FlagsAFixtureWithALiteral)$' ./internal/bridge"
  - criterion: "Headless families still deep-probe, tmux drivers still skip, and a failing probe still blocks"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2106_00[346]_' ./acs/cycle2106"
  - criterion: "A manifest that fails to load blocks the deep probe and the report carries the load error"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2106_008_' ./acs/cycle2106"
---

# Eval: The deep probe runs through the driver of its family (bridge-factory plan F2)

> Pins the probe part of component F2 of docs/plans/bridge-factory-2026-10.md. The agy probe in
> `go/internal/bridge/doctor.go` built its own argv, so it ignored the manifest model. Cycle 2106
> delivers acceptance 1, 3 and 4. Acceptance 2 (remove DriverFor and bareDriverMap) is deferred by
> triage because it needs a console change to a frozen predicate (HEALING_I10).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| agy-fast-model | agy probe argv carries the manifest fast model | 6/10 | `go test -tags acs -run 'TestC2106_00[12]_' ./acs/cycle2106` |
| ollama-pin | bare ollama mints to ollama-tmux | 7/10 | `go test -tags acs -run 'TestC2106_005_' ./acs/cycle2106` |
| doctor-guard | guard passes on tree, fails on fixture | 7/10 | `go test -run TestDoctorSource_HasNoLLMArgvLiteral ./internal/bridge` |
| probe-preservation | headless probe, tmux skip and blocked verdict unchanged | 5/10 | `go test -tags acs -run 'TestC2106_00[346]_' ./acs/cycle2106` |
| manifest-load-failure | a failed manifest load blocks the probe and is reported (audit M1) | 6/10 | `go test -tags acs -run TestC2106_008_ ./acs/cycle2106` |
