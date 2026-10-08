---
score_cap:
  - criterion: "No generated commands/<name>.md tells the reader to load evo:<name>, the id it is itself registered under"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1840_001_GeneratedCommandNeverLoadsItsOwnID ./acs/cycle1840"
  - criterion: "A generated wrapper names ${CLAUDE_PLUGIN_ROOT}/skills/<name>/SKILL.md and that path resolves inside the plugin"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1840_002_GeneratedCommandNamesPluginSkillPath ./acs/cycle1840"
  - criterion: "skillcheck.Check reports zero drift and the committed commands/*.md carry no self-reference"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1840_003_RepoCommandsHaveZeroDriftAndNoSelfReference ./acs/cycle1840"
  - criterion: "A legacy self-referencing stub is reported as drift"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1840_004_LegacySelfReferencingStubIsDrift ./acs/cycle1840"
  - criterion: "skillcheck unit tests pin the plugin-path stub and reject the self id"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestCommandDiffs_ProjectsStubPerSkill|TestRun_GenerateWritesCommandStubs' ./internal/skillcheck"
---

# Eval: Generated skill command must not shadow its skill

> `RenderCommandStub` emitted `Run the **<name>** skill ... (Skill tool id evo:<name>)`. The command
> wrapper and the skill share the id `evo:<name>`, so the Skill tool resolved the id back to the
> wrapper: a circular load that three console lanes reported on 2026-10-06. Cycle 1840 pins the fix:
> the wrapper reads `${CLAUDE_PLUGIN_ROOT}/skills/<name>/SKILL.md` directly, and the regenerated
> `commands/*.md` show zero drift.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| no-self-id | generate never emits evo:<name> | 8/10 | `TestC1840_001` |
| plugin-path | wrapper names a resolving SKILL.md path | 8/10 | `TestC1840_002` |
| zero-drift | repo commands regenerated | 7/10 | `TestC1840_003` |
| legacy-is-drift | old stub flagged | 6/10 | `TestC1840_004` |
| unit-pins | package tests updated | 6/10 | `go test ./internal/skillcheck` |
