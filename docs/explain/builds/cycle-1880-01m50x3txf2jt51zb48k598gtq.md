# Build Explanation — Cycle 1880

## Build Binding
- Cycle: 1880
- Base SHA: e26a4791cd393b90819d9cfb7ca05404fd554d2c

## Summary
The new verb `evolve bridge promote-rule` writes an auto-respond rule at stage shadow. It writes the rule only after the pattern passes a check against a healthy-pane corpus file. The verb is the first production caller of `interaction.PromoteRule`.

## Rationale
Before this cycle, only tests called `interaction.PromoteRule` (`go/internal/interaction/rulepromote.go`). An operator who learned a rule from a stall had one path, `evolve bridge add-rule`, and that path does no safety check. The new verb sends the rule through the existing validator. Thus a pattern that matches normal pane output is refused before it can send keys.

## Changed Areas
- `go/cmd/evolve/cmd_bridge.go` — adds the `promote-rule` case, `runBridgePromoteRule` and `corpusLines`, and lists `add-rule` and `promote-rule` in the bridge help. The existing `add-rule` case moves unchanged into `runBridgeAddRule`, because the function-length ratchet refused a longer `runBridge`.
- `go/cmd/evolve/cmd_bridge_test.go` — table tests for each exit path (0, 1, 10), the help output, the default registry directory and the corpus parser.
- `docs/operations/runtime-reference.md` — documents `evolve bridge add-rule` and `evolve bridge promote-rule`, with the exit codes.
- `.evolve/evals/cli-bridge-promote-rule.md` — the eval of this task, written by an earlier phase of this cycle.
- `go/acs/cycle1880/predicates_test.go` — the ACS predicates of this task, written by the TDD phase of this cycle.

## Design Decisions
- The CLI calls `interaction.ValidateRule` with no corpus first. A failure there is a usage error (exit 10): a short pattern, a pattern that does not compile, or a bad key. A failure in `interaction.PromoteRule` after that can only be a corpus match or a write error, so it exits 1. This split needs no change to the `interaction` package.
- The default directory is `<project-root>/.evolve/instincts/interaction-rules`. This is the path that `interactionRulesDir` in `go/internal/bridge/interaction_rules.go` returns. That helper is not exported, and the role guard refuses a build edit of `internal/bridge` (control plane). Thus the CLI repeats the one `filepath.Join`.
- Rejected: a new exported wrapper in `internal/bridge` that also adds the embedded healthy corpus. The role guard refused the edit. Boot validation in `bridge.loadShadowObservers` and `bridge.loadPromotedPrompts` still drops a rule that matches the embedded corpus, so no unsafe rule can fire.
- Rejected: the hand-written flag loop that `add-rule` uses. It ignores an unknown flag, and the eval requires exit 10 for an unknown flag. `flag.FlagSet` with `ContinueOnError` gives that result.

## Verification
- `go test -tags acs -count=1 ./acs/cycle1880`: 7/7 predicates PASS. The predicates run the built `evolve` binary, so they prove that the production entry point reaches `interaction.PromoteRule`.
- `go test -count=1 -run TestRunBridgePromoteRule ./cmd/evolve/`: PASS.
- `evolve acs suite --cycle 1880`: verdict PASS, red=0.

## Compatibility
`add-rule` keeps its flags, its output and its exit codes. The rule file has the format that `interaction.PromoteRule` already writes. A second promotion of the same pattern keeps the existing file.

## Limitations
The verb does not check the corpus that is embedded in the binary. Boot validation covers that corpus. The verb does not set a rule to `enforce`; the batch-end sweep does that after it measures the rule.
