---
score_cap:
  - criterion: "A reviewed drop decision retires the consumed fleet alias without removing an unrelated live carryover item"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1498_001_AliasDropRemovesOnlyTheNamedAlias$' ./acs/cycle1498"
  - criterion: "An unjustified (empty-reason) drop is rejected non-zero and leaves state.json byte-identical"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1498_002_EmptyReasonDropIsRejectedBeforeAnyWrite$' ./acs/cycle1498"
  - criterion: "Re-applying the same retirement decision is idempotent and leaves valid JSON"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1498_003_RepeatedAliasDropIsIdempotent$' ./acs/cycle1498"
  - criterion: "The named regression pin TestCarryoverApplyDecisions_DropsConsumedFleetAlias lives in go/cmd/evolve and passes in normal CI"
    max_if_missing: 5
    evidence: "cd go && go test ./cmd/evolve -run '^TestCarryoverApplyDecisions_DropsConsumedFleetAlias$' -v -count=1 | grep -q -- '--- PASS: TestCarryoverApplyDecisions_DropsConsumedFleetAlias'"
---

# Eval: Retire the consumed pipeline-blocker fleet alias safely

> Pins the retirement contract for the consumed fleet alias
> `pipeline-defect-pipeline-blocker`. The alias is a refuted premise that has been
> resident in `carryoverTodos` for 14 unpicked cycles and is projected into
> `router.RouteInput.CarryoverTodos` (`go/internal/router/router.go:69-74`), where
> it adds stale context to every DynamicLLM planner prompt. The safe retirement
> route is the EXISTING reviewed/locked path — `evolve carryover apply-decisions`
> (`go/cmd/evolve/cmd_carryover.go`) — never a prompt-side suppression rule and
> never a hand edit of `state.json`. Two properties of that path are what make the
> retirement auditable and must never regress: (1) removal is keyed on the exact
> `id`, so an unrelated live item whose *text* mentions the alias survives, and
> (2) an unjustified decision row is refused in memory before the lock is taken,
> so a rejected apply leaves `state.json` byte-identical. Source incident:
> cycle 1498 (`.evolve/state.json:3061`, todo
> `todo-consumed-alias-lifecycle-cleanup`).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| named-id-only removal | Alias retired, unrelated live sibling preserved | 7/10 | `go test -tags acs -run TestC1498_001... ./acs/cycle1498` |
| unjustified-drop refused | Empty-reason drop exits non-zero, state byte-identical | 8/10 | `go test -tags acs -run TestC1498_002... ./acs/cycle1498` |
| idempotent re-apply | Repeat apply exits 0, JSON stays valid, no collateral removal | 6/10 | `go test -tags acs -run TestC1498_003... ./acs/cycle1498` |
| durable regression pin | Named test resides with the command and runs in normal CI | 5/10 | `go test ./cmd/evolve -run ...DropsConsumedFleetAlias -v` |
