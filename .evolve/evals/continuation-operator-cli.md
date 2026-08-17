---
score_cap:
  - criterion: "`evolve continuation list` is registered and reports every registry binding with its scope id, snapshot SHA and ancestor cycle"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1515_004' ./acs/regression/cycle1515 | grep -q -- '--- PASS: TestC1515_004'"
  - criterion: "`evolve continuation list` on a project with no registry is a clean exit-0 report, not an error and not a phantom scope"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1515_005' ./acs/regression/cycle1515 | grep -q -- '--- PASS: TestC1515_005'"
  - criterion: "`evolve continuation release <scope-id>` releases exactly that binding and preserves the released value into the item file's released_continuations[]"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1515_006' ./acs/regression/cycle1515 | grep -q -- '--- PASS: TestC1515_006'"
  - criterion: "`evolve continuation release <unknown-scope>` fails loudly, names the scope, and leaves other bindings intact"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1515_007' ./acs/regression/cycle1515 | grep -q -- '--- PASS: TestC1515_007'"
  - criterion: "Malformed invocations (bare `continuation`, unknown subcommand, `release` with no scope) fail loudly and never mutate the registry"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1515_008' ./acs/regression/cycle1515 | grep -q -- '--- PASS: TestC1515_008'"
---

# Eval: `evolve continuation list` / `release <scope-id>` operator surface

> The scope-keyed continuation registry (`.evolve/continuation-registry.json`) is a
> first-class dispatch source — the wave planner mints lanes straight off it — but it
> had no operator surface at all. During the 2026-08-16 remediation console had to
> hand-edit the JSON under its flock sidecar twice (bindings for
> `context-fill-telemetry-and-cap`, `minted-phase-verdict-contract-unsatisfiable`,
> `dead-api-sweep`), preserving the salvage pointers by hand. This eval pins the
> command that replaces those hand edits: `list` must show what is bound, `release`
> must go through the same preserve-then-release path the lifecycle uses, and every
> malformed or unknown-scope invocation must fail loudly rather than reading as a
> successful release. Source incident: cycles 1487 and 1497 (burns #3 and #4 of
> `park-consume-releases-continuation-binding`), filed 2026-08-16.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| list-registered | Subcommand routes and reports scope/snapshot/cycle | 7/10 | `TestC1515_004` |
| list-empty-clean | Absent registry is the normal state, not an error | 5/10 | `TestC1515_005` |
| release-preserves | Release deletes the binding AND preserves the pointer | 8/10 | `TestC1515_006` |
| release-negative | Unknown scope fails loudly, no collateral release | 7/10 | `TestC1515_007` |
| malformed-input | Bad invocations never mutate the registry | 6/10 | `TestC1515_008` |
