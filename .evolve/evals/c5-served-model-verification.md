---
score_cap:
  - criterion: "A launch whose served model differs from the catalog pick emits the dispatch_model_mismatch signal, and the picked model emits nothing"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 ./internal/bridge -run '^(TestLaunchModelVerification_AgyClaudeBootingAnotherClaudeModelFailsOverWithTheMismatchSignal|TestLaunchModelVerification_ThePickedModelAtAnotherEffortStillVerifies|TestServedModel_AClaudeTmuxTranscriptModelOfAnotherTierEmitsTheMismatchSignal|TestServedModel_AClaudeTmuxTranscriptModelOfThePickedTierEmitsNothing)$'"
  - criterion: "claude-tmux reads message.model from the transcript and a transcript without it reports no served model"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./internal/tokenusage -run '^(TestScanConfigRoot_ServedModelIsTheLatestRealAssistantModelInTheWindow|TestScanConfigRoot_ATranscriptWithNoModelFieldReportsNoServedModel)$'"
  - criterion: "agy-tmux declares the Gemini rule and its driver runs it before the prompt"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./internal/bridge -run '^(TestLaunchModelCheckFor_AgyTmuxDeclaresTheGeminiRule|TestLaunchModelVerification_AgyTmuxBootingClaudeFailsOverWithTheMismatchSignal|TestEveryManifestDeclaringALaunchModelRuleIsDrivenByAVerifier)$'"
---

# Eval: Model currency C5 remainder, served-model verification

> Pins criteria 1 to 3 of inbox item model-currency-c5-launch-model-verification-remainder: the launch compares the served model, not the family, with the catalog pick. Source incident: cycles 2145 and 2195 landed the code, and the 2195 push lost to a diverged origin/main.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| mismatch-signal | served model differs from pick emits the signal | 5/10 | `go test ./internal/bridge -run LaunchModelVerification` |
| transcript-model | claude-tmux reads message.model | 6/10 | `go test ./internal/tokenusage -run ScanConfigRoot` |
| agy-rule | agy-tmux declares the Gemini rule | 6/10 | `go test ./internal/bridge -run LaunchModelCheckFor` |
