---
score_cap:
  - criterion: "bootBinaryRefresh never rebuilds or exec's while another fleet lane is confirmed concurrently active"
    max_if_missing: 9
    evidence: "cd go && go test -run TestBootBinaryRefresh_ConcurrentFleetLaneStopsRefresh -v ./cmd/evolve/"
  - criterion: "an unverifiable lease/lane check fails open (skips refresh) rather than proceeding to rebuild on an unproven state"
    max_if_missing: 7
    evidence: "cd go && go test -run TestBootBinaryRefresh_FleetLaneCheckErrorFailsOpen -v ./cmd/evolve/"
  - criterion: "a confirmed-active-lane WARN is textually distinguishable from a lease-check-error WARN"
    max_if_missing: 5
    evidence: "cd go && go test -run TestBootBinaryRefresh_FleetLaneWarningsAreDistinguishable -v ./cmd/evolve/"
  - criterion: "a confirmed-inactive fleet lane preserves the existing stale+go-delta rebuild-then-exec behavior byte-identical"
    max_if_missing: 6
    evidence: "cd go && go test -run TestBootBinaryRefresh_NoConcurrentLaneProceedsNormally -v ./cmd/evolve/"
---

# Eval: Fleet-lane-awareness guard for the plane-binary boot self-heal

> Pins chain-boundary-binary-refresh-stop (cycle-1353). `bootBinaryRefresh`
> (`go/cmd/evolve/cmd_loop_boot_refresh.go`) rebuilds and re-execs the plane
> binary at every fresh loop boot with zero awareness of whether another
> fleet lane is concurrently mid-batch. The function's own doc comment
> (lines 37-40) names this an "accepted risk" resting on an operational
> assumption ("simultaneous loop launches are already excluded operationally
> ... single-operator plane") — but the standing memory rule
> (`stale_binary_false_fail`, 2026-08-05) is stricter: "NEVER rebuild plane
> binary mid-batch (SELF_SHA); plane sync = merge-only `git merge
> origin/main`." The fleet architecture already runs N≥1 concurrent lanes
> (`fleet_width_always_respected`), so the comment's assumption is stale
> relative to actual runtime topology. This eval enforces an injected
> fleet-lane-concurrency seam (`bootRefreshFleetLaneFn`) that STOPS the
> refresh — before either rebuild or exec — whenever a concurrent lane is
> confirmed active, with the same fail-open-on-uncertainty posture every
> other check in this function already uses.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| active-lane-stop | No rebuild/exec while a concurrent lane is confirmed active | 9/10 | `go test -run TestBootBinaryRefresh_ConcurrentFleetLaneStopsRefresh` |
| check-error-fail-open | Lease-check error skips refresh instead of proceeding unverified | 7/10 | `go test -run TestBootBinaryRefresh_FleetLaneCheckErrorFailsOpen` |
| warn-distinguishability | Active-lane WARN text differs from check-error WARN text | 5/10 | `go test -run TestBootBinaryRefresh_FleetLaneWarningsAreDistinguishable` |
| no-regression | Inactive-lane path preserves existing rebuild+exec behavior | 6/10 | `go test -run TestBootBinaryRefresh_NoConcurrentLaneProceedsNormally` |
