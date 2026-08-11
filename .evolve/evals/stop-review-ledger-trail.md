# Eval: stop-review-ledger-trail

**Cycle:** 188
**Task:** Add `OnStopReview` callback to `Deps`, wire it in the tmux driver at stop-review checkpoints, and have the orchestrator append `kind=stop_review` ledger entries.

---

## Criteria

### AC-1: `OnStopReview` field in `Deps` [code]

```bash
grep -n "OnStopReview" go/internal/bridge/engine.go
```

Expected: at least one match showing the field declaration.

```bash
grep -c "OnStopReview" go/internal/bridge/engine.go
```

Expected: output >= 1

---

### AC-2: Driver calls `OnStopReview` at review checkpoint [code]

```bash
grep -n "OnStopReview" go/internal/bridge/driver_tmux_repl.go
```

Expected: at least one match.

```bash
# Negative: the driver must NOT call OnStopReview when it is nil (nil-safe)
grep -n "if.*OnStopReview\|OnStopReview != nil" go/internal/bridge/driver_tmux_repl.go
```

Expected: at least one nil-guard.

---

### AC-3: Orchestrator wires `OnStopReview` to ledger append [code]

```bash
grep -n "OnStopReview\|stop_review" go/internal/core/orchestrator.go
```

Expected: matches showing both the wiring of `OnStopReview` and the `kind="stop_review"` string.

---

### AC-4: `cyclehealth` `self_heal_events` detects stop_review pause [code]

```bash
grep -n "stop_review" go/internal/cyclehealth/cyclehealth.go
```

Expected: at least one match (the signal reads `kind=stop_review` for pause events).

---

### AC-5: Unit tests pass [code]

```bash
cd go && go test ./internal/bridge/... ./internal/cyclehealth/... ./internal/core/... 2>&1 | grep -E "^(ok|FAIL)" | grep -v "cycle[0-9]"
```

Expected: all listed packages show `ok`.

---

### AC-6: ADR-0026 updated [code]

```bash
grep -n "Stage 1.*#5\|#5.*DONE\|stop_review.*DONE\|DONE.*stop_review" docs/architecture/adr/0026-self-healing-review-layer.md
```

Expected: at least one match confirming Stage 1 #5 is marked done.

---

### AC-7 (negative): `withDefaults` still works when `OnStopReview` is nil [code]

```bash
cd go && go test ./internal/bridge/... -run TestWithDefaults -v 2>&1 | tail -5
```

Expected: PASS or no `TestWithDefaults` test (no crash with nil OnStopReview in production path).
