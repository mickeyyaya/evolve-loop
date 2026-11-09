# Build Explanation — Cycle 2234

## Build Binding
- Cycle: 2234
- Base SHA: b8292fd372edc03ca8156a7b6a0f907a8477fae0

## Summary
A seat that waits on its own background shell no longer raises LIVENESS_PHASE_STALLED at the plain stall limit. The pane observer uses a longer limit from policy when the pane footer shows a running shell.

## Rationale
A seat that waits on a shell shows no new transcript, so the plain limit marked a healthy seat as stalled. The footer already shows the shell count. The smallest fix reads that count and selects a longer limit from policy.json. A pane with no shell keeps the plain limit.

## Changed Areas
- `go/internal/bridge/panestream/tokenline.go` — adds `RunningShells`, which reads the count from the last non-blank pane line only.
- `go/internal/panewatch/panewatch.go` — adds the `shells` field to the snapshot.
- `go/internal/panewatch/tracker.go` — carries the shell count in a frame and republishes when it changes.
- `go/internal/bridge/pane_watch.go` — publishes the footer shell count from the production wait loop.
- `go/internal/policy/dispatch.go` — adds `observer.shell_stall_s` with default 3600.
- `go/internal/adapters/observer/observer.go` — picks the shell limit for a pane with running shells and names the shells in the reason.
- `go/internal/adapters/observer/core_adapter.go` — passes the resolved shell limit to the observer.
- `go/internal/adapters/observer/stall_signal.go` — adds the `shells` field and the limit used to the stall signal.
- `docs/architecture/packages/internal-adapters-observer.md` — records the shell-aware stall rule.

## Design Decisions
The limit is policy data, not a flag. The footer is the only source, so shell mentions in the transcript never count. The shell limit applies only when it is longer than the plain limit.

## Verification
Unit tests cover the footer parse, the bridge publish, the tracker, the policy default and override. Predicates TestC2234_001 to TestC2234_006 cover the behavior end to end.

## Compatibility
Snapshots without `shells` read as zero shells. Existing policy files keep working because `shell_stall_s` has a default.

## Limitations
The count comes from the Claude footer format. Other CLIs show no count and keep the plain limit.
