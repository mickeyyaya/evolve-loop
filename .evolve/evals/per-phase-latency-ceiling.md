# Eval: per-phase-latency-ceiling

**Cycle:** 188
**Task:** Extend `checkPhaseLatency` in `cyclehealth.go` to read per-phase ceiling overrides (`EVOLVE_<PHASE_UPPER>_LATENCY_CEILING_S`) before falling back to the global `EVOLVE_PHASE_LATENCY_CEILING_S`.

---

## Criteria

### AC-1: Per-phase env-var reading in `checkPhaseLatency` [code]

```bash
grep -n "EVOLVE_.*LATENCY_CEILING_S\|perPhaseCeiling\|phaseEnvCeiling\|perPhase" go/internal/cyclehealth/cyclehealth.go
```

Expected: at least one match showing per-phase ceiling lookup logic.

---

### AC-2: Phase name normalization (upper + dash→underscore) [code]

```bash
# Confirm the normalization path exists
grep -n "ToUpper\|strings.Replace\|ReplaceAll" go/internal/cyclehealth/cyclehealth.go
```

Expected: at least one match near the phase-latency check.

---

### AC-3: Unit tests cover per-phase override scenario [code]

```bash
grep -n "TestCheckPhaseLatency\|SCOUT_LATENCY\|BUILD_LATENCY\|per.phase" go/internal/cyclehealth/cyclehealth_test.go
```

Expected: at least one match showing a per-phase override test case.

---

### AC-4: Unit tests pass [code]

```bash
cd go && go test ./internal/cyclehealth/... 2>&1 | grep -E "^(ok|FAIL)"
```

Expected: `ok  github.com/mickeyyaya/evolve-loop/go/internal/cyclehealth`

---

### AC-5: Global ceiling still applies as fallback [code]

```bash
# Negative test: a phase with NO per-phase override still uses global ceiling
grep -n "globalCeiling\|fallback.*ceiling\|global.*default\|EVOLVE_PHASE_LATENCY_CEILING_S" go/internal/cyclehealth/cyclehealth.go
```

Expected: at least two matches (the global ceiling is still read and used as fallback).

---

### AC-6: CLAUDE.md env-var table documents per-phase pattern [code]

```bash
grep -n "EVOLVE.*LATENCY_CEILING\|per-phase.*latency\|per.*phase.*ceiling" CLAUDE.md
```

Expected: at least one match.

---

### AC-7 (negative): A phase-specific ceiling lower than global triggers WARN [code]

```bash
cd go && go test ./internal/cyclehealth/... -run TestCheckPhaseLatency -v 2>&1 | grep -E "(PASS|FAIL|per.phase|override)"
```

Expected: at least one `PASS` line; no `FAIL` lines.
