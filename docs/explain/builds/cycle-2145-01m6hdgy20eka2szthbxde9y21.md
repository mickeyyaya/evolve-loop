# Build Explanation — Cycle 2145

## Build Binding
- Cycle: 2145
- Base SHA: 5b25551a114e9878a5c24f63bfb96be3b11dcc22

## Summary
The launch check compares the model of the catalog pick. claude-tmux compares the transcript model with the pick. agy-tmux declares the launch rule. The boundary update has a time budget.

## Rationale
A family match let a wrong model of the right family run. The transcript already carries the served model, so the check reuses it. One deadline bounds the two-family stall. It adds no new failure class.

## Changed Areas
- `go/internal/bridge/launch_model_verification.go` — verifies the footer model name against the pick and adds `expected_model` and `served_model` to the mismatch signal.
- `go/internal/bridge/served_model.go` — new post-run comparison of the transcript model with the dispatched pick.
- `go/internal/bridge/attempt_telemetry.go` — calls the served-model check when a try is recorded.
- `go/internal/bridge/manifests/agy-tmux.json` — declares `launch_model_verification` for the gemini family.
- `go/internal/bridge/manifests/agy-claude-tmux.json` — drops its copy of the rule, which the base now supplies.
- `go/internal/cliupdate/cliupdate.go` — adds `DefaultBoundaryBudget` and `Seams.Budget`, and reports a budget cut as a skip.
- `docs/architecture/packages/internal-bridge.md` — documents the model-name check and the served-model check.
- `docs/architecture/packages/internal-cliupdate.md` — documents the boundary budget.
- `go/internal/bridge/adversarial_faults_test.go` — agy fixtures show a Gemini footer.
- `go/internal/bridge/agy_claude_target_test.go` — footer derived from the realized pick.
- `go/internal/bridge/driver_tmux_variants_test.go` — agy happy path shows a Gemini footer.
- `go/internal/bridge/model_tier_parity_test.go` — footer derived from the realized pick.
- `go/internal/bridge/parallel_serial_tests_test.go` — lists the new catalog-swapping tests as serial.
- `go/internal/bridge/probe_boot_retry_test.go` — agy cold-start fixture shows a Gemini footer.
- `go/internal/bridge/realizer_wiring_test.go` — agy fixtures show a Gemini footer.

## Design Decisions
The name comparison drops a leading vendor word, so a bare `Opus 5.5` label still verifies. A budget cut uses a context cause, so a caller cancel stays an interrupt.

## Verification
The bridge and cliupdate packages and the cycle 2145 predicates pass; one cliupdate test fails because of the cell environment.

## Compatibility
Every agy-tmux launch now needs a readable Gemini footer within 10 seconds, or it exits 87 and the walk moves on.

## Limitations
The transcript check only signals after the run; it kills nothing.
