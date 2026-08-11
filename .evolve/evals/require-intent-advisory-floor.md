# Eval: require-intent-advisory-floor

Cycle 240, defect 4. When `EVOLVE_REQUIRE_INTENT=1` is set, the intent phase must run even when advisory routing is active. Currently the advisor's plan omits intent (it doesn't know about the env-var requirement), so `enforceNext` overrides the static SM's "intent" next with the router's "scout" decision, silently skipping intent. Fix: force intent to `run:true` in the clamped plan when `IntentRequired` is set — mirroring how `ClampPlanToFloor` forces `build`/`audit`/`tdd`.

## Criteria

### C1 — Intent forced on when RequireIntent and plan absent [code]

```
cd go && go test ./internal/router/... -run TestClampPlanToFloor_IntentForcedWhenRequired -v
```

Expected: when `IntentRequired=true` in RouteInput and the plan lacks an intent entry (or has `run:false`), `ClampPlanToFloor` forces intent to `run:true` and records a clamp with Rule `"require-intent"`.

### C2 — Intent NOT forced when IntentRequired=false [code]

```
cd go && go test ./internal/router/... -run TestClampPlanToFloor_IntentNotForcedWhenNotRequired -v
```

Expected: when `IntentRequired=false`, a plan without intent is left untouched (no intent clamp added).

### C3 — Existing floor tests unchanged (no regression) [code]

```
cd go && go test ./internal/router/... -run TestClampPlanToFloor -count=1 -timeout 20s
```

Expected: exit 0, all prior floor tests pass.

### C4 — Integration: intent appears in cycle when REQUIRE_INTENT=1 with advisory routing [code]

```
cd go && go test ./cmd/evolve/... -run TestBuildCycleEnv_PropagatesRequireIntent -count=1 -timeout 30s
```

Expected: exit 0. The propagation test remains green (prerequisite: env var still threads through).

### C5 — Full core suite passes [code]

```
cd go && go test ./internal/router/... ./internal/core/... -count=1 -timeout 90s
```

Expected: exit 0, all tests pass.
