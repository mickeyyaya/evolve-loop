---
score_cap:
  - criterion: "parseArgs, artifactsFor, FakeExec.Run and runCycle are each ≤50 lines"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1770_001_FourTargetFunctionsFitTheRatchetLimit ./acs/cycle1770"
  - criterion: "go/internal/sizeratchet/offenders.json is untouched (allowances stay as unclaimed slack)"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1770_002_OffendersJSONLeftUnchanged ./acs/cycle1770"
  - criterion: "the module-wide size ratchet stays green after the shrink"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1770_003_ModuleWideRatchetCheckPasses ./acs/cycle1770"
  - criterion: "existing tests in cmd/evolve-fake-cli, test/fixtures and internal/routingtest pass unmodified (behavior preserved)"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 ./cmd/evolve-fake-cli ./test/fixtures ./internal/routingtest"
  - criterion: "no comment lines were added to the touched files"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC1770_005_NoCommentLinesAdded ./acs/cycle1770"
---

# Eval: Shrink evolve-fake-cli / fixtures / routingtest size-ratchet offenders

> Pins the cycle-1770 Extract Method refactor of `parseArgs` and `artifactsFor`
> in `go/cmd/evolve-fake-cli/main.go`, `FakeExec.Run` in
> `go/test/fixtures/cmd_runner.go`, and `runCycle` in
> `go/internal/routingtest/engine.go` — all four currently over the 50-line
> size-ratchet limit (95/79/56/95 lines) per
> `go/internal/sizeratchet/offenders.json`. Source: inbox item
> `sizeratchet-shrink-test-tooling` (2026-09-29T20-16-00Z), split by scout into
> tasks `shrink-fake-cli-parse-and-artifacts` and
> `shrink-fixtures-and-routingtest-offenders`, both committed to cycle 1770's
> `top_n`. The refactor must be behavior-preserving (existing test suites
> unmodified and green — all four offenders already have direct test coverage,
> so no new characterization tests are needed), must add no comments (naming
> carries intent per docs/conventions/code-comments.md), and must leave
> `offenders.json` untouched — an allowance is a ceiling only (since
> 2026-09-28), so a shrunk function's entry becomes unclaimed slack for a later
> boundary tighten rather than something this lane edits itself.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| ratchet-limit | All 4 target functions ≤50 lines | 8/10 | `go test -tags acs -run TestC1770_001_...` |
| offenders-untouched | offenders.json byte-identical to baseline, same 4 allowances | 7/10 | `go test -tags acs -run TestC1770_002_...` |
| module-ratchet-green | Whole-module sizeratchet.Check passes | 6/10 | `go test -tags acs -run TestC1770_003_...` |
| behavior-preserved | cmd/evolve-fake-cli, test/fixtures, internal/routingtest suites pass unmodified | 8/10 | `go test ./cmd/evolve-fake-cli ./test/fixtures ./internal/routingtest` |
| no-comments-added | commentaudit reports no added comment lines under the touched dirs | 5/10 | `go test -tags acs -run TestC1770_005_...` |
