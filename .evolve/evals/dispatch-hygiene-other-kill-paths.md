---
score_cap:
  - criterion: "the recipe-adapter probe (CaptureHelp) exports a parseable dispatch tag and EVOLVE_PROJECT_ROOT before the CLI launches, and persists its pane tree before kill-session"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 ./internal/bridge -run '^TestProbeTag_CaptureHelpTagsItsPaneAndPersistsTheTreeBeforeKillSession$'"
  - criterion: "the capture-models probe (CaptureModelPicker) exports a parseable dispatch tag and EVOLVE_PROJECT_ROOT before the CLI launches, and persists its pane tree before kill-session"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 ./internal/bridge -run '^TestProbeTag_CaptureModelPickerTagsItsPaneAndPersistsTheTreeBeforeKillSession$'"
  - criterion: "gc plans and stops the leaked pane child of each probe path through an injected signal sender"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./internal/bridge -run '^TestProbeGC_ALeaked(Help|ModelPicker|LaunchModelMismatch)Probe'"
  - criterion: "gc spares a live dispatch, another project and untagged processes"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 ./internal/bridge -run '^TestProbeGC_ALeakedProbeTagIsNoWarrant'"
---

# Eval: Tag and reap the probe sessions of capture_models, launch_model_verification and recipe_adapter

> Pins the dispatch-hygiene contract for the probe launch paths of plan docs/plans/dispatch-process-hygiene-2026-10.md.
> A probe session that starts without EVOLVE_DISPATCH_ID, EVOLVE_PROJECT_ROOT and a saved pane tree cannot be matched by gc once its owner dies.
> Source incident: cycle 2218, inbox item dispatch-hygiene-other-kill-paths.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| recipe-tag | CaptureHelp tags its pane and saves the tree | 5/10 | `go test -run TestProbeTag_CaptureHelp...` |
| picker-tag | CaptureModelPicker tags its pane and saves the tree | 5/10 | `go test -run TestProbeTag_CaptureModelPicker...` |
| gc-reap | gc stops the leaked probe child with an injected sender | 6/10 | `go test -run TestProbeGC_ALeaked...Probe` |
| gc-spare | gc spares live, foreign and untagged processes | 7/10 | `go test -run TestProbeGC_ALeakedProbeTagIsNoWarrant` |
