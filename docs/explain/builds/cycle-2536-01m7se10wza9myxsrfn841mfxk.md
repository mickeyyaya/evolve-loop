# Build Explanation — Cycle 2536

## Build Binding
- Cycle: 2536
- Base SHA: 15efad21b405115005b2b42180eabfd0691687a3

## Summary
Non-protected signal-code registry entries now write ASD-STE100 text, and `docs/architecture/signal-codes.md` matches them without drift. Protected control-plane files were restored for console work.

## Rationale
Rewriting registry strings updates the generated markdown table at its source. Restoring the 22 protected control-plane files honors ADR-0064 while delivering the non-protected rewrites.

## Changed Areas
- `docs/architecture/signal-codes.md` — updates the generated signal-code rows for the 64 non-protected Go files.
- `go/cmd/evolve/cli_queue.go` — rewrites `CLI_QUEUE_FAILED` doc text to remove banned words.
- `go/cmd/evolve/signal_loop.go` — splits loop halt signal doc strings into short ASD-STE100 sentences.
- `go/internal/bridge/driver_claudecloud.go` — splits cloud-launch signal doc strings into short ASD-STE100 sentences.
- `go/internal/bridge/driver_tmux_wait_interaction.go` — splits `BRIDGE_INBOX_DRAIN_FAILED` doc text into short ASD-STE100 sentences.
- `go/internal/bridge/launch_model_verification.go` — splits `BRIDGE_DISPATCH_MODEL_MISMATCH` doc text into short ASD-STE100 sentences.
- `go/internal/bridge/launchoutcome/signal.go` — splits exit-code signal doc strings and shared suffix into short ASD-STE100 sentences.
- `go/internal/bridge/signal.go` — rewrites bridge, liveness, and token signal doc strings into ASD-STE100 sentences.
- `go/internal/bypass/bypass.go` — splits `CONTROL_BYPASS_USED` doc text into short ASD-STE100 sentences.
- `go/internal/cli/agentcmd/verdict.go` — splits review verdict mismatch and parse-defect doc strings into short ASD-STE100 sentences.
- `go/internal/cli/phasecmd/phase_observer.go` — splits `OBSERVER_POLICY_LOAD_FAILED` doc text into short ASD-STE100 sentences.
- `go/internal/clihealth/auth_bench.go` — splits `BRIDGE_AUTH_BENCH_CLEARED` doc text into short ASD-STE100 sentences.
- `go/internal/config/signals.go` — splits config warning signal doc strings into short ASD-STE100 sentences.
- `go/internal/core/advisor/advisor.go` — splits advisor signal doc strings into short ASD-STE100 sentences.
- `go/internal/core/audit_repair_convergence.go` — splits `CONVERGENCE_RUNG` doc text into short ASD-STE100 sentences.
- `go/internal/core/build_floor_retake.go` — splits build-floor retake signal doc strings into short ASD-STE100 sentences.
- `go/internal/core/builder_selfcheck.go` — splits `BUILD_SELFCHECK_FINDINGS` doc text into short ASD-STE100 sentences.
- `go/internal/core/carryover/carryover.go` — splits carryover signal doc strings into short ASD-STE100 sentences.
- `go/internal/core/composition_carryforward.go` — splits `ORCHESTRATOR_COMPOSED_GATE_DECLINED` doc text into short ASD-STE100 sentences.
- `go/internal/core/cycle_spend_limit.go` — splits `CYCLE_SPEND_LIMIT` doc text into short ASD-STE100 sentences.
- `go/internal/core/explanation_correction_rung.go` — splits explanation-correction signal doc strings into short ASD-STE100 sentences.
- `go/internal/core/failurediag/diag.go` — splits `FAILUREDIAG_SIDECAR_WRITE_FAILED` doc text into short ASD-STE100 sentences.
- `go/internal/core/failurelearning/failurelearning.go` — splits failure-learning signal doc strings into short ASD-STE100 sentences.
- `go/internal/core/fleet_rebase_conflict_signal.go` — splits `ORCHESTRATOR_FLEET_REBASE_CONFLICT` doc text into short ASD-STE100 sentences.
- `go/internal/core/floor_convergence.go` — splits `BUILD_FLOOR_CONVERGING` doc text into short ASD-STE100 sentences.
- `go/internal/core/head_moved_rebind.go` — splits `ORCHESTRATOR_SHIP_RECOVERY_PATH` doc text into short ASD-STE100 sentences.
- `go/internal/core/host_effects.go` — rewrites `ORCHESTRATOR_HOST_EFFECT_FAILED` doc text to remove banned words.
- `go/internal/core/lane_pin_drift.go` — splits `ISOLATION_LANE_PIN_FORGED` doc text into short ASD-STE100 sentences.
- `go/internal/core/lint_floor.go` — splits `BUILD_LINT_FLOOR_SKIPPED` doc text into short ASD-STE100 sentences.
- `go/internal/core/outcome/recorder.go` — splits outcome sidecar and timing write failure doc strings into short ASD-STE100 sentences.
- `go/internal/core/plan_phase_gated_signal.go` — splits `ORCHESTRATOR_PLAN_PHASE_GATED` doc text into short ASD-STE100 sentences.
- `go/internal/core/repair_handoff.go` — splits repair hand-off signal doc strings into short ASD-STE100 sentences.
- `go/internal/core/resume_carrier_heal.go` — splits `ORCHESTRATOR_RESUME_CARRIER_PENDED` doc text into short ASD-STE100 sentences.
- `go/internal/core/retry_ladder.go` — splits retry-ladder signal doc strings into short ASD-STE100 sentences.
- `go/internal/core/scope_delta_floor.go` — splits `ORCHESTRATOR_SCOPE_DELTA` doc text into short ASD-STE100 sentences.
- `go/internal/core/ship_capacity.go` — splits `GATE_REPO_CAPACITY_PARKED` doc text into short ASD-STE100 sentences.
- `go/internal/core/ship_rebase_inbox.go` — splits `ORCHESTRATOR_FLEET_REBASE_INBOX_RESOLVED` doc text into short ASD-STE100 sentences.
- `go/internal/core/ship_recovery_debugger.go` — splits `ORCHESTRATOR_REBASE_REENTRY_ABORTED` doc text into short ASD-STE100 sentences.
- `go/internal/core/shipwindow_hold.go` — splits `ORCHESTRATOR_SHIP_WINDOW_LEASE_BROKEN` doc text into short ASD-STE100 sentences.
- `go/internal/core/signal.go` — splits orchestrator phase and repair signal doc strings into short ASD-STE100 sentences.
- `go/internal/core/signal_cycle.go` — splits cycle pause, defer, resume, and binding doc strings into short ASD-STE100 sentences.
- `go/internal/core/wall_signal.go` — splits `BRIDGE_LOGIN_REQUIRED` doc text into short ASD-STE100 sentences.
- `go/internal/deliverable/gatesignal/reporter.go` — splits contract-gate signal doc strings into short ASD-STE100 sentences.
- `go/internal/inboxcurator/curate.go` — splits `INBOX_CURATOR_PROPOSAL` doc text into short ASD-STE100 sentences.
- `go/internal/integration/driver.go` — splits `SHIP_INTEGRATION_DRIVER_FAILED` doc text into short ASD-STE100 sentences.
- `go/internal/isolation/codes.go` — splits `ISOLATION_I2_VIOLATION` and `ISOLATION_REQUEST_REFUSED` doc strings into short ASD-STE100 sentences.
- `go/internal/lanerouting/lazy.go` — splits `INBOX_ROUTING_FALLBACK` doc text into short ASD-STE100 sentences.
- `go/internal/loopchain/loopchain.go` — splits loop-chain signal doc strings into short ASD-STE100 sentences.
- `go/internal/loopwave/loopwave.go` — splits wave-dispatch signal doc strings into short ASD-STE100 sentences.
- `go/internal/modelcatalog/degraded_event.go` — splits `CONFIG_CATALOG_REFRESH_DEGRADED` doc text into short ASD-STE100 sentences.
- `go/internal/observerengine/observerengine.go` — splits observer signal doc strings into short ASD-STE100 sentences.
- `go/internal/orchestrator/orchestrator.go` — splits orchestrator pool signal doc strings into short ASD-STE100 sentences.
- `go/internal/phases/runner/host_effects.go` — splits `RUNNER_HOST_EFFECT_FAILED` doc text into short ASD-STE100 sentences.
- `go/internal/phases/runner/postmortem.go` — splits `RUNNER_ATTEMPT_POSTMORTEM_FAILED` doc text into short ASD-STE100 sentences.
- `go/internal/phases/runner/self_review.go` — splits `RUNNER_SELF_REVIEW_MISSING` doc text into short ASD-STE100 sentences.
- `go/internal/phases/ship/carry.go` — splits `SHIP_CARRY_NOT_REPROVEN` doc text into short ASD-STE100 sentences.
- `go/internal/phases/ship/deliverable.go` — splits ship integration delivery signal doc strings into short ASD-STE100 sentences.
- `go/internal/phases/ship/scanshadow.go` — splits `SHIP_SCAN_SHADOW_ESCAPE` doc text into short ASD-STE100 sentences.
- `go/internal/phases/ship/shipwindow_lease.go` — splits `SHIP_WINDOW_LEASE_BROKEN` doc text into short ASD-STE100 sentences.
- `go/internal/phasespec/routing_signal.go` — splits `CONFIG_PHASE_ROUTING_DEFECT` doc text into short ASD-STE100 sentences.
- `go/internal/shiperr/signalcodes.go` — splits ship error signal doc strings into short ASD-STE100 sentences.
- `go/internal/signalcenter/expected.go` — splits `SIGNAL_EXPECTED_ABSENT` doc text into short ASD-STE100 sentences.
- `go/internal/signalcenter/registry.go` — splits built-in signal doc strings into short ASD-STE100 sentences.
- `go/internal/triagecap/signals.go` — splits triage-cap signal doc strings into short ASD-STE100 sentences.
- `go/internal/usageevidence/usageevidence.go` — splits `BRIDGE_USAGE_EVIDENCE` doc text into short ASD-STE100 sentences.

## Design Decisions
Each updated `RegisterCode` entry keeps all field names, `key=value` tokens, vocabulary sets, and ADR references unchanged. Longer clauses split at periods instead of semicolons so every sentence stays within 25 words and every cell stays within six sentences. Protected control-plane files stay at base SHA under ADR-0064 so the console handles them.

## Verification
All twenty `TestC2536_*` predicates, `evolve docs ste-lint --strict`, `gofmt`, `go vet`, and clean-env package tests verify the lane-reachable changes.

## Compatibility
All signal code names, field keys, vocabulary strings, and `README.md` `| vX.Y |` rows stay unchanged.

## Limitations
Twenty-two protected control-plane files still hold non-STE doc strings and remain for console follow-up.
