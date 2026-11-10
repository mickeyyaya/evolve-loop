---
score_cap:
  - criterion: "dedupeLaunchFlags keeps a value shared by two multi-value flag runs"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./internal/bridge/ -run 'TestDedupeLaunchFlags_GroupsFlagWithWholeValueRun|TestDedupeLaunchFlags_TwoMultiValueFlagsKeepSharedValues'"
  - criterion: "headless codex realizes its reasoning effort from the codex-tmux family table"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 ./internal/bridge/ -run 'TestCodexEffort_HeadlessRealizationDrawsFromFamilyTable|TestCodexEffort_HeadlessManifestHoldsNoSecondEffortTable'"
  - criterion: "the headless codex argv carries the resolved launch effort and no fixed literal"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 ./internal/bridge/ -run 'TestCodexEffort_LaunchIntentEffortReachesHeadlessArgv'"
  - criterion: "argsContainEffort matches the model_reasoning_effort key only at a token start"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 ./internal/bridge/ -run 'TestArgsContainEffort'"
  - criterion: "the capability matrix has a row naming both codex effort keys"
    max_if_missing: 4
    evidence: "grep -n 'effort' docs/architecture/cli-capability-matrix.md | grep model_reasoning_effort | grep -q plan_mode_reasoning_effort"
---

# Eval: codex effort single source and dedupe grouping

> Pins three coupled fixes of cycle 2261. Headless codex took its effort from a literal `high` in the driver while the codex-tmux manifest held the real table. `dedupeLaunchFlags` paired a flag with one following token, so the tail of `--allowedTools Read Write Bash` was dropped from a later `--disallowedTools Bash Write`. The capability matrix named no effort row.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| flag-run grouping | a value shared by two multi-value flag runs survives | 6/10 | `go test -run DedupeLaunchFlags` |
| family table | headless effort comes from codex-tmux | 7/10 | `go test -run CodexEffort_HeadlessRealization` |
| no literal | argv carries the resolved effort | 7/10 | `go test -run CodexEffort_LaunchIntent` |
| exact key | argsContainEffort ignores look-alike keys | 5/10 | `go test -run ArgsContainEffort` |
| docs row | matrix names both effort keys | 4/10 | `grep` on the matrix |
