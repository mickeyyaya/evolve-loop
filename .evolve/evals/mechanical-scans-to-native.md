---
score_cap:
  - criterion: "secret-leak-scan dispatches as a native in-process Go phase and emits the LLM-contract artifact (clean diff → PASS, zero tokens)"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC785_001|TestC785_002' ./acs/cycle785"
  - criterion: "flake-rerun-scan natively re-runs new/failed tests and diffs outcomes deterministically (stable → identical PASS reports; stateful flake → named, non-PASS)"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC785_003|TestC785_004' ./acs/cycle785"
  - criterion: "phase spec selects impl per phase (kind: native|llm) with the LLM path still valid and no env-flag escape hatch"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC785_005|TestC785_006' ./acs/cycle785"
  - criterion: "native scan packages pass their unit contract under the race detector"
    max_if_missing: 6
    evidence: "cd go && go test -race -count=1 ./internal/phases/secretleakscan/... ./internal/phases/flakererunscan/..."
---

# Eval: Reimplement secret-leak-scan and flake-rerun-scan as native Go phases

> Pins the cycle-785 conversion of the two mechanical scan phases from
> full LLM boots (tmux CLI, 12 turns, ~5.5K output tokens measured for
> flake-rerun-scan on balanced tier) to native in-process Go, following the
> ship kind:"native" precedent. The load-bearing compatibility surface is the
> deliverable contract: the native impls must emit the same report path,
> classify sections, and canonical verdict vocabulary the LLM versions
> produced, because downstream gates pattern-match those artifacts. Source
> incident: tokenopt-2026-07 campaign, operator-boosted inbox item
> mechanical-scans-to-native (weight 0.95); Rule 5 (deterministic work
> belongs in code, not LLM cycles).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| native-dispatch-contract | secret-leak-scan native, contract artifact, planted-secret rejection | 7/10 | `go test -tags acs -run 'TestC785_001\|TestC785_002' ./acs/cycle785` |
| rerun-diff-determinism | flake-rerun-scan reruns + deterministic diff, stateful-flake detection | 7/10 | `go test -tags acs -run 'TestC785_003\|TestC785_004' ./acs/cycle785` |
| per-phase-impl-selection | kind native\|llm per phase, no env-flag escape hatch | 6/10 | `go test -tags acs -run 'TestC785_005\|TestC785_006' ./acs/cycle785` |
| race-clean | new packages green under -race | 6/10 | `go test -race ./internal/phases/secretleakscan/... ./internal/phases/flakererunscan/...` |
