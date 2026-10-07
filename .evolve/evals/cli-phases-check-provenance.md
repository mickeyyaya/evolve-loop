---
score_cap:
  - criterion: "evolve phases check-provenance --cycle N prints the violation and exits 1 when a phase artifact's provenance header disagrees with the cycle or the ledger"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1819_01[01]_' ./acs/cycle1819"
  - criterion: "check-provenance exits 0 on a cycle whose artifact agrees with its header and the ledger"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1819_012_' ./acs/cycle1819"
  - criterion: "check-provenance --json emits one valid JSON document carrying the violation, with the same exit code"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1819_013_' ./acs/cycle1819"
  - criterion: "A missing, zero, negative or non-numeric --cycle is check-provenance's own usage error (exit 10)"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1819_014_' ./acs/cycle1819"
  - criterion: "An unreadable ledger, or a ledger line over the scanner's limit, exits 2 and names the ledger instead of passing silently"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1819_01[56]_' ./acs/cycle1819"
  - criterion: "runtime-reference.md documents evolve phases check-provenance"
    max_if_missing: 4
    evidence: "grep -q 'phases check-provenance' docs/operations/runtime-reference.md"
  - criterion: "The phasecoherence package stays green"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 ./internal/phasecoherence/"
---

# Eval: evolve phases check-provenance reaches the ledger provenance check

> Pins `evolve phases check-provenance --cycle N [--json]`, added in cycle 1819
> (inbox `cli-phases-check-provenance`, 2026-09-30, from the core-function CLI
> inventory). Before it, `phasecoherence.CheckProvenance` had only a test caller,
> ignored a ledger it could not open, and stopped silently past a 64 KiB ledger
> line (inbox `phasecoherence-provenance-silent`). The command must exit 0 when
> clean, 1 on any violation and 2 when the ledger cannot be read or scanned
> completely. Predicates drive `phasecmd.RunPhases` (the `phases` entry in
> `go/cmd/evolve/registry.go`) over a temp project carrying the repo's phase
> registry, a ledger and a `runs/cycle-1819/build-report.md` fixture.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| mismatch-exit-1 | header cycle or ledger tree_sha mismatch -> printed, exit 1 | 8/10 | `go test -tags acs -run 'TestC1819_01[01]_' ./acs/cycle1819` |
| clean-exit-0 | agreeing header and ledger -> exit 0 | 7/10 | `go test -tags acs -run TestC1819_012_ ./acs/cycle1819` |
| json-output | --json is valid JSON with the violation | 5/10 | `go test -tags acs -run TestC1819_013_ ./acs/cycle1819` |
| cycle-flag | missing/invalid --cycle -> exit 10 from the subcommand | 5/10 | `go test -tags acs -run TestC1819_014_ ./acs/cycle1819` |
| ledger-unreadable-exit-2 | unreadable ledger or over-limit line -> exit 2 | 8/10 | `go test -tags acs -run 'TestC1819_01[56]_' ./acs/cycle1819` |
| docs | runtime-reference.md documents the command | 4/10 | `grep -q 'phases check-provenance' docs/operations/runtime-reference.md` |
| package-health | phasecoherence unit tests green | 7/10 | `go test -count=1 ./internal/phasecoherence/` |
