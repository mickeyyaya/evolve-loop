---
score_cap:
  - criterion: "Two probes leave two history lines and usage-windows.json holds the latest only"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2062_001|TestC2062_002|TestC2062_003|TestC2062_004' ./acs/cycle2062"
  - criterion: "A history write failure warns and the probe still acts"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC2062_005 ./acs/cycle2062"
  - criterion: "evolve clihealth usage output is unchanged and creates no history"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2062_006 ./acs/cycle2062"
  - criterion: "appendHistory and RecordObservation have 100 percent statement coverage"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2062_007 ./acs/cycle2062"
---

# Eval: usage observation history

> Pins token-telemetry plan component T4 (docs/plans/token-telemetry-2026-10.md): every quota observation appends one NDJSON line per window to `.evolve/usage-history.ndjson` while `usage-windows.json` keeps the latest snapshot. Source: cycle 2055 failed this item at the build gate on coverage; cycle 2062 continues it.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| history-append | Two probes leave two lines, snapshot holds the latest | 4/10 | `go test -tags acs -run TestC2062_001..004 ./acs/cycle2062` |
| failure-isolation | A history failure warns, the probe still benches | 5/10 | `go test -tags acs -run TestC2062_005 ./acs/cycle2062` |
| output-preservation | `clihealth usage` stdout unchanged, no history written | 6/10 | `go test -tags acs -run TestC2062_006 ./acs/cycle2062` |
| coverage-floor | appendHistory and RecordObservation at 100% | 6/10 | `go test -tags acs -run TestC2062_007 ./acs/cycle2062` |
