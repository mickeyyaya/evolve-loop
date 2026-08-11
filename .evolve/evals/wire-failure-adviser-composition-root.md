---
slug: wire-failure-adviser-composition-root
title: Wire FailureAdviser at Composition Root (ADR-0044 completion)
phase: build
version: 1
---

# Eval: wire-failure-adviser-composition-root

## Goal

Verify that `wireOrchestratorDeps` (the composition root) wires the ADR-0044 `FailureAdviser`
into the orchestrator, completing the shadow-safe composition chain. In default shadow mode
(`EVOLVE_PHASE_RECOVERY=shadow`) the adviser must never be consulted; at enforce mode it must fire
for unclassified artifact-timeout failures.

## Acceptance Criteria

### C1 — Compile check [code]

```bash
cd go && go build ./cmd/evolve/
echo "exit:$?"
```

Expected: `exit:0`

### C2 — Persona file loads without error [code]

```bash
cd go && go test ./cmd/evolve/ -run TestWireOrchestratorDeps_FailureAdviserPersonaLoads -v -count=1
```

Expected: `--- PASS: TestWireOrchestratorDeps_FailureAdviserPersonaLoads`

### C3 — Adviser is wired (non-nil) in production composition [code]

```bash
cd go && go test ./cmd/evolve/ -run TestWireOrchestratorDeps_FailureAdviserWired -v -count=1
```

Expected: `--- PASS: TestWireOrchestratorDeps_FailureAdviserWired`

### C4 — Shadow-mode gate: adviser wired but never called below enforce [code]

```bash
cd go && go test ./internal/core/ -run TestPhaseRecovery_ShadowDefault_NoCorrectiveAction -v -count=1
```

Expected: `--- PASS: TestPhaseRecovery_ShadowDefault_NoCorrectiveAction`

### C5 — Negative: nil adviser = inert hook (regression) [code]

```bash
cd go && go test ./internal/core/ -run TestAdviseHook_NilAdviser_IsInert -v -count=1
```

Expected: `--- PASS: TestAdviseHook_NilAdviser_IsInert`

### C6 — Full internal test suite passes [code]

```bash
cd go && go test ./internal/core/... -count=1 2>&1 | tail -3
```

Expected: `ok  	github.com/mickeyyaya/evolve-loop/go/internal/core`

## Negative / Edge Cases

- **N1 (cheapest fake):** Adding `WithFailureAdviser(nil)` — must not wire a nil adviser (the
  existing nil-guard in `WithFailureAdviser` prevents this).
- **N2 (shadow bypass):** Setting `EVOLVE_PHASE_RECOVERY=shadow` and triggering an artifact timeout
  must NOT call the adviser (C4 covers this).
- **N3 (persona missing):** If `evolve-failure-advisor.md` cannot be parsed, the adviser still
  wires with an empty persona (best-effort, same pattern as the router adviser).
